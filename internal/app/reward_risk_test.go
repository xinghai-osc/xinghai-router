package app

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

func TestNormalizeRiskNameUnicode(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"width case and separators", " Ｆａｒｍ－００１_ ", "farm001"},
		{"compatibility ligature", "Oﬃce", "office"},
		{"case folding", "Straße", "strasse"},
		{"canonical composition", "Cafe\u0301", "café"},
		{"format characters", "A\u200bB\u200dC\ufeff", "abc"},
		{"composition across removed format character", "A\u200b\u030A", "å"},
		{"unicode whitespace punctuation", "星海\u3000用·户—１２", "星海用户12"},
		{"greek case folding", "ΟΣ ος", "οσοσ"},
		{"non Latin letters remain distinct", "раypal", "раypal"},
		{"only separators", " _-\u200b\t", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRiskName(tt.input)
			if got != tt.want {
				t.Fatalf("normalized %q = %q, want %q", tt.input, got, tt.want)
			}
			if normalizeRiskName(got) != got {
				t.Fatalf("normalization is not idempotent for %q", got)
			}
		})
	}
	got := normalizeRiskName(strings.Repeat("界", 101))
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 100 {
		t.Fatalf("long Unicode name was not bounded by complete runes: %q", got)
	}
	if normalizeRiskName("paypal") == normalizeRiskName("раypal") {
		t.Fatal("normalization must not invent cross-script identity equivalence")
	}
}

func TestRiskNameSimilarityUsesRunesAndProtectsShortNames(t *testing.T) {
	for _, tt := range []struct {
		left, right string
		want        float64
	}{
		{"", "", 0},
		{"", "member001", 0},
		{"王小明", "王小明", 1},
		{"王小明", "王小亮", 0},
		{"alice", "alixe", 0},
		{"abcdef", "abcxef", 1 - 1.0/6},
		{"abcdef", "abcdefg", 1 - 1.0/7},
		{"abcdef", "abdcef", 1 - 2.0/6},
		{"星海账户用户甲", "星海账户用户乙", 1 - 1.0/7},
		{"abcdef", "uvwxyz", 0},
	} {
		got := riskNameSimilarity(tt.left, tt.right)
		if math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("similarity(%q,%q) = %v, want %v", tt.left, tt.right, got, tt.want)
		}
		if reverse := riskNameSimilarity(tt.right, tt.left); math.Abs(got-reverse) > 1e-12 {
			t.Errorf("similarity is asymmetric for %q and %q", tt.left, tt.right)
		}
	}
	left := normalizeRiskName("ＦＡＲＭ－００１")
	right := normalizeRiskName("farm_001")
	if riskNameSimilarity(left, right) != 1 {
		t.Fatal("equivalent normalized names must match exactly")
	}
}

func TestRiskNameTemplateBoundaries(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"member12", "member#number"},
		{"member987", "member#number"},
		{"membera1b2c3d4", "member#hex"},
		{"memberd4c3b2a1", "member#hex"},
		{"member1", ""},
		{"ab12", ""},
		{"memberabcdefab", ""},
		{"membera1b2c3", ""},
		{"member", ""},
	} {
		if got := riskNameTemplate(tt.name); got != tt.want {
			t.Errorf("template(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
	if got := riskNameTemplate(normalizeRiskName("ＭＥＭＢＥＲ－１２")); got != "member#number" {
		t.Fatalf("normalized full-width numbered name template = %q", got)
	}
}

func TestRewardRiskClusterRequiresCorrelatedCohort(t *testing.T) {
	peer := func(id, name string, browser, ipInviter, rtc bool) rewardRiskPeer {
		return rewardRiskPeer{ID: id, Name: name, Browser: browser, HTTP: ipInviter, IPInviter: ipInviter, RTC: rtc, BurstBrowser: browser, BurstIPInviter: ipInviter}
	}
	similar := func(id string, browser, ipInviter, rtc bool) rewardRiskPeer {
		return peer(id, "member00"+id, browser, ipInviter, rtc)
	}
	unrelatedBrowser := []rewardRiskPeer{
		peer("10", "horizon", true, false, false),
		peer("11", "orchard", true, false, false),
		peer("12", "mountain", true, false, false),
		peer("13", "windingriver", true, false, false),
	}
	for _, tt := range []struct {
		name    string
		peers   []rewardRiskPeer
		ban     bool
		reasons []string
	}{
		{"no peers", nil, false, nil},
		{"similarity alone", []rewardRiskPeer{similar("2", false, false, false), similar("3", false, false, false)}, false, []string{"username_cluster"}},
		{"WebRTC similarity alone", []rewardRiskPeer{similar("2", false, false, true), similar("3", false, false, true)}, false, []string{"username_cluster", "webrtc_username_cluster"}},
		{"browser burst without name cohort", unrelatedBrowser, false, []string{"account_burst"}},
		{"matching browser cohort below burst threshold", []rewardRiskPeer{similar("2", true, false, false), similar("3", true, false, false), peer("4", "horizon", true, false, false)}, false, []string{"username_cluster"}},
		{"browser burst with correlated names", []rewardRiskPeer{similar("2", true, false, false), similar("3", true, false, false), peer("4", "horizon", true, false, false), peer("5", "orchard", true, false, false)}, true, []string{"username_cluster", "account_burst"}},
		{"IP inviter burst with correlated names", []rewardRiskPeer{similar("2", false, true, false), similar("3", false, true, false), peer("4", "horizon", false, true, false), peer("5", "orchard", false, true, false)}, true, []string{"username_cluster", "account_burst"}},
		{"cannot stitch RTC names into unrelated browser burst", append(slices.Clone(unrelatedBrowser), similar("2", false, false, true), similar("3", false, false, true)), false, []string{"username_cluster", "webrtc_username_cluster", "account_burst"}},
		{"cannot stitch IP inviter names into browser burst", append(slices.Clone(unrelatedBrowser), similar("2", false, true, false), similar("3", false, true, false)), false, []string{"username_cluster", "account_burst"}},
		{"cannot combine two insufficient burst cohorts", []rewardRiskPeer{similar("2", true, false, false), similar("3", true, false, false), similar("4", false, true, false), similar("5", false, true, false)}, false, []string{"username_cluster"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reasons, details, ban := rewardRiskCluster(defaultRewardRiskSettings(), "member001", tt.peers)
			if ban != tt.ban {
				t.Fatalf("ban = %v, want %v; details=%v", ban, tt.ban, details)
			}
			if !slices.Equal(reasons, tt.reasons) {
				t.Fatalf("reasons = %v, want %v", reasons, tt.reasons)
			}
		})
	}
}

func TestRewardRiskClusterTemplatesAndEvidenceCap(t *testing.T) {
	cfg := defaultRewardRiskSettings()
	cfg.UsernameSimilarity = 1
	peers := []rewardRiskPeer{
		{ID: "2", Name: "member987", BurstBrowser: true},
		{ID: "3", Name: "member654", BurstBrowser: true},
		{ID: "4", Name: "orchard", BurstBrowser: true},
		{ID: "5", Name: "horizon", BurstBrowser: true},
	}
	_, _, ban := rewardRiskCluster(cfg, "member123", peers)
	if !ban {
		t.Fatal("numbered template cohort must remain detectable at exact similarity threshold")
	}
	peers = nil
	for i := 0; i < 40; i++ {
		peers = append(peers, rewardRiskPeer{ID: strconv.Itoa(i + 2), Name: "member" + strconv.Itoa(100+i)})
	}
	_, details, ban := rewardRiskCluster(cfg, "member123", peers)
	if ban || details["similar_accounts"] != 41 {
		t.Fatalf("evidence cap changed cohort decision/count: ban=%v details=%v", ban, details)
	}
	matches, ok := details["matched_accounts"].([]map[string]any)
	if !ok || len(matches) != 30 {
		t.Fatalf("matched account evidence must be capped at 30, got %v", details["matched_accounts"])
	}
}

func TestPublicRiskAddressAndNetworkCanonicalization(t *testing.T) {
	for _, tt := range []struct{ raw, address, network string }{
		{"8.8.8.8", "8.8.8.8", "8.8.8.8"},
		{" ::ffff:8.8.8.8 ", "8.8.8.8", "8.8.8.8"},
		{"2606:4700:4700:0000:0000:0000:0000:1111", "2606:4700:4700::1111", "2606:4700:4700::/64"},
	} {
		a, ok := publicRiskAddress(tt.raw)
		if !ok || a.String() != tt.address || riskNetwork(tt.raw) != tt.network {
			t.Errorf("address/network(%q) = %v,%v,%q", tt.raw, a, ok, riskNetwork(tt.raw))
		}
	}
	for _, raw := range []string{
		"", "not-an-ip", "device.local", "8.8.8.8:53", "[2606:4700:4700::1111]", "fe80::1%eth0",
		"127.0.0.1", "::1", "0.0.0.0", "::", "10.1.2.3", "172.16.1.2", "192.168.1.2", "fc00::1",
		"169.254.1.1", "fe80::1", "224.0.0.1", "ff02::1", "100.64.0.1", "192.0.0.1", "198.18.0.1", "240.0.0.1",
		"192.0.2.1", "198.51.100.1", "203.0.113.1", "2001:db8::1", "::ffff:192.168.1.2", "::ffff:203.0.113.1",
	} {
		if _, ok := publicRiskAddress(raw); ok || riskNetwork(raw) != "" {
			t.Errorf("non-public or malformed address accepted: %q", raw)
		}
	}
	if riskNetwork("2606:4700:4700::1111") != riskNetwork("2606:4700:4700:0:abcd::2") {
		t.Fatal("IPv6 addresses in the same /64 should share a network")
	}
	if riskNetwork("2606:4700:4700::1111") == riskNetwork("2606:4700:4700:1::1111") {
		t.Fatal("distinct IPv6 /64 networks must stay separate")
	}
}

func TestRewardRiskHMACIsolation(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "fake-unit-test-risk-key-one"}}
	other := &Service{cfg: Config{EncryptionKey: "fake-unit-test-risk-key-two"}}
	seen := map[string]bool{}
	for _, purpose := range []string{"network", "rtc", "browser", "cookie", "oauth"} {
		got := s.riskHash(purpose, "same-input")
		if got == "" || got != s.riskHash(purpose, "same-input") || seen[got] {
			t.Fatalf("HMAC is not deterministic and purpose-isolated for %q", purpose)
		}
		if got == other.riskHash(purpose, "same-input") || got == s.riskHash(purpose, "different-input") {
			t.Fatalf("key or value isolation failed for %q", purpose)
		}
		if !strings.HasPrefix(got, "v1:") {
			t.Fatalf("hash does not carry its key scheme version: %q", got)
		}
		decoded, err := hex.DecodeString(strings.TrimPrefix(got, "v1:"))
		if err != nil || len(decoded) != 32 {
			t.Fatalf("hash has invalid digest encoding: %q", got)
		}
		seen[got] = true
		if s.riskHash(purpose, "") != "" {
			t.Fatalf("missing %q signal must not become a shared fingerprint", purpose)
		}
	}
}

func rewardRiskTestCookie(s *Service, issued int64, nonce string) string {
	unsigned := "v1." + strconv.FormatInt(issued, 10) + "." + nonce
	return unsigned + "." + s.riskHash("cookie", unsigned)
}

func TestRewardBrowserCookieSignatureAndTimeBoundaries(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "fake-unit-test-browser-cookie-key"}}
	now := time.Unix(1800000000, 0)
	nonce := "fake-browser-nonce-long-enough"
	for _, tt := range []struct {
		name   string
		issued int64
		valid  bool
	}{
		{"current", now.Unix(), true},
		{"expiry boundary", now.Unix() - 90*86400, true},
		{"expired", now.Unix() - 90*86400 - 1, false},
		{"clock skew tolerance", now.Unix() + 60, true},
		{"future timestamp", now.Unix() + 61, false},
		{"negative timestamp overflow", math.MinInt64, false},
		{"maximum timestamp", math.MaxInt64, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := s.rewardBrowserValue(rewardRiskTestCookie(s, tt.issued, nonce), now)
			if (got != "") != tt.valid {
				t.Fatalf("cookie valid = %v, want %v", got != "", tt.valid)
			}
			if tt.valid && got != s.riskHash("browser", nonce) {
				t.Fatal("valid cookie did not derive its browser identity")
			}
		})
	}
	valid := rewardRiskTestCookie(s, now.Unix(), nonce)
	for _, value := range []string{
		"", valid + "x", strings.Replace(valid, nonce, nonce+"x", 1), strings.Replace(valid, "v1.", "v2.", 1),
		rewardRiskTestCookie(s, now.Unix(), "short"), strings.Repeat("x", 257),
	} {
		if s.rewardBrowserValue(value, now) != "" {
			t.Errorf("invalid or tampered cookie accepted: %q", value)
		}
	}
	other := &Service{cfg: Config{EncryptionKey: "fake-unit-test-different-cookie-key"}}
	if other.rewardBrowserValue(valid, now) != "" {
		t.Fatal("cookie signed under a different key was accepted")
	}
}

func TestRewardSignalsCookieLifecycle(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "fake-unit-test-signal-key", SessionCookieSecure: true}}
	r := httptest.NewRequest(http.MethodPost, "/auth/risk-context", nil)
	r.RemoteAddr = "8.8.8.8:443"
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "fake-existing-session"})
	w := httptest.NewRecorder()
	sig, err := s.rewardSignals(w, r)
	if err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one browser cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != rewardBrowserCookie || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 90*86400 {
		t.Fatalf("unexpected browser cookie security attributes: %+v", cookie)
	}
	if sig.BrowserHash == "" || sig.HTTPIPHash != s.riskHash("network", "8.8.8.8") || sig.RTCStatus != "unknown" || sig.RTCHashes == nil || len(sig.RTCHashes) != 0 {
		t.Fatalf("unexpected initial signals: %+v", sig)
	}
	if current, err := r.Cookie(rewardBrowserCookie); err != nil || current.Value != cookie.Value {
		t.Fatal("new browser cookie is unavailable for later checks on the same request")
	}
	if session, err := r.Cookie(sessionCookieName); err != nil || session.Value != "fake-existing-session" {
		t.Fatal("risk cookie creation discarded the current account session")
	}
	w = httptest.NewRecorder()
	again, err := s.rewardSignals(w, r)
	if err != nil || !reflect.DeepEqual(sig, again) || len(w.Result().Cookies()) != 0 {
		t.Fatalf("valid cookie was not reused: signals=%+v err=%v", again, err)
	}
	r = httptest.NewRequest(http.MethodPost, "/auth/risk-context", nil)
	r.RemoteAddr = "8.8.8.8:443"
	r.AddCookie(&http.Cookie{Name: rewardBrowserCookie, Value: cookie.Value + "tampered"})
	w = httptest.NewRecorder()
	replaced, err := s.rewardSignals(w, r)
	if err != nil || replaced.BrowserHash == "" || replaced.BrowserHash == sig.BrowserHash || len(w.Result().Cookies()) != 1 {
		t.Fatalf("tampered browser cookie was not replaced: signals=%+v err=%v", replaced, err)
	}
}

type rewardRiskUnitRow func(...any) error

func (row rewardRiskUnitRow) Scan(dest ...any) error { return row(dest...) }

type rewardRiskUnitTx struct {
	pgx.Tx
	calls int
	row   func(...any) pgx.Row
}

func (tx *rewardRiskUnitTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	tx.calls++
	if tx.row == nil {
		panic("unexpected risk database query")
	}
	return tx.row(args...)
}

func TestRewardContextValidationAndStorageFailures(t *testing.T) {
	s := &Service{}
	original := rewardRiskSignals{HTTPIPHash: "network-hash", BrowserHash: "browser-hash", RTCHashes: []string{}, RTCStatus: "unknown"}
	for _, id := range []string{"", "bad", "00000000-0000-0000-0000-00000000000x"} {
		tx := &rewardRiskUnitTx{}
		got, err := s.consumeRewardContextTx(context.Background(), tx, id, "register", original, "7")
		if id == "" && err != nil || id != "" && !errors.Is(err, errRiskContext) {
			t.Fatalf("context %q returned %v", id, err)
		}
		if tx.calls != 0 || !reflect.DeepEqual(got, original) {
			t.Fatalf("absent or malformed context changed signals or queried storage: %+v", got)
		}
	}
	id := "00000000-0000-4000-8000-000000000007"
	storageErr := errors.New("unit-test unavailable storage")
	for _, databaseErr := range []error{pgx.ErrNoRows, storageErr} {
		tx := &rewardRiskUnitTx{row: func(...any) pgx.Row {
			return rewardRiskUnitRow(func(...any) error { return databaseErr })
		}}
		got, err := s.consumeRewardContextTx(context.Background(), tx, id, "checkin", original, "7")
		want := databaseErr
		if errors.Is(databaseErr, pgx.ErrNoRows) {
			want = errRiskContext
		}
		if !errors.Is(err, want) || !reflect.DeepEqual(got, original) {
			t.Fatalf("context storage failure was not propagated safely: signals=%+v error=%v", got, err)
		}
	}
	var supplied []any
	tx := &rewardRiskUnitTx{row: func(args ...any) pgx.Row {
		supplied = args
		return rewardRiskUnitRow(func(dest ...any) error {
			*dest[0].(*[]string) = []string{"server-stored-rtc-hash"}
			*dest[1].(*string) = "completed"
			return nil
		})
	}}
	got, err := s.consumeRewardContextTx(context.Background(), tx, id, "checkin", original, "7")
	if err != nil || got.HTTPIPHash != original.HTTPIPHash || got.BrowserHash != original.BrowserHash || !slices.Equal(got.RTCHashes, []string{"server-stored-rtc-hash"}) || got.RTCStatus != "completed" {
		t.Fatalf("stored context was not merged with independent HTTP/browser evidence: %+v err=%v", got, err)
	}
	if !reflect.DeepEqual(supplied, []any{id, "checkin", original.BrowserHash, "7"}) {
		t.Fatalf("context consumption did not bind identity, purpose, browser and account: %v", supplied)
	}
}

func TestOAuthRiskContextLookupFailsClosedOnStorageErrors(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "fake-unit-test-oauth-risk-key"}}
	sig := rewardRiskSignals{HTTPIPHash: "network", BrowserHash: "browser", RTCHashes: []string{}, RTCStatus: "unknown"}
	storageErr := errors.New("unit-test storage unavailable")
	for _, tt := range []struct {
		name, outcome string
		want          error
	}{
		{"no optional context", "absent", nil},
		{"lookup database failure", "lookup-failure", storageErr},
		{"expired reused or mismatched context", "rejected", errRiskContext},
		{"consume database failure", "consume-failure", storageErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			step := 0
			tx := &rewardRiskUnitTx{row: func(args ...any) pgx.Row {
				step++
				if step == 1 {
					if !reflect.DeepEqual(args, []any{s.riskHash("oauth", "state-nonce")}) {
						t.Fatalf("OAuth context lookup did not use hashed nonce: %v", args)
					}
					return rewardRiskUnitRow(func(dest ...any) error {
						switch tt.outcome {
						case "absent":
							return pgx.ErrNoRows
						case "lookup-failure":
							return storageErr
						}
						*dest[0].(*string) = "00000000-0000-4000-8000-000000000007"
						return nil
					})
				}
				if !reflect.DeepEqual(args, []any{"00000000-0000-4000-8000-000000000007", "github", sig.BrowserHash}) {
					t.Fatalf("OAuth consumption omitted provider/browser binding: %v", args)
				}
				return rewardRiskUnitRow(func(...any) error {
					if tt.outcome == "rejected" {
						return pgx.ErrNoRows
					}
					return storageErr
				})
			}}
			got, err := s.consumeOAuthRiskContextTx(context.Background(), tx, "github", "state-nonce", sig, "7")
			if !errors.Is(err, tt.want) || !reflect.DeepEqual(got, sig) {
				t.Fatalf("OAuth context result=%+v error=%v, want error=%v", got, err, tt.want)
			}
		})
	}
}

func TestRewardRiskSettingsBoundaries(t *testing.T) {
	if err := validateRewardRiskSettings(defaultRewardRiskSettings()); err != nil {
		t.Fatalf("default settings invalid: %v", err)
	}
	for _, tt := range []struct {
		name   string
		change func(*rewardRiskSettings)
		valid  bool
	}{
		{"minimum windows", func(c *rewardRiskSettings) { c.WindowHours = 1; c.BurstMinutes = 1 }, true},
		{"maximum windows", func(c *rewardRiskSettings) { c.WindowHours = 720; c.BurstMinutes = 1440 }, true},
		{"zero observation window", func(c *rewardRiskSettings) { c.WindowHours = 0 }, false},
		{"observation window too long", func(c *rewardRiskSettings) { c.WindowHours = 721 }, false},
		{"zero burst window", func(c *rewardRiskSettings) { c.BurstMinutes = 0 }, false},
		{"burst window too long", func(c *rewardRiskSettings) { c.BurstMinutes = 1441 }, false},
		{"burst outside observation window", func(c *rewardRiskSettings) { c.WindowHours = 1; c.BurstMinutes = 61 }, false},
		{"similarity lower bound", func(c *rewardRiskSettings) { c.UsernameSimilarity = 0.7 }, true},
		{"similarity upper bound", func(c *rewardRiskSettings) { c.UsernameSimilarity = 1 }, true},
		{"similarity too low", func(c *rewardRiskSettings) { c.UsernameSimilarity = 0.699 }, false},
		{"similarity too high", func(c *rewardRiskSettings) { c.UsernameSimilarity = 1.001 }, false},
		{"similarity NaN", func(c *rewardRiskSettings) { c.UsernameSimilarity = math.NaN() }, false},
		{"similarity infinity", func(c *rewardRiskSettings) { c.UsernameSimilarity = math.Inf(1) }, false},
		{"similarity cluster too small", func(c *rewardRiskSettings) { c.SimilarAccounts = 2 }, false},
		{"maximum cohort", func(c *rewardRiskSettings) { c.SimilarAccounts = 500; c.BurstAccounts = 500 }, true},
		{"cohort too large", func(c *rewardRiskSettings) { c.SimilarAccounts = 501; c.BurstAccounts = 501 }, false},
		{"burst below similarity cohort", func(c *rewardRiskSettings) { c.BurstAccounts = 2 }, false},
		{"no STUN is supported", func(c *rewardRiskSettings) { c.STUNURLs = nil }, true},
		{"two STUN servers", func(c *rewardRiskSettings) {
			c.STUNURLs = []string{"stun:stun.example.test:3478", "stun:[2606:4700:4700::1111]:65535"}
		}, true},
		{"too many STUN servers", func(c *rewardRiskSettings) {
			c.STUNURLs = []string{"stun:one.test:1", "stun:two.test:2", "stun:three.test:3"}
		}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultRewardRiskSettings()
			tt.change(&cfg)
			if err := validateRewardRiskSettings(cfg); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v, want valid=%v", err == nil, err, tt.valid)
			}
		})
	}
	limits := []struct {
		name string
		set  func(*rewardRiskSettings, int)
	}{
		{"registrations", func(c *rewardRiskSettings, n int) { c.RegistrationsPerIP = n }},
		{"IP checkins", func(c *rewardRiskSettings, n int) { c.CheckinsPerIP = n }},
		{"browser checkins", func(c *rewardRiskSettings, n int) { c.CheckinsPerBrowser = n }},
		{"invitations", func(c *rewardRiskSettings, n int) { c.InvitationsPerInviter = n }},
	}
	for _, limit := range limits {
		for _, n := range []int{0, 1, 10000, 10001} {
			cfg := defaultRewardRiskSettings()
			limit.set(&cfg, n)
			valid := n >= 1 && n <= 10000
			if err := validateRewardRiskSettings(cfg); (err == nil) != valid {
				t.Errorf("%s=%d validation error=%v", limit.name, n, err)
			}
		}
	}
	for _, raw := range []string{"https://stun.example.test:3478", "turn:stun.example.test:3478", "stun:user@stun.example.test:3478", "stun:host:0", "stun:host:65536", "stun:host", "stun::3478", "stun:host/path:3478", "stun:host?x:3478", "stun:host#x:3478", "stun:host name:3478"} {
		cfg := defaultRewardRiskSettings()
		cfg.STUNURLs = []string{raw}
		if validateRewardRiskSettings(cfg) == nil {
			t.Errorf("invalid STUN URL accepted: %q", raw)
		}
	}
}

func TestLoadRewardRiskSettingsDefaultsAndFailClosed(t *testing.T) {
	storageErr := errors.New("unit-test missing settings")
	for _, tt := range []struct {
		name, raw string
		dbError   error
		valid     bool
	}{
		{"partial settings preserve defaults", `{"enabled":false}`, nil, true},
		{"malformed JSON", `{`, nil, false},
		{"invalid persisted settings", `{"window_hours":0}`, nil, false},
		{"missing settings", "", storageErr, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := &rewardRiskUnitTx{row: func(...any) pgx.Row {
				return rewardRiskUnitRow(func(dest ...any) error {
					if tt.dbError != nil {
						return tt.dbError
					}
					*dest[0].(*[]byte) = []byte(tt.raw)
					return nil
				})
			}}
			cfg, err := loadRewardRiskSettings(context.Background(), q)
			if (err == nil) != tt.valid {
				t.Fatalf("settings loaded=%v error=%v, want loaded=%v", err == nil, err, tt.valid)
			}
			if tt.valid && (cfg.Enabled || cfg.WindowHours != defaultRewardRiskSettings().WindowHours || cfg.SimilarAccounts != defaultRewardRiskSettings().SimilarAccounts) {
				t.Fatalf("partial settings did not preserve explicit values and defaults: %+v", cfg)
			}
			if tt.dbError != nil && !errors.Is(err, tt.dbError) {
				t.Fatalf("storage failure not propagated: %v", err)
			}
		})
	}
}

func TestRewardRiskAdminFiltersRejectInvalidChoices(t *testing.T) {
	for _, tt := range []struct{ kind, query string }{
		{"claims", "status=allow"},
		{"claims", "source=payment"},
		{"events", "decision=credited"},
		{"bans", "status=pending"},
		{"claims", "q=" + strings.Repeat("x", 201)},
	} {
		r := httptest.NewRequest(http.MethodGet, "/admin/reward-risk?"+tt.query, nil)
		if _, _, err := rewardRiskListWhere(r, tt.kind); err == nil {
			t.Errorf("invalid admin filter accepted: kind=%s query=%s", tt.kind, tt.query)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/admin/reward-risk?q=alice&status=pending&source=checkin", nil)
	_, args, err := rewardRiskListWhere(r, "claims")
	if err != nil || !reflect.DeepEqual(args, []any{"%alice%", "alice", "pending", "checkin"}) {
		t.Fatalf("valid filters were not bound as parameters: args=%v error=%v", args, err)
	}
}

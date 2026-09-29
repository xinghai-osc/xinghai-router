package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const rewardBrowserCookie = "xinghai.risk-browser"

var errRiskContext = errors.New("invalid or expired risk context")
var errRiskAccountRestricted = errors.New("account is restricted")

type rewardRiskSignals struct {
	HTTPIPHash  string
	BrowserHash string
	RTCHashes   []string
	RTCStatus   string
}

func (s *Service) riskHash(purpose, value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.EncryptionKey))
	mac.Write([]byte("xinghai.reward-risk.v1\x00" + purpose + "\x00" + value))
	return "v1:" + hex.EncodeToString(mac.Sum(nil))
}

func publicRiskAddress(raw string) (netip.Addr, bool) {
	a, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || a.Zone() != "" {
		return netip.Addr{}, false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return netip.Addr{}, false
	}
	for _, p := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "198.18.0.0/15", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(p).Contains(a) {
			return netip.Addr{}, false
		}
	}
	return a, true
}

func riskNetwork(raw string) string {
	a, ok := publicRiskAddress(raw)
	if !ok {
		return ""
	}
	if a.Is6() {
		return netip.PrefixFrom(a, 64).Masked().String()
	}
	return a.String()
}

func (s *Service) rewardBrowserValue(value string, now time.Time) string {
	if len(value) > 256 {
		return ""
	}
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != "v1" || len(parts[2]) < 20 {
		return ""
	}
	issued, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || issued <= 0 || issued > now.Unix()+60 || issued < now.Unix()-90*86400 {
		return ""
	}
	expected := s.riskHash("cookie", strings.Join(parts[:3], "."))
	if !hmac.Equal([]byte(expected), []byte(parts[3])) {
		return ""
	}
	return s.riskHash("browser", parts[2])
}

func (s *Service) rewardSignals(w http.ResponseWriter, r *http.Request) (rewardRiskSignals, error) {
	sig := rewardRiskSignals{HTTPIPHash: s.riskHash("network", riskNetwork(requestMetadata(r).clientIP)), RTCHashes: []string{}, RTCStatus: "unknown"}
	now := time.Now()
	if c, err := r.Cookie(rewardBrowserCookie); err == nil {
		sig.BrowserHash = s.rewardBrowserValue(c.Value, now)
	}
	if sig.BrowserHash == "" {
		nonce, err := randomSecret("")
		if err != nil {
			return sig, err
		}
		value := "v1." + strconv.FormatInt(now.Unix(), 10) + "." + nonce
		value += "." + s.riskHash("cookie", value)
		cookie := &http.Cookie{Name: rewardBrowserCookie, Value: value, Path: "/", HttpOnly: true, Secure: s.cfg.SessionCookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 90 * 86400, Expires: now.Add(90 * 24 * time.Hour)}
		http.SetCookie(w, cookie)
		cookies := r.Cookies()
		r.Header.Del("Cookie")
		for _, existing := range cookies {
			if existing.Name != rewardBrowserCookie {
				r.AddCookie(existing)
			}
		}
		r.AddCookie(cookie)
		sig.BrowserHash = s.rewardBrowserValue(value, now)
	}
	return sig, nil
}

func (s *Service) createRewardRiskContext(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Purpose   string   `json:"purpose"`
		Status    string   `json:"status"`
		Addresses []string `json:"addresses"`
	}
	if decode(r, &in) != nil || len(in.Addresses) > 8 {
		writeError(w, 400, "invalid_request", "invalid risk context")
		return
	}
	if in.Purpose != "register" && in.Purpose != "oauth" && in.Purpose != "checkin" {
		writeError(w, 400, "invalid_request", "invalid risk context purpose")
		return
	}
	switch in.Status {
	case "completed", "timeout", "unsupported", "error", "disabled":
	default:
		writeError(w, 400, "invalid_request", "invalid collection status")
		return
	}
	userID := accountFromContext(r).userID
	if in.Purpose == "checkin" && userID == "" {
		writeError(w, 401, "unauthorized", "sign in to check in")
		return
	}
	allowed, err := allowRateLimit(s.limiter, "risk:context:"+clientIP(r), 20)
	if err != nil {
		writeError(w, 503, "rate_limiter_unavailable", "risk service unavailable")
		return
	}
	if !allowed {
		writeError(w, 429, "rate_limit_exceeded", "too many risk context requests")
		return
	}
	cfg, err := loadRewardRiskSettings(r.Context(), s.db)
	if err != nil {
		writeError(w, 503, "risk_unavailable", "risk service unavailable")
		return
	}
	sig, err := s.rewardSignals(w, r)
	if err != nil {
		writeError(w, 500, "internal_error", "could not create risk context")
		return
	}
	seen := map[string]bool{}
	hashes := []string{}
	for _, raw := range in.Addresses {
		if len(raw) > 64 {
			writeError(w, 400, "invalid_request", "invalid candidate address")
			return
		}
		a, parseErr := netip.ParseAddr(raw)
		if parseErr != nil || a.Zone() != "" {
			writeError(w, 400, "invalid_request", "invalid candidate address")
			return
		}
		a, ok := publicRiskAddress(raw)
		if !ok {
			continue
		}
		h := s.riskHash("rtc", a.String())
		if !seen[h] {
			hashes = append(hashes, h)
			seen[h] = true
		}
	}
	if !cfg.Enabled || !cfg.WebRTCEnabled {
		hashes = []string{}
		in.Status = "disabled"
	}
	sort.Strings(hashes)
	id, err := randomID()
	if err != nil {
		writeError(w, 500, "internal_error", "could not create risk context")
		return
	}
	var expires time.Time
	err = s.db.QueryRow(r.Context(), `insert into risk_contexts(id,purpose,user_id,browser_hash,rtc_hashes,rtc_status,expires_at) values($1,$2,nullif($3,'')::bigint,$4,$5,$6,clock_timestamp()+interval '5 minutes') returning expires_at`, id, in.Purpose, func() string {
		if in.Purpose == "checkin" {
			return userID
		}
		return ""
	}(), sig.BrowserHash, hashes, in.Status).Scan(&expires)
	if err != nil {
		writeError(w, 500, "internal_error", "could not create risk context")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "expires_at": expires})
}

func (s *Service) consumeRewardContextTx(ctx context.Context, tx pgx.Tx, id, purpose string, sig rewardRiskSignals, userID string) (rewardRiskSignals, error) {
	if id == "" {
		return sig, nil
	}
	if !validRiskUUID(id) {
		return sig, errRiskContext
	}
	err := tx.QueryRow(ctx, `update risk_contexts set consumed_at=clock_timestamp() where id=$1 and purpose=$2 and browser_hash=$3 and consumed_at is null and oauth_nonce_hash is null and expires_at>clock_timestamp() and (user_id is null or user_id=nullif($4,'')::bigint) returning rtc_hashes,rtc_status`, id, purpose, sig.BrowserHash, userID).Scan(&sig.RTCHashes, &sig.RTCStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return sig, errRiskContext
	}
	return sig, err
}

func validRiskUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func (s *Service) bindOAuthRiskContext(ctx context.Context, id, provider, stateNonce string, sig rewardRiskSignals) error {
	if id == "" {
		return nil
	}
	if !validRiskUUID(id) {
		return errRiskContext
	}
	tag, err := s.db.Exec(ctx, `update risk_contexts set oauth_nonce_hash=$1,oauth_provider=$2,expires_at=clock_timestamp()+interval '10 minutes' where id=$3 and purpose='oauth' and browser_hash=$4 and oauth_nonce_hash is null and consumed_at is null and expires_at>clock_timestamp()`, s.riskHash("oauth", stateNonce), provider, id, sig.BrowserHash)
	if err == nil && tag.RowsAffected() != 1 {
		return errRiskContext
	}
	return err
}

func (s *Service) consumeOAuthRiskContextTx(ctx context.Context, tx pgx.Tx, provider, stateNonce string, sig rewardRiskSignals, userID string) (rewardRiskSignals, error) {
	var id string
	err := tx.QueryRow(ctx, `select id::text from risk_contexts where oauth_nonce_hash=$1`, s.riskHash("oauth", stateNonce)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sig, nil
	}
	if err != nil {
		return sig, err
	}
	err = tx.QueryRow(ctx, `update risk_contexts set consumed_at=clock_timestamp() where id=$1 and purpose='oauth' and oauth_provider=$2 and browser_hash=$3 and consumed_at is null and expires_at>clock_timestamp() returning rtc_hashes,rtc_status`, id, provider, sig.BrowserHash).Scan(&sig.RTCHashes, &sig.RTCStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return sig, errRiskContext
	}
	return sig, err
}

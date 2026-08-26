//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationEncryptionKey = "integration-test-encryption-key-2026"

func integrationPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; run with -tags integration against an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := newPool(ctx, Config{DatabaseURL: dsn, DBMaxConns: 16})
	if err != nil {
		t.Fatalf("newPool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	return pool, dsn
}

func resetIntegrationDatabase(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.Exec(ctx, `drop schema public cascade; create schema public`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("migrate empty database: %v", err)
	}
}

func integrationService(t *testing.T, db *pgxpool.Pool) *Service {
	t.Helper()
	s := &Service{
		cfg: Config{DatabaseURL: os.Getenv("TEST_DATABASE_URL"), EncryptionKey: integrationEncryptionKey, ConversationCacheDir: t.TempDir()},
		db:  db, httpClient: http.DefaultClient, streamClient: http.DefaultClient,
		limiter: newMemoryLimiter(1000), ipLimiter: newMemoryLimiter(1000),
		groupLimiter: NewGroupLimiter(), userLimiter: NewGroupLimiter(),
		pricingCache:      newTTLCache[string, pricingRule](time.Minute),
		channelCache:      newTTLCache[channelRouteKey, []channel](time.Minute),
		channelKeyCache:   newTTLCache[int64, []channelKeyCredential](time.Minute),
		subscriptionCache: newTTLCache[subscriptionRouteKey, subscriptionAccess](time.Minute),
		keyTouchCache:     newTTLCache[string, struct{}](time.Minute),
		background:        newBackgroundWriter(),
	}
	t.Cleanup(func() { s.background.close(); s.limiter.close(); s.ipLimiter.close() })
	return s
}

func integrationUser(t *testing.T, db *pgxpool.Pool, email string, balance float64) (string, string) {
	t.Helper()
	ctx := context.Background()
	var userID string
	passwordHash, err := hashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.QueryRow(ctx, `insert into users(email,name,password_hash) values($1,$2,$3) returning id`, email, strings.Split(email, "@")[0], passwordHash).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(ctx, `insert into user_wallets(user_id,balance) values($1,$2)`, userID, balance); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}
	return userID, strings.Split(email, "@")[0]
}

func integrationKey(t *testing.T, db *pgxpool.Pool, userID, secret string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(context.Background(), `insert into api_keys(id,user_id,name,key_prefix,secret_hash,secret_encrypted) values(gen_random_uuid(),$1,'integration', $2,$3,$4) returning id`, userID, secret[:12], hashSecret(secret), mustCrypt(t, secret)).Scan(&id); err != nil {
		t.Fatalf("insert API key: %v", err)
	}
	return id
}

func mustCrypt(t *testing.T, value string) string {
	t.Helper()
	out, err := crypt(integrationEncryptionKey, value, false)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}
	return out
}

func TestIntegrationMigrateEmptyDatabaseAndIdempotency(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	var first int
	if err := db.QueryRow(context.Background(), `select count(*) from schema_migrations`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first < 80 {
		t.Fatalf("only %d migrations applied", first)
	}
	if err := migrate(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var second int
	if err := db.QueryRow(context.Background(), `select count(*) from schema_migrations`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("migration count changed on repeat: %d -> %d", first, second)
	}
}

func TestIntegrationWalletReservationReleaseAndChargeConcurrent(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "wallet-integration@example.com", 1)
	keyID := integrationKey(t, db, userID, "sk-xh-wallet-integration")
	s := integrationService(t, db)
	key := keyContext{userID: userID, keyID: keyID}
	pricing := pricingRule{found: true, input: 1000, output: 1000, multiplier: 1}
	const requests = 20
	results := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.reserveUsage(context.Background(), key, "integration", 300, 1000, pricing, 1)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	reserved := 0
	for err := range results {
		if err == nil {
			reserved++
		}
	}
	if reserved == 0 || reserved >= requests {
		t.Fatalf("unexpected reservations: %d", reserved)
	}
	var held float64
	if err := db.QueryRow(context.Background(), `select reserved from user_wallets where user_id=$1`, userID).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held <= 0 || held > 1 {
		t.Fatalf("reserved=%v", held)
	}
	// Release every successful hold concurrently; this exercises the detached release path.
	var release sync.WaitGroup
	for i := 0; i < reserved; i++ {
		release.Add(1)
		go func() {
			defer release.Done()
			s.releaseReservation(context.Background(), key, reservation{amount: held / float64(reserved)})
		}()
	}
	release.Wait()
	var after float64
	if err := db.QueryRow(context.Background(), `select reserved from user_wallets where user_id=$1`, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after > 0.000001 {
		t.Fatalf("reserved after release=%v", after)
	}
}

func TestIntegrationPaymentNotifyIsIdempotentConcurrently(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "payment-integration@example.com", 0)
	order := "integration-payment-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.Exec(context.Background(), `insert into payment_orders(id,order_no,user_id,provider,payment_type,amount) values(gen_random_uuid(),$1,$2,'epay','alipay',10.00)`, order, userID); err != nil {
		t.Fatal(err)
	}
	merchantKey := "integration-merchant-key"
	if _, err := db.Exec(context.Background(), `update payment_settings set enabled=true,base_url='https://pay.example.test',merchant_id='1001',merchant_key_encrypted=$1,public_base_url='https://router.example.test' where provider='epay'`, mustCrypt(t, merchantKey)); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	values := url.Values{"pid": {"1001"}, "trade_status": {"TRADE_SUCCESS"}, "out_trade_no": {order}, "money": {"10.00"}, "trade_no": {"trade-" + order}}
	values.Set("sign", epaySign(values, merchantKey))
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodPost, "/payments/epay/notify", strings.NewReader(values.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			s.epayNotify(rec, r)
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("notify status=%d", code)
		}
	}
	var balance float64
	var ledger int
	if err := db.QueryRow(context.Background(), `select balance from user_wallets where user_id=$1`, userID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(context.Background(), `select count(*) from wallet_ledger where user_id=$1 and request_id=$2`, userID, order).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if balance != 10 || ledger != 1 {
		t.Fatalf("balance=%v ledger=%d", balance, ledger)
	}
}

func TestIntegrationMockOpenAIAndAnthropicUpstreams(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"mock-model","object":"model"}]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-mock","model":"mock-model","choices":[{"index":0,"message":{"role":"assistant","content":"mock response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
	}))
	defer upstream.Close()
	userID, _ := integrationUser(t, db, "upstream-integration@example.com", 100)
	secret := "sk-xh-upstream-integration"
	keyID := integrationKey(t, db, userID, secret)
	var channelID string
	if err := db.QueryRow(context.Background(), `insert into channels(id,name,base_url,api_key,models,provider) values(gen_random_uuid(),'mock', $1,'upstream-key','["mock-model"]','openai') returning id`, upstream.URL).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'mock-model',1,1)`); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	s.httpClient = upstream.Client()
	s.streamClient = upstream.Client()
	for _, tc := range []struct{ name, path, contentType string }{{"openai", "/v1/chat/completions", "application/json"}, {"anthropic", "/v1/messages", "application/json"}} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"max_tokens":8}`
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+secret)
			req.Header.Set("X-API-Key", secret)
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Anthropic-Version", "2023-06-01")
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !json.Valid(rec.Body.Bytes()) {
				t.Fatalf("invalid JSON: %s", rec.Body.String())
			}
		})
	}
	_ = channelID
	_ = keyID
}

func TestIntegrationAuthCreateKeyAndModelsE2E(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	_, _ = integrationUser(t, db, "e2e-integration@example.com", 0)
	s := integrationService(t, db)
	s.cfg.EncryptionKey = integrationEncryptionKey
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"e2e-integration@example.com","password":"password123"}`))
	s.login(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("login response=%s", login.Body.String())
	}
	create := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/account/keys", strings.NewReader(`{"name":"e2e"}`))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	s.Handler().ServeHTTP(create, req)
	if create.Code != http.StatusCreated {
		t.Fatalf("create key status=%d body=%s", create.Code, create.Body.String())
	}
	var key struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &key); err != nil || key.Key == "" {
		t.Fatalf("key response=%s", create.Body.String())
	}
	models := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	s.Handler().ServeHTTP(models, req)
	if models.Code != http.StatusOK {
		t.Fatalf("models status=%d body=%s", models.Code, models.Body.String())
	}
}

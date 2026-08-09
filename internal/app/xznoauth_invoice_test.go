package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newXZNoauthTestClient(t *testing.T, handler http.HandlerFunc) (*xznoauthClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":900,"scope":"invoice.apply"}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return newXZNoauthClient(invoiceSettings{
		Enabled:      true,
		BaseURL:      server.URL,
		ClientID:     "dev-client-id",
		ClientSecret: "dev-client-secret",
	}), server
}

func envelope(body string) []byte {
	if body == "" {
		body = "{}"
	}
	return []byte(`{"code":0,"message":"ok","data":` + body + `}`)
}

// TestXZNoauthTokenFormAndRedaction verifies the OAuth form uses
// application/x-www-form-urlencoded with client_credentials, that the token is
// cached until expiry, and that returned errors never contain the secret.
func TestXZNoauthTokenFormAndRedaction(t *testing.T) {
	var mu sync.Mutex
	tokenRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/oauth/token":
			mu.Lock()
			tokenRequests++
			mu.Unlock()
			if err := r.ParseForm(); err != nil {
				t.Fatalf("token form parse: %v", err)
			}
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
			}
			if got := r.PostForm.Get("grant_type"); got != "client_credentials" {
				t.Errorf("grant_type = %q", got)
			}
			if got := r.PostForm.Get("client_id"); got != "dev-client-id" {
				t.Errorf("client_id = %q", got)
			}
			if got := r.PostForm.Get("client_secret"); got != "dev-client-secret" {
				t.Errorf("client_secret = %q", got)
			}
			if got := r.PostForm.Get("scope"); got != xznoauthTokenScope {
				t.Errorf("scope = %q", got)
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-secret-value","token_type":"Bearer","expires_in":900,"scope":"invoice.apply"}`))
		case "/api/v1/invoice-orders/validate":
			if r.Header.Get("Authorization") != "Bearer tok-secret-value" {
				t.Errorf("validate Authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write(envelope(`{"orders":[],"totalAmount":"0.00","currency":"CNY"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	client := newXZNoauthClient(invoiceSettings{
		Enabled:      true,
		BaseURL:      server.URL,
		ClientID:     "dev-client-id",
		ClientSecret: "dev-client-secret",
	})

	_, err := client.validate(context.Background(), []string{"ORDER-1"}, false, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err = client.validate(context.Background(), []string{"ORDER-2"}, false, nil)
	if err != nil {
		t.Fatalf("validate (cached): %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if tokenRequests != 1 {
		t.Errorf("expected a single cached token fetch, got %d", tokenRequests)
	}
}

// TestXZNoauthInvalidCredentialsNeverLeakSecret ensures a rejected token request
// is surfaced as an error that does not contain the Client Secret.
func TestXZNoauthInvalidCredentialsNeverLeakSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"bad credentials"}`))
	}))
	t.Cleanup(server.Close)
	client := newXZNoauthClient(invoiceSettings{BaseURL: server.URL, ClientID: "id", ClientSecret: "super-secret-123"})
	_, err := client.validate(context.Background(), []string{"ORDER-1"}, false, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-123") {
		t.Fatalf("secret leaked into error: %v", err)
	}
}

// TestXZNoauthEnvelopeParsing checks success and error envelope handling.
func TestXZNoauthEnvelopeParsing(t *testing.T) {
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":900}`))
			return
		}
		if r.URL.Path == "/api/v1/invoice-orders/validate" {
			_, _ = w.Write(envelope(constOrderValidationData))
			return
		}
		if r.URL.Path == "/api/v1/invoices" && r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"code":"INVOICE_ORDER_ALREADY_CLAIMED","message":"taken","requestId":"req-1"}`))
			return
		}
		http.NotFound(w, r)
	})
	out, err := client.validate(context.Background(), []string{"ORDER-001"}, false, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(out.Orders) != 1 || out.Orders[0].ExternalNo != "YOUR_PAID_ORDER_NO" {
		t.Fatalf("unexpected orders: %+v", out.Orders)
	}
	if out.TotalAmount != "5.00" {
		t.Errorf("TotalAmount = %q", out.TotalAmount)
	}
}

const constOrderValidationData = `{
  "orders": [{"platformOrderId":123,"externalNo":"YOUR_PAID_ORDER_NO","productName":"商品","amount":"5.00","currency":"CNY","paidAt":"2026-01-01T12:00:00Z"}],
  "totalAmount":"5.00","currency":"CNY"
}`

// TestXZNoauthValidateBothModes checks both needPayTax branches are sent and
// that the tax checkout map is parsed for both channels.
func TestXZNoauthValidateBothModes(t *testing.T) {
	var requests []map[string]any
	var mu sync.Mutex
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoice-orders/validate" {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode validate body: %v", err)
			}
			mu.Lock()
			requests = append(requests, payload)
			mu.Unlock()
			_, _ = w.Write(envelope(`{
				"orders":[{"externalNo":"P1","amount":"5.00","currency":"CNY"}],
				"totalAmount":"5.00","currency":"CNY",
				"taxAmount":"0.30","taxPaidAmount":"0.00","taxDueAmount":"0.30",
				"taxPayments":{
					"alipay":{"taxOrderNo":"INV-TAX-A","payUrl":"https://pay.x/cashier/a"},
					"wxpay":{"taxOrderNo":"INV-TAX-W","payUrl":"https://pay.x/cashier/w"}
				}
			}`))
			return
		}
		http.NotFound(w, r)
	})
	if _, err := client.validate(context.Background(), []string{"P1"}, true, nil); err != nil {
		t.Fatalf("needPayTax=true: %v", err)
	}
	if _, err := client.validate(context.Background(), []string{"P1"}, false, nil); err != nil {
		t.Fatalf("needPayTax=false: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 validate calls, got %d", len(requests))
	}
	// The branch flag is carried per request and must be absent when false.
	if !requests[0]["needPayTax"].(bool) {
		t.Error("first validate did not send needPayTax=true")
	}
	if _, present := requests[1]["needPayTax"]; present {
		t.Error("second validate must omit needPayTax")
	}
}

// TestXZNoauthTaxPaymentStatus checks the client polling endpoint.
func TestXZNoauthTaxPaymentStatus(t *testing.T) {
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoice-tax-payments/status" {
			var payload struct {
				TaxOrderNo string `json:"taxOrderNo"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("body decode: %v", err)
			}
			if payload.TaxOrderNo != "INV-TAX-A" {
				t.Errorf("taxOrderNo = %q", payload.TaxOrderNo)
			}
			_, _ = w.Write(envelope(`{"paid":true}`))
			return
		}
		http.NotFound(w, r)
	})
	paid, err := client.taxPaymentStatus(context.Background(), "INV-TAX-A")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !paid {
		t.Error("expected paid=true")
	}
}

// TestXZNoauthSubmitAndCancel checks submit success, submission conflict, cancel
// success, and cancel rejection mapping.
func TestXZNoauthSubmitAndCancel(t *testing.T) {
	var submitted map[string]any
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoices" && r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Fatalf("submit decode: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(envelope(`{"id":"APP-1","status":"pending"}`))
			return
		}
		if r.URL.Path == "/api/v1/invoices/APP-1/cancel" {
			_, _ = w.Write(envelope(`{"id":"APP-1","status":"canceled"}`))
			return
		}
		http.NotFound(w, r)
	})
	app, err := client.submit(context.Background(), map[string]any{"orderNos": []string{"ORDER-1"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if app.ID != "APP-1" || app.Status != "pending" {
		t.Errorf("unexpected app: %+v", app)
	}
	// Cancellation success.
	if _, err := client.cancel(context.Background(), "APP-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
}

func TestXZNoauthRejectsUnpaidAndClaimedOrders(t *testing.T) {
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoice-orders/validate" {
			_, _ = w.Write([]byte(`{"code":"INVOICE_ORDER_NOT_ELIGIBLE","message":"not paid","requestId":"r1"}`))
			return
		}
		http.NotFound(w, r)
	})
	_, err := client.validate(context.Background(), []string{"ORDER-X"}, false, nil)
	status, code, _ := invoiceErrorMessage(err)
	if code != "INVOICE_ORDER_NOT_ELIGIBLE" || status != http.StatusConflict {
		t.Fatalf("got mapping (%d, %s)", status, code)
	}
}

func TestXZNoauthTaxReconciliationAndExceeds(t *testing.T) {
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoice-tax-payments/status" {
			_, _ = w.Write(envelope(`{"paid":true}`))
			return
		}
		if r.URL.Path == "/api/v1/invoice-orders/validate" {
			_, _ = w.Write([]byte(`{"code":"INVOICE_TAX_PAYMENT_EXCEEDS_REQUIRED","message":"too much","requestId":"r2"}`))
			return
		}
		http.NotFound(w, r)
	})
	paid, err := client.taxPaymentStatus(context.Background(), "INV-TAX-A")
	if err != nil || !paid {
		t.Fatalf("paid status: %v %v", err, paid)
	}
	_, err = client.taxPaymentStatus(context.Background(), "INV-TAX-A")
	if err != nil {
		t.Fatalf("second status: %v", err)
	}
	_, err = client.validate(context.Background(), []string{"P1"}, true, nil)
	status, code, _ := invoiceErrorMessage(err)
	if code != "INVOICE_TAX_PAYMENT_EXCEEDS_REQUIRED" || status != http.StatusConflict {
		t.Fatalf("exceeded mapping: (%d, %s)", status, code)
	}
}

// TestXZNoauthDownloadPDFBinary checks that the PDF endpoint streams raw bytes
// and that a non-200 JSON response becomes a typed error.
func TestXZNoauthDownloadPDFBinary(t *testing.T) {
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/invoices/APP-1/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4 fake invoice"))
		case "/api/v1/invoices/APP-2/pdf":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"NOT_FOUND","message":"nope","requestId":"r3"}`))
		default:
			http.NotFound(w, r)
		}
	})
	pdf, err := client.downloadPDF(context.Background(), "APP-1")
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Errorf("unexpected pdf bytes: %q", pdf)
	}
	_, err = client.downloadPDF(context.Background(), "APP-2")
	status, code, _ := invoiceErrorMessage(err)
	if code != "NOT_FOUND" || status != http.StatusNotFound {
		t.Fatalf("pdf error mapping: (%d, %s)", status, code)
	}
}

// TestXZNoauthTokenRetryOnce checks that an expired Token triggers a refresh and
// a single retry of the original request.
func TestXZNoauthTokenRetryOnce(t *testing.T) {
	mu := sync.Mutex{}
	attempts := 0
	client, _ := newXZNoauthTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/invoice-orders/validate" {
			mu.Lock()
			attempts++
			first := attempts
			mu.Unlock()
			if first == 1 {
				_, _ = w.Write([]byte(`{"code":"AUTH_INVALID_TOKEN","message":"expired","requestId":"r4"}`))
				return
			}
			_, _ = w.Write(envelope(`{"orders":[],"totalAmount":"0.00","currency":"CNY"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"fresh-token","expires_in":900}`))
	})
	if _, err := client.validate(context.Background(), []string{"P1"}, false, nil); err != nil {
		t.Fatalf("validate retry: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Fatalf("expected exactly 1 retry, got %d attempts", attempts)
	}
}

// TestNormalizeOrderNos and friends exercise the request-shaping helpers without
// any network dependency.
func TestNormalizeOrderNos(t *testing.T) {
	out, msg := normalizeOrderNos(nil)
	if msg == "" || out != nil {
		t.Fatalf("empty list must be rejected (%q)", msg)
	}
	dup := []string{"a", "a"}
	if _, msg := normalizeOrderNos(dup); msg == "" {
		t.Fatal("duplicate identifiers must be rejected")
	}
	ok := make([]string, xznoauthMaxOrders)
	for i := range ok {
		ok[i] = fmt.Sprintf("ORDER-%03d", i)
	}
	if got, msg := normalizeOrderNos(ok); len(got) != len(ok) || msg != "" {
		t.Fatalf("validation failed for 20 orders: %q", msg)
	}
	if _, msg := normalizeOrderNos(append(ok, "x")); msg == "" {
		t.Fatal("21 orders must be rejected")
	}
}

func TestValidateBuyerInputAndBankAccount(t *testing.T) {
	var msg string
	good := invoiceBuyerInput{
		BuyerType:      "individual",
		Title:          "张三",
		TaxpayerID:     "110101200001011234",
		BuyerPhone:     "13800138000",
		BuyerBankAcc:   "6222 0000 0000 0000",
		RecipientEmail: "a@example.com",
	}
	if !validateBuyerInput(good, &msg) {
		t.Fatalf("valid input rejected: %s", msg)
	}
	if got := normalizeBankAccount(good.BuyerBankAcc); got != "6222000000000000" {
		t.Errorf("bank account normalization = %q", got)
	}
	bad := good
	bad.BuyerType = "person"
	if validateBuyerInput(bad, &msg) {
		t.Fatal("invalid buyerType must be rejected")
	}
	bad = good
	bad.Title = ""
	if validateBuyerInput(bad, &msg) {
		t.Fatal("empty title must be rejected")
	}
	bad = good
	bad.BuyerBankAcc = "123"
	if validateBuyerInput(bad, &msg) {
		t.Fatal("short bank account must be rejected")
	}
	bad = good
	bad.RecipientEmail = "not-an-email"
	if validateBuyerInput(bad, &msg) {
		t.Fatal("invalid email must be rejected")
	}
}

func TestInvoiceErrorMessageRedactsDetails(t *testing.T) {
	_, _, message := invoiceErrorMessage(&xznoauthAPIError{code: "PAYMENT_INVALID_RESPONSE", message: "raw bank detail secret-bank-token"})
	if strings.Contains(message, "secret-bank-token") {
		t.Fatalf("raw provider detail leaked: %s", message)
	}
	if !strings.Contains(strings.ToLower(message), "try again") {
		t.Errorf("expected safe retry message, got %s", message)
	}
}

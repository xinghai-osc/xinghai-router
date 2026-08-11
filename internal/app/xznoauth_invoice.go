package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	xznoauthDefaultBaseURL = "https://oauth.xzncraft.cn"
	xznoauthTokenScope     = "invoice.apply"
	xznoauthMaxOrders      = 20
	xznoauthTimeout        = 30 * time.Second
	maxInvoicePDFSize      = 32 << 20

	maxInvoiceIdentifier     = 128
	maxInvoiceTitleLen       = 255
	maxInvoiceTaxIDLen       = 32
	maxInvoiceBuyerFieldLen  = 255
	maxInvoicePhoneLen       = 64
	maxInvoiceBankAccountLen = 32
	minInvoiceBankAccountLen = 8
)

// invoiceSettings is the server-side XZNoAuth developer-application configuration.
// The Client Secret is encrypted at rest like the epay merchant key.
type invoiceSettings struct {
	Enabled         bool   `json:"enabled"`
	BaseURL         string `json:"base_url"`
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"-"`
	HasClientSecret bool   `json:"has_client_secret"`
	NeedPayTax      bool   `json:"need_pay_tax"`
}

func (settings invoiceSettings) ready() bool {
	return settings.Enabled && settings.BaseURL != "" && settings.ClientID != "" && settings.ClientSecret != ""
}

func (s *Service) loadInvoiceSettings(ctx context.Context) (invoiceSettings, error) {
	var settings invoiceSettings
	var encrypted string
	err := s.db.QueryRow(ctx, `select enabled,base_url,client_id,client_secret_encrypted,need_pay_tax from invoice_settings where id=1`).Scan(&settings.Enabled, &settings.BaseURL, &settings.ClientID, &encrypted, &settings.NeedPayTax)
	if err != nil {
		return settings, err
	}
	settings.HasClientSecret = encrypted != ""
	if encrypted != "" {
		if settings.ClientSecret, err = crypt(s.cfg.EncryptionKey, encrypted, true); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

// xznoauthAPIError carries the XZNoAuth error envelope fields plus the HTTP status.
type xznoauthAPIError struct {
	status    int
	code      string
	message   string
	requestID string
}

func (e *xznoauthAPIError) Error() string {
	return fmt.Sprintf("xznoauth %s (%d): %s", e.code, e.status, e.message)
}

// retryable reports a failed client-authentication attempt: an expired or
// invalidated Token, a missing scope, or a revoked developer application.
func (e *xznoauthAPIError) retryable() bool {
	if e.status == http.StatusUnauthorized {
		return true
	}
	switch e.code {
	case "AUTH_INVALID_TOKEN", "AUTH_INSUFFICIENT_SCOPE", "AUTH_FORBIDDEN":
		return true
	default:
		return false
	}
}

// xznoauthEnvelope is the shared JSON response wrapper. code is 0 on success and
// a textual error code otherwise, so it is decoded as raw JSON.
type xznoauthEnvelope struct {
	Code      json.RawMessage `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

func (e xznoauthEnvelope) codeText() string {
	return strings.Trim(string(e.Code), "\"")
}

func (e xznoauthEnvelope) ok() bool {
	code := strings.TrimSpace(e.codeText())
	return code == "" || code == "0" || code == "null"
}

// xznoauthClient performs client_credentials token acquisition (cached until just
// before expiry) and the authenticated invoice API calls. Secrets and tokens are
// never written into returned errors or logs.
type xznoauthClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpAt  time.Time
}

func newXZNoauthClient(settings invoiceSettings) *xznoauthClient {
	return &xznoauthClient{
		baseURL:      strings.TrimRight(settings.BaseURL, "/"),
		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,
		http:         newHTTPClient(xznoauthTimeout),
	}
}

func (c *xznoauthClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpAt.Add(-15*time.Second)) {
		return c.accessToken, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("scope", xznoauthTokenScope)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("xznoauth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("xznoauth token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", &xznoauthAPIError{status: resp.StatusCode, code: "oauth_token", message: strings.TrimSpace(string(body))}
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("xznoauth token: malformed token response")
	}
	c.accessToken = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	c.tokenExpAt = time.Now().Add(ttl)
	return c.accessToken, nil
}

func (c *xznoauthClient) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpAt = time.Time{}
}

// do performs one authenticated JSON call, retrying exactly once with a freshly
// fetched token when the first attempt failed client authentication.
func (c *xznoauthClient) do(ctx context.Context, method, path string, payload, out any) error {
	err := c.doOnce(ctx, method, path, payload, out)
	var apiErr *xznoauthAPIError
	if errors.As(err, &apiErr) && apiErr.retryable() {
		c.invalidateToken()
		if retryErr := c.doOnce(ctx, method, path, payload, out); retryErr == nil {
			return nil
		} else {
			return retryErr
		}
	}
	return err
}

func (c *xznoauthClient) doOnce(ctx context.Context, method, path string, payload, out any) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("xznoauth encode: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("xznoauth request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("xznoauth %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var env xznoauthEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("xznoauth %s %s: malformed envelope", method, path)
	}
	if !env.ok() {
		return &xznoauthAPIError{status: resp.StatusCode, code: env.codeText(), message: env.Message, requestID: env.RequestID}
	}
	if len(env.Data) > 0 && out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("xznoauth %s %s: decode data: %w", method, path, err)
		}
	}
	return nil
}

// downloadPDF streams a completed invoice PDF for an application owned by the
// current client Token. The endpoint returns binary application/pdf, not the
// JSON envelope.
func (c *xznoauthClient) downloadPDF(ctx context.Context, applicationID string) ([]byte, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/invoices/"+url.PathEscape(applicationID)+"/pdf", nil)
	if err != nil {
		return nil, fmt.Errorf("xznoauth pdf request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/pdf")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xznoauth pdf: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInvoicePDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("xznoauth pdf: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var env xznoauthEnvelope
		if json.Unmarshal(body, &env) == nil && !env.ok() {
			return nil, &xznoauthAPIError{status: resp.StatusCode, code: env.codeText(), message: env.Message, requestID: env.RequestID}
		}
		return nil, &xznoauthAPIError{status: resp.StatusCode, code: "pdf_download", message: "invoice pdf unavailable"}
	}
	return body, nil
}

// ---- XZNoAuth API model ----

type xznoauthOrder struct {
	PlatformOrderID any    `json:"platformOrderId"`
	OrderNo         string `json:"orderNo"`
	ExternalNo      string `json:"externalNo"`
	ProductName     string `json:"productName"`
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	PaidAt          string `json:"paidAt"`
	VerifiedAt      string `json:"verifiedAt"`
	TransactionID   string `json:"transactionId,omitempty"`
}

type xznoauthCheckout struct {
	TaxOrderNo string `json:"taxOrderNo"`
	PayURL     string `json:"payUrl"`
}

type xznoauthValidation struct {
	Orders        []xznoauthOrder             `json:"orders"`
	TotalAmount   string                      `json:"totalAmount"`
	Currency      string                      `json:"currency"`
	TaxAmount     string                      `json:"taxAmount"`
	TaxPaidAmount string                      `json:"taxPaidAmount"`
	TaxDueAmount  string                      `json:"taxDueAmount"`
	TaxPayments   map[string]xznoauthCheckout `json:"taxPayments"`
	TaxOrderNo    string                      `json:"taxOrderNo"`
	PayURL        string                      `json:"payUrl"`
}

// xznoauthApplication is the subset of an invoice application the router stores
// locally so user ownership survives the client-scoped list endpoint, which
// omits owner user IDs and emails.
type xznoauthApplication struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	OrderNos    []string `json:"orderNos,omitempty"`
	TaxOrderNos []string `json:"taxOrderNos,omitempty"`
	Amount      string   `json:"amount,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
}

// validate sends the exact paid order identifiers described in the contract and,
// when needPayTax is true, passes any previously paid tax order numbers for
// reconciliation before creating a new tax checkout.
func (c *xznoauthClient) validate(ctx context.Context, orderNos []string, needPayTax bool, taxOrderNos []string) (*xznoauthValidation, error) {
	payload := map[string]any{"orderNos": orderNos}
	if needPayTax {
		payload["needPayTax"] = true
	}
	if taxOrderNos := normalizeTaxOrderNos(taxOrderNos); len(taxOrderNos) > 0 {
		payload["taxOrderNos"] = taxOrderNos
	}
	var out xznoauthValidation
	if err := c.do(ctx, http.MethodPost, "/api/v1/invoice-orders/validate", payload, &out); err != nil {
		return nil, err
	}
	if out.Orders == nil {
		out.Orders = []xznoauthOrder{}
	}
	return &out, nil
}

func (c *xznoauthClient) taxPaymentStatus(ctx context.Context, taxOrderNo string) (bool, error) {
	var out struct {
		Paid bool `json:"paid"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/invoice-tax-payments/status", map[string]string{"taxOrderNo": taxOrderNo}, &out); err != nil {
		return false, err
	}
	return out.Paid, nil
}

func (c *xznoauthClient) submit(ctx context.Context, payload map[string]any) (*xznoauthApplication, error) {
	var out xznoauthApplication
	if err := c.do(ctx, http.MethodPost, "/api/v1/invoices", payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *xznoauthClient) list(ctx context.Context, page, pageSize int) ([]xznoauthApplication, error) {
	var out struct {
		Items    []xznoauthApplication `json:"items"`
		Total    int                   `json:"total"`
		Page     int                   `json:"page"`
		PageSize int                   `json:"pageSize"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/invoices?page=%d&pageSize=%d", page, pageSize), nil, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []xznoauthApplication{}
	}
	return out.Items, nil
}

func (c *xznoauthClient) cancel(ctx context.Context, applicationID string) (*xznoauthApplication, error) {
	var out xznoauthApplication
	if err := c.do(ctx, http.MethodPost, "/api/v1/invoices/"+url.PathEscape(applicationID)+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// invoiceErrorMessage maps XZNoAuth error codes to safe user-facing messages;
// raw provider responses are never exposed to end users.
func invoiceErrorMessage(err error) (int, string, string) {
	var apiErr *xznoauthAPIError
	if !errors.As(err, &apiErr) {
		return http.StatusBadGateway, "upstream_error", "invoice service temporarily unavailable"
	}
	status := http.StatusBadGateway
	message := "invoice service returned an error"
	switch apiErr.code {
	case "OAUTH_INVALID_CLIENT", "AUTH_INVALID_TOKEN", "AUTH_INSUFFICIENT_SCOPE", "AUTH_FORBIDDEN":
		status = http.StatusServiceUnavailable
		message = "invoice service configuration error; contact the administrator"
	case "INVOICE_ORDER_NOT_ELIGIBLE", "INVOICE_ORDER_ALREADY_CLAIMED":
		status = http.StatusConflict
		message = "one or more orders are not eligible or are already claimed"
	case "INVOICE_TAX_PAYMENT_NOT_ELIGIBLE":
		status = http.StatusConflict
		message = "tax payment check failed; restart the tax payment flow"
	case "INVOICE_TAX_PAYMENT_EXCEEDS_REQUIRED":
		status = http.StatusConflict
		message = "paid tax exceeds what the selected orders require; keep the tax orders and restore the original orders"
	case "INVOICE_TAX_ORDER_ALREADY_CLAIMED":
		status = http.StatusConflict
		message = "the tax order was already used by another application"
	case "INVOICE_TAX_ORDER_REQUIRED", "INVOICE_INVALID_TAX_ORDERS", "INVOICE_DUPLICATE_TAX_ORDER":
		status = http.StatusBadRequest
		message = "complete and deduplicate the tax orders before submitting"
	case "INVOICE_INVALID_ORDERS", "INVOICE_DUPLICATE_ORDER":
		status = http.StatusBadRequest
		message = "the order list is invalid"
	case "INVOICE_INVALID_FORM":
		status = http.StatusBadRequest
		message = "buyer data is incomplete or invalid"
	case "INVOICE_CANNOT_CANCEL":
		status = http.StatusConflict
		message = "the application cannot be cancelled in its current state"
	case "INVOICE_ORDER_RATE_LIMITED", "INVOICE_RATE_LIMITED", "INVOICE_TAX_STATUS_RATE_LIMITED":
		status = http.StatusTooManyRequests
		message = "too many requests; please try again later"
	case "NOT_FOUND":
		status = http.StatusNotFound
		message = "the application was not found or does not belong to you"
	case "PAYMENT_NOT_CONFIGURED", "PAYMENT_MERCHANT_NOT_CONFIGURED", "INVOICE_ADMIN_EMAIL_NOT_CONFIGURED":
		status = http.StatusServiceUnavailable
		message = "the invoice service is missing required configuration"
	case "PAYMENT_UNAVAILABLE", "PAYMENT_INVALID_RESPONSE":
		status = http.StatusBadGateway
		message = "the invoice payment step failed; try again later"
	}
	return status, apiErr.code, message
}

func writeInvoiceError(w http.ResponseWriter, err error) {
	status, code, msg := invoiceErrorMessage(err)
	writeError(w, status, code, msg)
	var apiErr *xznoauthAPIError
	if errors.As(err, &apiErr) && apiErr.requestID != "" {
		log.Printf("invoice upstream %s: requestId=%s", apiErr.code, apiErr.requestID)
	}
}

// ---- Admin: invoice settings ----

func (s *Service) adminGetInvoiceSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadInvoiceSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load invoice settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Service) adminUpdateInvoiceSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled      bool   `json:"enabled"`
		BaseURL      string `json:"base_url"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		NeedPayTax   bool   `json:"need_pay_tax"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid invoice settings")
		return
	}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.ClientSecret = strings.TrimSpace(in.ClientSecret)
	if in.BaseURL != "" && validUpstreamURL(in.BaseURL) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "base_url must be an HTTP or HTTPS URL")
		return
	}
	if in.ClientID != "" && len(in.ClientID) > maxInvoiceIdentifier {
		writeError(w, http.StatusBadRequest, "invalid_request", "client_id is too long")
		return
	}
	current, err := s.loadInvoiceSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load invoice settings")
		return
	}
	secret := in.ClientSecret
	if secret == "" {
		secret = current.ClientSecret
	}
	if in.Enabled && secret == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "client_secret is required when invoice is enabled")
		return
	}
	encrypted := ""
	if secret != "" {
		encrypted, err = crypt(s.cfg.EncryptionKey, secret, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not secure client secret")
			return
		}
	}
	_, err = s.db.Exec(r.Context(), `update invoice_settings set enabled=$1,base_url=$2,client_id=$3,client_secret_encrypted=$4,need_pay_tax=$5,updated_at=now() where id=1`, in.Enabled, in.BaseURL, in.ClientID, encrypted, in.NeedPayTax)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save invoice settings")
		return
	}
	s.audit(r, "invoice.settings_updated", "invoice_settings", "1", map[string]any{"enabled": in.Enabled, "need_pay_tax": in.NeedPayTax})
	settings, err := s.loadInvoiceSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reload invoice settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// ---- Account: invoice application flow ----

func (s *Service) accountInvoiceSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadInvoiceSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load invoice settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"enabled":      settings.ready(),
		"need_pay_tax": settings.NeedPayTax,
	})
}

type invoiceEligibleOrder struct {
	OrderNo   string `json:"order_no"`
	InvoiceNo string `json:"invoice_no"`
	OrderType string `json:"order_type"`
	PlanName  string `json:"plan_name"`
	Amount    string `json:"amount"`
	PaidAt    any    `json:"paid_at"`
}

func (s *Service) accountInvoiceEligibleOrders(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select o.order_no,o.inv_no,o.amount,o.paid_at,o.order_type,o.plan_name from (
    select po.order_no, coalesce(po.provider_trade_no, po.order_no) as inv_no, po.amount::text as amount, po.paid_at, 'payment' as order_type, '' as plan_name
    from payment_orders po where po.user_id=$1 and po.status='paid' and po.paid_at is not null
    union all
    select so.order_no, coalesce(so.provider_trade_no, so.order_no), so.amount::text, so.paid_at, 'subscription', p.name
    from subscription_orders so join subscription_plans p on p.id=so.plan_id where so.user_id=$1 and so.status='paid' and so.paid_at is not null
  ) o
  where not exists (
    select 1 from invoice_application_orders io
    join invoice_applications ia on ia.id=io.application_id
    where io.local_order_no=o.order_no and ia.status<>'canceled'
  )
  order by o.paid_at desc`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load invoice orders")
		return
	}
	defer rows.Close()
	orders := []invoiceEligibleOrder{}
	for rows.Next() {
		var order invoiceEligibleOrder
		if rows.Scan(&order.OrderNo, &order.InvoiceNo, &order.Amount, &order.PaidAt, &order.OrderType, &order.PlanName) == nil {
			orders = append(orders, order)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": orders})
}

func (s *Service) accountInvoiceValidate(w http.ResponseWriter, r *http.Request) {
	client, ok := s.invoiceClient(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "invoice_unavailable", "invoice service is not configured")
		return
	}
	var in struct {
		OrderNos    []string `json:"orderNos"`
		NeedPayTax  bool     `json:"needPayTax"`
		TaxOrderNos []string `json:"taxOrderNos"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid validation payload")
		return
	}
	orderNos, msg := normalizeOrderNos(in.OrderNos)
	if msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	out, err := client.validate(r.Context(), orderNos, in.NeedPayTax, in.TaxOrderNos)
	if err != nil {
		writeInvoiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Service) accountInvoiceTaxStatus(w http.ResponseWriter, r *http.Request) {
	client, ok := s.invoiceClient(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "invoice_unavailable", "invoice service is not configured")
		return
	}
	var in struct {
		TaxOrderNo string `json:"taxOrderNo"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.TaxOrderNo) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "taxOrderNo is required")
		return
	}
	paid, err := client.taxPaymentStatus(r.Context(), strings.TrimSpace(in.TaxOrderNo))
	if err != nil {
		writeInvoiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"paid": paid})
}

type invoiceBuyerInput struct {
	OrderNos       []string `json:"orderNos"`
	NeedPayTax     bool     `json:"needPayTax"`
	TaxOrderNos    []string `json:"taxOrderNos"`
	BuyerType      string   `json:"buyerType"`
	Title          string   `json:"title"`
	TaxpayerID     string   `json:"taxpayerId"`
	BuyerAddress   string   `json:"buyerAddress"`
	BuyerPhone     string   `json:"buyerPhone"`
	BuyerBank      string   `json:"buyerBank"`
	BuyerBankAcc   string   `json:"buyerBankAccount"`
	RecipientEmail string   `json:"recipientEmail"`
}

func (s *Service) accountInvoiceSubmit(w http.ResponseWriter, r *http.Request) {
	client, ok := s.invoiceClient(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "invoice_unavailable", "invoice service is not configured")
		return
	}
	account := accountFromContext(r)
	var in invoiceBuyerInput
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid invoice application")
		return
	}
	orderNos, msg := normalizeOrderNos(in.OrderNos)
	if msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	if !validateBuyerInput(in, &msg) {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	localOrders, amount, err := s.resolveInvoiceableOrders(r.Context(), account.userID, orderNos)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not verify source orders")
		return
	}
	if len(localOrders) == 0 {
		writeError(w, http.StatusConflict, "invoice_order_not_eligible", "所选订单不可用于开票")
		return
	}
	payload := map[string]any{
		"orderNos":         orderNos,
		"buyerType":        strings.ToLower(strings.TrimSpace(in.BuyerType)),
		"title":            strings.TrimSpace(in.Title),
		"taxpayerId":       strings.TrimSpace(in.TaxpayerID),
		"buyerAddress":     strings.TrimSpace(in.BuyerAddress),
		"buyerPhone":       strings.TrimSpace(in.BuyerPhone),
		"buyerBank":        strings.TrimSpace(in.BuyerBank),
		"buyerBankAccount": normalizeBankAccount(in.BuyerBankAcc),
		"recipientEmail":   strings.TrimSpace(in.RecipientEmail),
	}
	if in.NeedPayTax {
		taxNos := normalizeTaxOrderNos(in.TaxOrderNos)
		if len(taxNos) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "complete the tax payment before submitting")
			return
		}
		payload["needPayTax"] = true
		payload["taxOrderNos"] = taxNos
	}
	xapp, err := client.submit(r.Context(), payload)
	if err != nil {
		writeInvoiceError(w, err)
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create invoice application")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create invoice application")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `insert into invoice_applications(id,user_id,application_id,status,buyer_type,title,taxpayer_id,buyer_address,buyer_phone,buyer_bank,buyer_bank_account,recipient_email,total_amount,currency,need_pay_tax) values($1,$2,$3,'pending',$4,$5,$6,$7,$8,$9,$10,$11,$12,'CNY',$13)`,
		id, account.userID, xapp.ID, strings.ToLower(strings.TrimSpace(in.BuyerType)), strings.TrimSpace(in.Title), strings.TrimSpace(in.TaxpayerID), strings.TrimSpace(in.BuyerAddress), strings.TrimSpace(in.BuyerPhone), strings.TrimSpace(in.BuyerBank), normalizeBankAccount(in.BuyerBankAcc), strings.TrimSpace(in.RecipientEmail), amount, in.NeedPayTax)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create invoice application")
		return
	}
	for _, local := range localOrders {
		if _, err = tx.Exec(r.Context(), `insert into invoice_application_orders(application_id,order_no,order_type,local_order_no,amount) values($1,$2,$3,$4,$5)`, id, local.InvoiceNo, local.OrderNo, local.OrderNo, normalizeAmountCents(local.Amount)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not create invoice application")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create invoice application")
		return
	}
	s.audit(r, "invoice.application_created", "invoice_application", id, map[string]any{"application_id": xapp.ID, "orders": len(orderNos), "need_pay_tax": in.NeedPayTax})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "application_id": xapp.ID, "status": "pending"})
}

type localInvoiceApplication struct {
	ID             string `json:"id"`
	ApplicationID  string `json:"application_id"`
	Status         string `json:"status"`
	BuyerType      string `json:"buyer_type"`
	Title          string `json:"title"`
	RecipientEmail string `json:"recipient_email"`
	TotalAmount    string `json:"total_amount"`
	Currency       string `json:"currency"`
	NeedPayTax     bool   `json:"need_pay_tax"`
	CreatedAt      any    `json:"created_at"`
}

func (s *Service) accountInvoices(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	if r.URL.Query().Has("sync") {
		s.syncInvoiceStatuses(r.Context())
	}
	rows, err := s.db.Query(r.Context(), `select id,application_id,status,buyer_type,title,recipient_email,total_amount::text,currency,need_pay_tax,created_at from invoice_applications where user_id=$1 order by created_at desc limit 100`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load invoices")
		return
	}
	defer rows.Close()
	apps := []localInvoiceApplication{}
	for rows.Next() {
		var app localInvoiceApplication
		if rows.Scan(&app.ID, &app.ApplicationID, &app.Status, &app.BuyerType, &app.Title, &app.RecipientEmail, &app.TotalAmount, &app.Currency, &app.NeedPayTax, &app.CreatedAt) == nil {
			apps = append(apps, app)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": apps})
}

// syncInvoiceStatuses refreshes locally tracked statuses from the client-scoped
// list endpoint, which omits owner user IDs. Only application IDs that already
// point at a local row are reconciled, so user ownership is preserved.
func (s *Service) syncInvoiceStatuses(ctx context.Context) {
	settings, err := s.loadInvoiceSettings(ctx)
	if err != nil || !settings.ready() {
		return
	}
	client := newXZNoauthClient(settings)
	for page := 1; page <= 5; page++ {
		items, err := client.list(ctx, page, 100)
		if err != nil {
			return
		}
		for _, item := range items {
			if item.ID == "" {
				continue
			}
			_, _ = s.db.Exec(ctx, `update invoice_applications set status=$1,updated_at=now() where application_id=$2 and status<>$1`, item.Status, item.ID)
		}
		if len(items) < 100 {
			break
		}
	}
}

func (s *Service) accountInvoicePDF(w http.ResponseWriter, r *http.Request) {
	client, ok := s.invoiceClient(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "invoice_unavailable", "invoice service is not configured")
		return
	}
	account := accountFromContext(r)
	var appID, status string
	err := s.db.QueryRow(r.Context(), `select application_id,status from invoice_applications where id=$1 and user_id=$2`, r.PathValue("id"), account.userID).Scan(&appID, &status)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "invoice application not found")
		return
	}
	if status != "completed" {
		writeError(w, http.StatusConflict, "invoice_not_ready", "invoice is not completed")
		return
	}
	pdf, err := client.downloadPDF(r.Context(), appID)
	if err != nil {
		writeInvoiceError(w, err)
		return
	}
	if len(pdf) > maxInvoicePDFSize {
		writeError(w, http.StatusBadGateway, "upstream_error", "invoice pdf is too large")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="invoice-`+appID+`.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func (s *Service) accountInvoiceCancel(w http.ResponseWriter, r *http.Request) {
	client, ok := s.invoiceClient(r)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "invoice_unavailable", "invoice service is not configured")
		return
	}
	account := accountFromContext(r)
	var localID, appID, status string
	err := s.db.QueryRow(r.Context(), `select id,application_id,status from invoice_applications where id=$1 and user_id=$2`, r.PathValue("id"), account.userID).Scan(&localID, &appID, &status)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "invoice application not found")
		return
	}
	if status != "pending" && status != "approved" {
		writeError(w, http.StatusConflict, "invoice_cannot_cancel", "the application cannot be cancelled in its current state")
		return
	}
	xapp, err := client.cancel(r.Context(), appID)
	if err != nil {
		writeInvoiceError(w, err)
		return
	}
	_, _ = s.db.Exec(r.Context(), `update invoice_applications set status='canceled',updated_at=now() where id=$1`, localID)
	s.audit(r, "invoice.application_canceled", "invoice_application", localID, map[string]any{"application_id": appID, "status": xapp.Status})
	writeJSON(w, http.StatusOK, map[string]string{"id": localID, "application_id": appID, "status": "canceled"})
}

// ---- helpers ----

func (s *Service) invoiceClient(r *http.Request) (*xznoauthClient, bool) {
	settings, err := s.loadInvoiceSettings(r.Context())
	if err != nil || !settings.ready() {
		return nil, false
	}
	return newXZNoauthClient(settings), true
}

func normalizeOrderNos(orderNos []string) ([]string, string) {
	if len(orderNos) == 0 || len(orderNos) > xznoauthMaxOrders {
		return nil, "orderNos must contain 1-20 orders"
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range orderNos {
		value := strings.TrimSpace(raw)
		if value == "" || len(value) > maxInvoiceIdentifier {
			return nil, "orderNos contains an invalid identifier"
		}
		if seen[value] {
			return nil, "orderNos must be unique"
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, ""
}

func normalizeTaxOrderNos(taxOrderNos []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range taxOrderNos {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func normalizeBankAccount(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), " ", ""), "-", "")
}

func validateBuyerInput(in invoiceBuyerInput, msg *string) bool {
	*msg = ""
	buyerType := strings.ToLower(strings.TrimSpace(in.BuyerType))
	if buyerType != "individual" && buyerType != "company" {
		*msg = "buyerType must be individual or company"
		return false
	}
	title := strings.TrimSpace(in.Title)
	if title == "" || len(title) > maxInvoiceTitleLen {
		*msg = "title is required and at most 255 characters"
		return false
	}
	taxpayerID := strings.TrimSpace(in.TaxpayerID)
	if taxpayerID == "" || len(taxpayerID) > maxInvoiceTaxIDLen {
		*msg = "taxpayerId is required and at most 32 characters"
		return false
	}
	if len(strings.TrimSpace(in.BuyerAddress)) > maxInvoiceBuyerFieldLen {
		*msg = "buyerAddress is too long"
		return false
	}
	if len(strings.TrimSpace(in.BuyerPhone)) > maxInvoicePhoneLen {
		*msg = "buyerPhone is too long"
		return false
	}
	if len(strings.TrimSpace(in.BuyerBank)) > maxInvoiceBuyerFieldLen {
		*msg = "buyerBank is too long"
		return false
	}
	account := normalizeBankAccount(in.BuyerBankAcc)
	if account != "" {
		if len(account) < minInvoiceBankAccountLen || len(account) > maxInvoiceBankAccountLen {
			*msg = "buyerBankAccount must be 8-32 digits"
			return false
		}
		for _, ch := range account {
			if ch < '0' || ch > '9' {
				*msg = "buyerBankAccount must be 8-32 digits"
				return false
			}
		}
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(in.RecipientEmail)); err != nil {
		*msg = "recipientEmail is invalid"
		return false
	}
	return true
}

// resolveInvoiceableOrders maps the submitted identifiers back to the user's own
// paid orders (either the platform order no or the provider transaction id) and
// sums their amounts. Any identifier that does not belong to the user is skipped.
func (s *Service) resolveInvoiceableOrders(ctx context.Context, userID string, orderNos []string) ([]invoiceEligibleOrder, string, error) {
	rows, err := s.db.Query(ctx, `select order_no,inv_no,amount,order_type,plan_name from (
		select po.order_no, coalesce(po.provider_trade_no, po.order_no) as inv_no, po.amount::text as amount, 'payment' as order_type, '' as plan_name from payment_orders po
		where po.user_id=$1 and po.status='paid' and po.paid_at is not null
		union all
		select so.order_no, coalesce(so.provider_trade_no, so.order_no), so.amount::text, 'subscription', p.name from subscription_orders so
		join subscription_plans p on p.id=so.plan_id where so.user_id=$1 and so.status='paid' and so.paid_at is not null
	) o where o.inv_no = any($2)`, userID, orderNos)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	byIdentifier := map[string]invoiceEligibleOrder{}
	var totalCents int64
	for rows.Next() {
		var order invoiceEligibleOrder
		if rows.Scan(&order.OrderNo, &order.InvoiceNo, &order.Amount, &order.OrderType, &order.PlanName) == nil {
			byIdentifier[order.InvoiceNo] = order
		}
		if cents, _, ok := parsePaymentAmount(order.Amount); ok {
			totalCents += cents
		}
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	out := make([]invoiceEligibleOrder, 0, len(orderNos))
	for _, no := range orderNos {
		if order, exists := byIdentifier[no]; exists {
			out = append(out, order)
		}
	}
	return out, fmt.Sprintf("%d.%02d", totalCents/100, totalCents%100), nil
}

func normalizeAmountCents(amount string) any {
	if cents, _, ok := parsePaymentAmount(amount); ok {
		return fmt.Sprintf("%d.%02d", cents/100, cents%100)
	}
	return "0.00"
}

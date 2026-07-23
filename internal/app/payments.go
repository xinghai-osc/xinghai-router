package app

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	minPaymentCents   int64 = 100
	maxPaymentCents   int64 = 10_000_000
	maxPaymentURLLen        = 2048
	maxMerchantIDLen        = 128
	maxMerchantKeyLen       = 4096
)

type paymentOrder struct {
	OrderNo         string `json:"order_no"`
	PaymentType     string `json:"payment_type"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	ProviderTradeNo string `json:"provider_trade_no,omitempty"`
	PaidAt          any    `json:"paid_at"`
	CreatedAt       any    `json:"created_at"`
}

type epaySettings struct {
	Enabled        bool            `json:"enabled"`
	BaseURL        string          `json:"base_url"`
	MerchantID     string          `json:"merchant_id"`
	MerchantKey    string          `json:"-"`
	HasMerchantKey bool            `json:"has_merchant_key"`
	PublicBaseURL  string          `json:"public_base_url"`
	Methods        []paymentMethod `json:"methods"`
}

type paymentMethod struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CreatedAt any    `json:"created_at"`
}

func (s *Service) loadEpaySettings(r *http.Request) (epaySettings, error) {
	var settings epaySettings
	var encryptedKey string
	err := s.db.QueryRow(r.Context(), `select enabled,base_url,merchant_id,merchant_key_encrypted,public_base_url from payment_settings where provider='epay'`).Scan(&settings.Enabled, &settings.BaseURL, &settings.MerchantID, &encryptedKey, &settings.PublicBaseURL)
	if err != nil {
		return settings, err
	}
	settings.HasMerchantKey = encryptedKey != ""
	if encryptedKey != "" {
		settings.MerchantKey, err = crypt(s.cfg.EncryptionKey, encryptedKey, true)
		if err != nil {
			return settings, err
		}
	}
	rows, err := s.db.Query(r.Context(), `select id,code,name,enabled,created_at from payment_methods where provider='epay' order by created_at,id`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()
	settings.Methods = []paymentMethod{}
	for rows.Next() {
		var method paymentMethod
		if err = rows.Scan(&method.ID, &method.Code, &method.Name, &method.Enabled, &method.CreatedAt); err != nil {
			return settings, err
		}
		settings.Methods = append(settings.Methods, method)
	}
	return settings, rows.Err()
}

func (settings epaySettings) ready() bool {
	return settings.Enabled && settings.BaseURL != "" && settings.MerchantID != "" && settings.MerchantKey != "" && settings.PublicBaseURL != ""
}

func (s *Service) getPaymentSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadEpaySettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load payment settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Service) updatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled       bool   `json:"enabled"`
		BaseURL       string `json:"base_url"`
		MerchantID    string `json:"merchant_id"`
		MerchantKey   string `json:"merchant_key"`
		PublicBaseURL string `json:"public_base_url"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payment settings")
		return
	}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.PublicBaseURL = strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	in.MerchantID = strings.TrimSpace(in.MerchantID)
	in.MerchantKey = strings.TrimSpace(in.MerchantKey)
	if len(in.BaseURL) > maxPaymentURLLen || len(in.PublicBaseURL) > maxPaymentURLLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "payment URLs must be at most 2048 characters")
		return
	}
	if len(in.MerchantID) > maxMerchantIDLen || len(in.MerchantKey) > maxMerchantKeyLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "merchant_id must be at most 128 characters and merchant_key at most 4096")
		return
	}
	if in.Enabled {
		if validPublicURL(in.BaseURL) != nil || validPublicURL(in.PublicBaseURL) != nil || in.MerchantID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "enabled payment requires valid URLs and merchant credentials")
			return
		}
	}
	current, err := s.loadEpaySettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load payment settings")
		return
	}
	merchantKey := in.MerchantKey
	if merchantKey == "" {
		merchantKey = current.MerchantKey
	}
	if in.Enabled && merchantKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "enabled payment requires valid URLs and merchant credentials")
		return
	}
	encryptedKey := ""
	if merchantKey != "" {
		encryptedKey, err = crypt(s.cfg.EncryptionKey, merchantKey, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not secure merchant key")
			return
		}
	}
	_, err = s.db.Exec(r.Context(), `update payment_settings set enabled=$1,base_url=$2,merchant_id=$3,merchant_key_encrypted=$4,public_base_url=$5,updated_at=now() where provider='epay'`, in.Enabled, in.BaseURL, in.MerchantID, encryptedKey, in.PublicBaseURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save payment settings")
		return
	}
	s.audit(r, "payment.settings_updated", "payment_provider", "epay", map[string]any{"enabled": in.Enabled})
	settings, err := s.loadEpaySettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reload payment settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Service) createPaymentMethod(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if decode(r, &in) != nil || !validPaymentMethod(in.Code, in.Name) {
		writeError(w, http.StatusBadRequest, "invalid_request", "code and name are required")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create payment method")
		return
	}
	in.Code = strings.ToLower(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	_, err = s.db.Exec(r.Context(), `insert into payment_methods(id,provider,code,name,enabled) values($1,'epay',$2,$3,$4)`, id, in.Code, in.Name, in.Enabled)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "payment method code already exists")
		return
	}
	s.audit(r, "payment.method_created", "payment_method", id, map[string]any{"code": in.Code})
	writeJSON(w, http.StatusCreated, paymentMethod{ID: id, Code: in.Code, Name: in.Name, Enabled: in.Enabled})
}

func (s *Service) updatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if decode(r, &in) != nil || !validPaymentMethod(in.Code, in.Name) {
		writeError(w, http.StatusBadRequest, "invalid_request", "code and name are required")
		return
	}
	in.Code = strings.ToLower(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	result, err := s.db.Exec(r.Context(), `update payment_methods set code=$1,name=$2,enabled=$3,updated_at=now() where id=$4 and provider='epay'`, in.Code, in.Name, in.Enabled, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "payment method code already exists")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "payment method not found")
		return
	}
	s.audit(r, "payment.method_updated", "payment_method", r.PathValue("id"), map[string]any{"code": in.Code, "enabled": in.Enabled})
	writeJSON(w, http.StatusOK, paymentMethod{ID: r.PathValue("id"), Code: in.Code, Name: in.Name, Enabled: in.Enabled})
}

func (s *Service) deletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `delete from payment_methods where id=$1 and provider='epay'`, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete payment method")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "payment method not found")
		return
	}
	s.audit(r, "payment.method_deleted", "payment_method", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) createAccountPayment(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadEpaySettings(r)
	if err != nil || !settings.ready() {
		writeError(w, http.StatusServiceUnavailable, "payment_unavailable", "online payment is not configured")
		return
	}
	var in struct {
		Amount string `json:"amount"`
		Type   string `json:"type"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "amount and payment type are required")
		return
	}
	cents, amount, ok := parsePaymentAmount(in.Amount)
	paymentType := strings.ToLower(strings.TrimSpace(in.Type))
	if !ok || !paymentAmountInBounds(cents) || !methodEnabled(settings.Methods, paymentType) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid amount or payment type")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create payment order")
		return
	}
	randomPart := strings.ReplaceAll(id, "-", "")[:12]
	orderNo := fmt.Sprintf("xh%d%s", time.Now().UnixMilli(), randomPart)
	account := accountFromContext(r)
	_, err = s.db.Exec(r.Context(), `insert into payment_orders(id,order_no,user_id,provider,payment_type,amount) values($1,$2,$3,'epay',$4,$5)`, id, orderNo, account.userID, paymentType, amount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create payment order")
		return
	}
	params := url.Values{
		"pid":          {settings.MerchantID},
		"type":         {paymentType},
		"out_trade_no": {orderNo},
		"notify_url":   {settings.PublicBaseURL + "/api/payments/epay/notify"},
		"return_url":   {settings.PublicBaseURL + "/console/wallet?payment_order=" + url.QueryEscape(orderNo)},
		"name":         {"Xinghai Router balance top-up"},
		"money":        {amount},
	}
	params.Set("sign", epaySign(params, settings.MerchantKey))
	params.Set("sign_type", "MD5")
	writeJSON(w, http.StatusCreated, map[string]any{"order_no": orderNo, "amount": amount, "status": "pending", "pay_url": settings.BaseURL + "/submit.php?" + params.Encode()})
}

func (s *Service) listAccountPayments(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select order_no,payment_type,amount::text,status,coalesce(provider_trade_no,''),paid_at,created_at from payment_orders where user_id=$1 order by created_at desc limit 50`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load payment orders")
		return
	}
	defer rows.Close()
	orders := []paymentOrder{}
	for rows.Next() {
		var order paymentOrder
		if rows.Scan(&order.OrderNo, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTradeNo, &order.PaidAt, &order.CreatedAt) == nil {
			orders = append(orders, order)
		}
	}
	settings, settingsErr := s.loadEpaySettings(r)
	methods := []paymentMethod{}
	if settingsErr == nil && settings.ready() {
		for _, method := range settings.Methods {
			if method.Enabled {
				methods = append(methods, method)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": len(methods) > 0, "payment_methods": methods, "data": orders})
}

func (s *Service) getAccountPayment(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var order paymentOrder
	err := s.db.QueryRow(r.Context(), `select order_no,payment_type,amount::text,status,coalesce(provider_trade_no,''),paid_at,created_at from payment_orders where order_no=$1 and user_id=$2`, r.PathValue("order_no"), account.userID).Scan(&order.OrderNo, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTradeNo, &order.PaidAt, &order.CreatedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "payment order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load payment order")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Service) epayNotify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	settings, err := s.loadEpaySettings(r)
	if err != nil || settings.MerchantKey == "" || r.ParseForm() != nil || r.Form.Get("pid") != settings.MerchantID || !equalSecret(strings.ToLower(r.Form.Get("sign")), epaySign(r.Form, settings.MerchantKey)) {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	if r.Form.Get("trade_status") != "TRADE_SUCCESS" {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	_, notifiedAmount, ok := parsePaymentAmount(r.Form.Get("money"))
	if !ok || strings.TrimSpace(r.Form.Get("trade_no")) == "" {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		http.Error(w, "fail", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())
	var id, userID, amount, status string
	err = tx.QueryRow(r.Context(), `select id,user_id,amount::text,status from payment_orders where order_no=$1 and provider='epay' for update`, r.Form.Get("out_trade_no")).Scan(&id, &userID, &amount, &status)
	if err == nil {
		if amount != notifiedAmount {
			http.Error(w, "fail", http.StatusBadRequest)
			return
		}
		if status == "paid" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return
		}
		if status != "pending" {
			http.Error(w, "fail", http.StatusConflict)
			return
		}
		if _, err = tx.Exec(r.Context(), `update payment_orders set status='paid',provider_trade_no=$1,paid_at=now(),updated_at=now() where id=$2`, r.Form.Get("trade_no"), id); err != nil {
			http.Error(w, "fail", http.StatusConflict)
			return
		}
		if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict do nothing`, userID); err != nil {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		var balance string
		if err = tx.QueryRow(r.Context(), `update user_wallets set balance=balance+$1::numeric,updated_at=now() where user_id=$2 returning balance::text`, amount, userID).Scan(&balance); err != nil {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		ledgerID, err := randomID()
		if err != nil {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,'topup',$5,$6)`, ledgerID, userID, amount, balance, r.Form.Get("out_trade_no"), "Epay top-up"); err != nil {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
		return
	}
	// Otherwise treat as a subscription order activation.
	activated, err := s.activateSubscriptionOrderTx(r.Context(), tx, r.Form.Get("out_trade_no"), r.Form.Get("trade_no"), notifiedAmount)
	if err != nil {
		http.Error(w, "fail", http.StatusConflict)
		return
	}
	if !activated {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "fail", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func validPaymentMethod(code, name string) bool {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || len(code) > 50 || name == "" || len(name) > 100 {
		return false
	}
	for _, ch := range code {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

func methodEnabled(methods []paymentMethod, target string) bool {
	for _, method := range methods {
		if method.Enabled && method.Code == target {
			return true
		}
	}
	return false
}

func validPublicURL(value string) error {
	if err := validOutboundURL(value); err != nil {
		return fmt.Errorf("must be an HTTPS URL (HTTP is allowed for localhost)")
	}
	return nil
}

func paymentAmountInBounds(cents int64) bool {
	return cents >= minPaymentCents && cents <= maxPaymentCents
}

func parsePaymentAmount(value string) (int64, string, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 6 {
		return 0, "", false
	}
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return 0, "", false
		}
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 0 || len(fraction) > 2 {
			return 0, "", false
		}
		for _, ch := range fraction {
			if ch < '0' || ch > '9' {
				return 0, "", false
			}
		}
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	var cents int64
	for _, ch := range parts[0] + fraction {
		cents = cents*10 + int64(ch-'0')
	}
	return cents, fmt.Sprintf("%d.%02d", cents/100, cents%100), true
}

func epaySign(values url.Values, key string) string {
	keys := make([]string, 0, len(values))
	for name := range values {
		if name != "sign" && name != "sign_type" && values.Get(name) != "" {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, name := range keys {
		parts = append(parts, name+"="+values.Get(name))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

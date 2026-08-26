package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// captchaPurposeXxx are the Corptcha verification purposes. They are fixed
	// server-side per endpoint, never taken from the client, so a token issued
	// for one purpose cannot be replayed on another.
	captchaPurposeLogin     = "login"
	captchaPurposeRegister  = "register"
	captchaPurposeEmailCode = "email_code"
	captchaPurposeReset     = "reset"
	captchaPurposeCheckin   = "checkin"

	corptchaAPIBaseURL = "https://cpt-api.25y.cn"
	corptchaVerifyPath = "/v1/verify"
)

// corptchaPayload is the token produced by the Corptcha widget after the user
// completes the challenge. The purpose echoes what the widget was told, but
// the backend verifies against its own per-endpoint purpose so a mismatched
// token is rejected by the verification API.
type corptchaPayload struct {
	Token   string `json:"captcha_token"`
	Purpose string `json:"captcha_purpose"`
}

func (p corptchaPayload) complete() bool {
	return p.Token != ""
}

// verifyCaptcha dispatches to the active provider: Corptcha when selected,
// Geetest otherwise. Providers that are not configured pass through.
func (s *Service) verifyCaptcha(ctx context.Context, geetest geetestPayload, corptcha corptchaPayload, purpose string) error {
	sys := s.loadSystemConfig(ctx)
	switch sys.captchaProvider() {
	case "corptcha":
		return s.verifyCorptcha(ctx, sys, corptcha, purpose)
	default:
		return s.verifyGeetest(ctx, sys, geetest)
	}
}

// verifyCorptcha validates a one-time Corptcha token with the Corptcha
// verification API. See the integration guide: the request is authorised with
// the site secret and the response must be HTTP 200 with success=true.
func (s *Service) verifyCorptcha(ctx context.Context, sys systemConfig, payload corptchaPayload, purpose string) error {
	if !payload.complete() {
		return fmt.Errorf("captcha validation is required")
	}
	body, err := json.Marshal(map[string]string{"token": payload.Token, "purpose": purpose, "siteKey": sys.CorptchaSiteID})
	if err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, corptchaAPIBaseURL+corptchaVerifyPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sys.CorptchaSecret)
	resp, err := newHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	if resp.StatusCode != http.StatusOK || !out.Success {
		return fmt.Errorf("captcha validation failed")
	}
	return nil
}

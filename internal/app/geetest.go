package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// geetestPayload is the client-side validation result produced by the
// Geetest v4 widget after the user completes the challenge.
type geetestPayload struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

func (p geetestPayload) complete() bool {
	return p.LotNumber != "" && p.CaptchaOutput != "" && p.PassToken != "" && p.GenTime != ""
}

// verifyGeetest validates a Geetest v4 challenge result with the Geetest
// server. See https://docs.geetest.com/gt4/apirefer/api/server.
func (s *Service) verifyGeetest(ctx context.Context, payload geetestPayload) error {
	sys := s.loadSystemConfig(ctx)
	if !sys.geetestEnabled() {
		return nil
	}
	if !payload.complete() {
		return fmt.Errorf("captcha validation is required")
	}
	mac := hmac.New(sha256.New, []byte(sys.GeetestCaptchaKey))
	mac.Write([]byte(payload.LotNumber))
	form := url.Values{
		"lot_number":     {payload.LotNumber},
		"captcha_output": {payload.CaptchaOutput},
		"pass_token":     {payload.PassToken},
		"gen_time":       {payload.GenTime},
		"sign_token":     {hex.EncodeToString(mac.Sum(nil))},
	}
	endpoint := fmt.Sprintf("https://gcaptcha4.geetest.com/validate?captcha_id=%s", url.QueryEscape(sys.GeetestCaptchaID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := newHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	defer resp.Body.Close()
	var out struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("captcha verification unavailable")
	}
	if out.Result != "success" {
		return fmt.Errorf("captcha validation failed")
	}
	return nil
}

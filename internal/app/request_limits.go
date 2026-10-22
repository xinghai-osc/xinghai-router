package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultWSMaxMessageBytes     = 2 << 20
	defaultRequestBodyTimeout    = 30 * time.Second
	defaultConsoleBodyMaxBytes   = 1 << 20
	defaultGatewayBodyMaxBytes   = 20 << 20
	defaultImageBodyMaxBytes     = 50 << 20
	defaultWSIdleTimeout         = 2 * time.Minute
	defaultHTTPReadHeaderTimeout = 10 * time.Second
	defaultHTTPIdleTimeout       = 2 * time.Minute
	defaultHTTPMaxHeaderBytes    = 1 << 20
)

func loadRequestLimits(c *Config) error {
	for _, setting := range []struct {
		name     string
		target   *int64
		fallback int64
	}{
		{"WS_MAX_MESSAGE_BYTES", &c.WSMaxMessageBytes, defaultWSMaxMessageBytes},
		{"GATEWAY_MAX_BODY_BYTES", &c.GatewayMaxBodyBytes, defaultGatewayBodyMaxBytes},
		{"IMAGE_MAX_BODY_BYTES", &c.ImageMaxBodyBytes, defaultImageBodyMaxBytes},
	} {
		*setting.target = setting.fallback
		if raw, exists := os.LookupEnv(setting.name); exists {
			n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil || n <= 0 || n >= int64(^uint(0)>>1) {
				return fmt.Errorf("%s must be a positive byte count smaller than %d", setting.name, int64(^uint(0)>>1))
			}
			*setting.target = n
		}
	}
	c.HTTPMaxHeaderBytes = defaultHTTPMaxHeaderBytes
	if raw, exists := os.LookupEnv("HTTP_MAX_HEADER_BYTES"); exists {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n <= 0 || n > int(^uint(0)>>1)-4096 {
			return fmt.Errorf("HTTP_MAX_HEADER_BYTES must be a positive byte count smaller than %d", int(^uint(0)>>1)-4096)
		}
		c.HTTPMaxHeaderBytes = n
	}
	for _, setting := range []struct {
		name     string
		target   *time.Duration
		fallback time.Duration
	}{
		{"REQUEST_BODY_TIMEOUT", &c.RequestBodyTimeout, defaultRequestBodyTimeout},
		{"WS_IDLE_TIMEOUT", &c.WSIdleTimeout, defaultWSIdleTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", &c.HTTPReadHeaderTimeout, defaultHTTPReadHeaderTimeout},
		{"HTTP_IDLE_TIMEOUT", &c.HTTPIdleTimeout, defaultHTTPIdleTimeout},
	} {
		*setting.target = setting.fallback
		if raw, exists := os.LookupEnv(setting.name); exists {
			duration, err := time.ParseDuration(strings.TrimSpace(raw))
			if err != nil || duration < 0 || (duration == 0 && setting.name != "REQUEST_BODY_TIMEOUT") {
				if setting.name == "REQUEST_BODY_TIMEOUT" {
					return fmt.Errorf("%s must be a positive duration or 0 for no request body timeout", setting.name)
				}
				return fmt.Errorf("%s must be a positive duration", setting.name)
			}
			*setting.target = duration
		}
	}
	return nil
}

func positiveRequestLimit(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func positiveRequestTimeout(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func readRequestBody(w http.ResponseWriter, r *http.Request, timeout time.Duration) (body []byte, err error) {
	return readRequestBodyLimited(w, r, timeout, defaultGatewayBodyMaxBytes)
}

func readRequestBodyLimited(w http.ResponseWriter, r *http.Request, timeout time.Duration, maxBytes int64) (body []byte, err error) {
	controller := http.NewResponseController(w)
	defer func() {
		if err == nil {
			_ = controller.SetReadDeadline(time.Time{})
		} else {
			_ = controller.SetReadDeadline(time.Now())
			if r.ProtoMajor == 1 {
				w.Header().Set("Connection", "close")
			}
		}
	}()
	if maxBytes <= 0 {
		maxBytes = defaultGatewayBodyMaxBytes
	}
	if r.ContentLength > maxBytes {
		return nil, errRequestBodyTooLarge
	}
	if timeout > 0 {
		if deadlineErr := controller.SetReadDeadline(time.Now().Add(timeout)); deadlineErr != nil && !errors.Is(deadlineErr, http.ErrNotSupported) {
			return nil, deadlineErr
		}
	}
	body, err = io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errRequestBodyTooLarge
	}
	return body, nil
}

var errRequestBodyTooLarge = errors.New("request body exceeds size limit")

func (s *Service) requestBodyTimeout(ctx context.Context) time.Duration {
	if s == nil || s.db == nil || s.reliabilityData == nil {
		if s != nil {
			return s.cfg.RequestBodyTimeout
		}
		return defaultRequestBodyTimeout
	}
	seconds := s.reliabilitySettings(ctx).RequestBodyTimeoutSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func parseRequestForm(w http.ResponseWriter, r *http.Request, timeout time.Duration) error {
	if r.PostForm == nil && r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err == nil && (mediaType == "application/x-www-form-urlencoded" || mediaType == "multipart/form-data") {
			body, err := readRequestBodyLimited(w, r, timeout, defaultConsoleBodyMaxBytes)
			if err != nil {
				return err
			}
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}
	return r.ParseForm()
}

func requestBodyError(err error) (int, string, string) {
	if errors.Is(err, errRequestBodyTooLarge) {
		return http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the size limit"
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return http.StatusRequestTimeout, "request_timeout", "request body read timed out"
	}
	return http.StatusBadRequest, "invalid_request", "could not read request body"
}

func (s *Service) rejectRequestBody(w http.ResponseWriter, r *http.Request, err error, started time.Time) {
	status, code, message := requestBodyError(err)
	s.logReject(r.Context(), "", status, code, started)
	writeError(w, status, code, message)
}

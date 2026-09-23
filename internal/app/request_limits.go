package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGatewayMaxBodyBytes   = 2 << 20
	defaultImageMaxBodyBytes     = 50 << 20
	defaultWSMaxMessageBytes     = 2 << 20
	defaultRequestBodyTimeout    = 30 * time.Second
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
		{"GATEWAY_MAX_BODY_BYTES", &c.GatewayMaxBodyBytes, defaultGatewayMaxBodyBytes},
		{"IMAGE_MAX_BODY_BYTES", &c.ImageMaxBodyBytes, defaultImageMaxBodyBytes},
		{"WS_MAX_MESSAGE_BYTES", &c.WSMaxMessageBytes, defaultWSMaxMessageBytes},
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
			if err != nil || duration <= 0 {
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

func readRequestBody(w http.ResponseWriter, r *http.Request, limit int64, timeout time.Duration) (body []byte, err error) {
	limit = positiveRequestLimit(limit, defaultGatewayMaxBodyBytes)
	timeout = positiveRequestTimeout(timeout, defaultRequestBodyTimeout)
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
	if deadlineErr := controller.SetReadDeadline(time.Now().Add(timeout)); deadlineErr != nil && !errors.Is(deadlineErr, http.ErrNotSupported) {
		return nil, deadlineErr
	}
	if r.ContentLength > limit {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	body, err = io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func requestBodyError(err error) (int, string, string) {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return http.StatusRequestEntityTooLarge, "request_too_large", fmt.Sprintf("request body exceeds %d bytes", maxBytes.Limit)
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

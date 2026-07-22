package app

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRedirects = 5

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return validateRedirectURL(req.URL)
		},
	}
}

func validateRedirectURL(u *url.URL) error {
	if u == nil || u.Host == "" {
		return fmt.Errorf("redirect missing host")
	}
	host := u.Hostname()
	switch strings.ToLower(u.Scheme) {
	case "https":
		if isUnsafeRedirectHost(host) {
			return fmt.Errorf("redirect to non-public host is not allowed")
		}
		return nil
	case "http":
		if !isLoopbackHost(host) {
			return fmt.Errorf("redirect to non-loopback HTTP is not allowed")
		}
		return nil
	default:
		return fmt.Errorf("redirect scheme %q is not allowed", u.Scheme)
	}
}

func isUnsafeRedirectHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

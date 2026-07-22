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
	if u == nil {
		return fmt.Errorf("redirect missing host")
	}
	return validOutboundURL(u.String())
}

// validOutboundURL accepts:
//   - https:// to a public hostname or public IP (blocks private/link-local/loopback IP literals)
//   - http:// only to loopback (local services such as Ollama)
func validOutboundURL(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" {
		return fmt.Errorf("must be an HTTPS URL (HTTP is allowed for loopback)")
	}
	host := u.Hostname()
	switch strings.ToLower(u.Scheme) {
	case "https":
		if isNonPublicHost(host) {
			return fmt.Errorf("https URL must not target private, link-local, or loopback hosts")
		}
		return nil
	case "http":
		if !isLoopbackHost(host) {
			return fmt.Errorf("http is only allowed for loopback hosts")
		}
		return nil
	default:
		return fmt.Errorf("scheme must be https (or http for loopback)")
	}
}

// isNonPublicHost reports whether host is empty, localhost, or a non-public IP literal.
// Non-IP hostnames are treated as public (DNS-based SSRF is out of scope for this check).
func isNonPublicHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func isUnsafeRedirectHost(host string) bool {
	return isNonPublicHost(host)
}

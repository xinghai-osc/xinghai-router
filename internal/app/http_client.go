package app

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)

const maxRedirects = 5

// upstreamTransport is shared by every outbound client so keep-alive connections
// to provider endpoints are reused across requests. The stdlib default only keeps
// two idle connections per host, which forces a TCP+TLS handshake on almost every
// concurrent gateway request.
var upstreamTransport = sync.OnceValue(func() *http.Transport {
	idlePerHost := 32 * runtime.GOMAXPROCS(0)
	if idlePerHost < 64 {
		idlePerHost = 64
	}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8 * idlePerHost,
		MaxIdleConnsPerHost:   idlePerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ReadBufferSize:        64 << 10,
		WriteBufferSize:       64 << 10,
	}
})

func checkUpstreamRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	return validateRedirectURL(req.URL)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport:     upstreamTransport(),
		Timeout:       timeout,
		CheckRedirect: checkUpstreamRedirect,
	}
}

// newStreamClient has no overall deadline because an SSE response legitimately stays
// open far longer than RequestTimeout; the upstream must still send response headers
// within headerTimeout, and the request context still bounds the whole exchange.
func newStreamClient(headerTimeout time.Duration) *http.Client {
	base := upstreamTransport().Clone()
	base.ResponseHeaderTimeout = headerTimeout
	return &http.Client{
		Transport:     base,
		CheckRedirect: checkUpstreamRedirect,
	}
}

func validateRedirectURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("redirect missing host")
	}
	return validUpstreamURL(u.String())
}

// validUpstreamURL accepts http:// or https:// to any host. Upstream base URLs are
// configured by the admin, who may legitimately point at plaintext or private-network
// endpoints (LAN proxies, internal one-api/new-api instances, Ollama, ...).
func validUpstreamURL(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" {
		return fmt.Errorf("must be an HTTP or HTTPS URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("scheme must be http or https")
	}
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

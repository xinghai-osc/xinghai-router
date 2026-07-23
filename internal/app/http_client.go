package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRedirects = 5

func newHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = safeDialContext(dialer)
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return validateRedirectURL(req.URL)
		},
	}
}

func safeDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if !allowDialIP(host, ip) {
				return nil, fmt.Errorf("connection to non-public address %s is not allowed", ip)
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, ipAddr := range ips {
			if !allowDialIP(host, ipAddr.IP) {
				if firstErr == nil {
					firstErr = fmt.Errorf("connection to non-public address %s for host %q is not allowed", ipAddr.IP, host)
				}
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipAddr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("no addresses for host %q", host)
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

func isNonPublicIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// isNonPublicHost reports whether host is empty, localhost, or a non-public IP literal.
// Non-IP hostnames are treated as public at parse time; DialContext re-checks after DNS resolve.
func isNonPublicHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isNonPublicIP(ip)
}

// allowDialIP reports whether connecting to ip for request host is allowed.
// Public IPs are always allowed. Loopback IPs are allowed only when the original
// host is intentionally loopback (Ollama). Private/link-local/metadata IPs are never allowed,
// blocking DNS rebinding SSRF even when the URL hostname looks public.
func allowDialIP(host string, ip net.IP) bool {
	if ip == nil {
		return false
	}
	if !isNonPublicIP(ip) {
		return true
	}
	return isLoopbackHost(host) && ip.IsLoopback()
}

// isUnsafeRedirectHost is kept as an alias for redirect checks (same policy as first-hop HTTPS).
func isUnsafeRedirectHost(host string) bool {
	return isNonPublicHost(host)
}

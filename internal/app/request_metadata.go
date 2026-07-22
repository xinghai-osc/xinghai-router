package app

import (
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"sync"
)

type requestMetadataInfo struct {
	clientIP, forwardedFor, userAgent string
	browser, browserVersion           string
	operatingSystem, operatingSystemVersion, deviceType string
	isBot bool
}

var uaVersion = regexp.MustCompile(`(?:Chrome|Firefox|Version|Edg|OPR|CriOS|FxiOS|MSIE)[ /]([\d.]+)`)

var (
	trustedProxiesMu   sync.RWMutex
	trustedProxyNets   []netip.Prefix
	trustProxyHeaders  bool
)

func setTrustedProxies(specs string) error {
	nets, err := parseTrustedProxies(specs)
	if err != nil {
		return err
	}
	trustedProxiesMu.Lock()
	trustedProxyNets = nets
	trustProxyHeaders = len(nets) > 0
	trustedProxiesMu.Unlock()
	return nil
}

func parseTrustedProxies(specs string) ([]netip.Prefix, error) {
	specs = strings.TrimSpace(specs)
	if specs == "" {
		return nil, nil
	}
	var out []netip.Prefix
	for _, part := range strings.Split(specs, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if lower == "loopback" {
			out = append(out, netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128"))
			continue
		}
		if lower == "private" {
			out = append(out,
				netip.MustParsePrefix("10.0.0.0/8"),
				netip.MustParsePrefix("172.16.0.0/12"),
				netip.MustParsePrefix("192.168.0.0/16"),
				netip.MustParsePrefix("fc00::/7"),
			)
			continue
		}
		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, err
			}
			out = append(out, prefix)
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, err
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		out = append(out, netip.PrefixFrom(addr, bits))
	}
	return out, nil
}

func remoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

func isTrustedProxy(remoteAddr string, nets []netip.Prefix) bool {
	addr, ok := remoteAddrIP(remoteAddr)
	if !ok {
		return false
	}
	for _, prefix := range nets {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func clientIPFromRequest(r *http.Request, nets []netip.Prefix) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if len(nets) > 0 && isTrustedProxy(r.RemoteAddr, nets) {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			if addr, err := netip.ParseAddr(strings.Trim(realIP, "[]")); err == nil {
				return addr.String()
			}
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			if comma := strings.IndexByte(xff, ','); comma >= 0 {
				xff = strings.TrimSpace(xff[:comma])
			}
			if addr, err := netip.ParseAddr(strings.Trim(xff, "[]")); err == nil {
				return addr.String()
			}
		}
	}
	if addr, ok := remoteAddrIP(r.RemoteAddr); ok {
		return addr.String()
	}
	return remote
}

func requestMetadataFromUA(ua string) (browser, version, operatingSystem, osVersion, device string, bot bool) {
	lower := strings.ToLower(ua)
	bot = strings.Contains(lower, "bot") || strings.Contains(lower, "crawler") || strings.Contains(lower, "spider")
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "opr/"):
		browser = "Opera"
	case strings.Contains(lower, "chrome/") || strings.Contains(lower, "crios/"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox/") || strings.Contains(lower, "fxios/"):
		browser = "Firefox"
	case strings.Contains(lower, "safari/"):
		browser = "Safari"
	case strings.Contains(lower, "msie") || strings.Contains(lower, "trident/"):
		browser = "Internet Explorer"
	default:
		browser = "Other"
	}
	if match := uaVersion.FindStringSubmatch(ua); len(match) > 1 {
		version = match[1]
	}
	switch {
	case strings.Contains(lower, "windows"):
		operatingSystem, device = "Windows", "desktop"
	case strings.Contains(lower, "android"):
		operatingSystem, device = "Android", "mobile"
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad"):
		operatingSystem, device = "iOS", "mobile"
	case strings.Contains(lower, "mac os"):
		operatingSystem, device = "macOS", "desktop"
	case strings.Contains(lower, "linux"):
		operatingSystem, device = "Linux", "desktop"
	default:
		operatingSystem, device = "Other", "unknown"
	}
	return
}

func requestMetadata(r *http.Request) requestMetadataInfo {
	ua := strings.TrimSpace(r.UserAgent())
	trustedProxiesMu.RLock()
	nets := trustedProxyNets
	trustedProxiesMu.RUnlock()
	clientIP := clientIPFromRequest(r, nets)
	browser, browserVersion, os, osVersion, device, bot := requestMetadataFromUA(ua)
	return requestMetadataInfo{clientIP: clientIP, forwardedFor: strings.TrimSpace(r.Header.Get("X-Forwarded-For")), userAgent: ua, browser: browser, browserVersion: browserVersion, operatingSystem: os, operatingSystemVersion: osVersion, deviceType: device, isBot: bot}
}

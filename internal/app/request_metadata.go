package app

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"sync"
)

type requestMetadataInfo struct {
	clientIP, forwardedFor, userAgent                   string
	browser, browserVersion                             string
	operatingSystem, operatingSystemVersion, deviceType string
	isBot                                               bool
}

var uaVersion = regexp.MustCompile(`(?:Chrome|Firefox|Version|Edg|OPR|CriOS|FxiOS|MSIE)[ /]([\d.]+)`)

var (
	trustedProxiesMu  sync.RWMutex
	trustedProxyNets  []netip.Prefix
	trustProxyHeaders bool
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
			if prefix.Addr().Is4In6() {
				if prefix.Bits() < 96 {
					return nil, fmt.Errorf("mapped IPv4 proxy prefix must be at least /96: %s", part)
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil || addr.Zone() != "" {
			return nil, fmt.Errorf("invalid proxy address: %s", part)
		}
		addr = addr.Unmap()
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
	return addr.WithZone("").Unmap(), true
}

func isTrustedProxy(remoteAddr string, nets []netip.Prefix) bool {
	addr, ok := remoteAddrIP(remoteAddr)
	return ok && trustedProxyAddress(addr, nets)
}

func trustedProxyAddress(addr netip.Addr, nets []netip.Prefix) bool {
	for _, prefix := range nets {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func forwardedIPAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = value[1 : len(value)-1]
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func clientIPFromRequest(r *http.Request, nets []netip.Prefix) string {
	peer, ok := remoteAddrIP(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}
	if !trustedProxyAddress(peer, nets) {
		return peer.String()
	}
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		forwarded := strings.Join(values, ",")
		hops := strings.Split(forwarded, ",")
		if len(forwarded) > 4096 || len(hops) > 32 {
			return peer.String()
		}
		current := peer
		for i := len(hops) - 1; i >= 0; i-- {
			if !trustedProxyAddress(current, nets) {
				return current.String()
			}
			next, valid := forwardedIPAddress(hops[i])
			if !valid {
				return peer.String()
			}
			current = next
		}
		return current.String()
	}
	if values := r.Header.Values("X-Real-IP"); len(values) == 1 {
		if addr, valid := forwardedIPAddress(values[0]); valid {
			return addr.String()
		}
	}
	return peer.String()
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

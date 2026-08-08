package app

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// validateSourceDSN rejects a migration source DSN whose target host would let a
// system.manage operator pivot the router into the internal network. Loopback and
// public Internet hosts are accepted; private, link-local, and reserved ranges are
// not. Hostnames are treated as public (DNS-based SSRF is out of scope, matching
// validOutboundURL).
func validateSourceDSN(dsn string) error {
	host, err := migrationDSNHost(dsn)
	if err != nil {
		return err
	}
	if !validSourceDBHost(host) {
		return fmt.Errorf("source database host %q is not reachable: private and link-local addresses are blocked", host)
	}
	return nil
}

// migrationDSNHost extracts the network host from a mysql- or postgres-style DSN.
// Empty and socket-based targets map to "localhost"; absolute sqlite paths and
// postgres key=value DSNs without a host are also treated as local.
func migrationDSNHost(dsn string) (string, error) {
	raw := strings.TrimSpace(dsn)
	// URL forms: mysql://user:pass@host:port/db, postgres://...
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Hostname(), nil
	}
	// MySQL protocol form: user:pass@tcp(host:port)/db, user:pass@unix(/path)/db,
	// user:pass@(host:port)/db, or a bare host[:port]: user:pass@db:3306/db.
	if at := strings.IndexByte(raw, '@'); at >= 0 {
		authority := raw[at+1:]
		if i := strings.IndexByte(authority, '/'); i >= 0 {
			authority = authority[:i]
		}
		switch {
		case authority == "":
			return "localhost", nil // default tcp socket on localhost
		case strings.HasPrefix(authority, "unix("):
			return "localhost", nil
		case strings.HasPrefix(authority, "tcp(") || strings.HasPrefix(authority, "udp(") || strings.HasPrefix(authority, "("):
			open, close := strings.IndexByte(authority, '('), strings.LastIndexByte(authority, ')')
			if open < 0 || close <= open {
				return "", fmt.Errorf("malformed protocol address in source DSN")
			}
			return stripDSNHostPort(authority[open+1 : close]), nil
		default:
			return stripDSNHostPort(authority), nil
		}
	}
	// PostgreSQL key=value form: host=db.example port=5432 ...
	for _, kv := range strings.Fields(raw) {
		name, value, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(name, "host") {
			return strings.TrimSpace(value), nil
		}
	}
	// sqlite path or a DSN we cannot place: treated as local.
	return "localhost", nil
}

func stripDSNHostPort(host string) string {
	if strings.HasPrefix(host, "/") {
		return "localhost" // socket path
	}
	if strings.HasPrefix(host, "[") {
		if i := strings.IndexByte(host, ']'); i > 0 {
			return host[1:i]
		}
	}
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		return host[:i]
	}
	return host
}

// validSourceDBHost allows loopback and public addresses but blocks the ranges a
// migration DSN could otherwise point at to reach the internal network.
func validSourceDBHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	if i := strings.IndexByte(host, '%'); i >= 0 {
		host = host[:i] // drop IPv6 zone identifiers
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true // hostname: assumed public
	}
	if ip.IsLoopback() || ip.IsUnspecified() {
		return true
	}
	if r := ip.To4(); r != nil {
		// 100.64.0.0/10 shared CGNAT space
		if r[0] == 100 && r[1]&0xc0 == 0x40 {
			return false
		}
		// 192.0.0.0/24 IETF protocol assignments
		if r[0] == 192 && r[1] == 0 && r[2] == 0 {
			return false
		}
		// 198.18.0.0/15 benchmarking
		if r[0] == 198 && r[1]&0xfe == 0x12 {
			return false
		}
	}
	return !(ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast())
}

// migrationCredentialRE matches the user:password@ head of a database authority.
// It is deliberately narrow so only credential-bearing text is rewritten.
var migrationCredentialRE = regexp.MustCompile(`(?i)([a-z][a-z0-9_.+-]*):([^/\s@]*)@`)

// redactMigrationError strips credentials a driver error may echo back verbatim,
// most commonly a mysql DSN, before the text is stored or rendered. dsn is the
// caller's exact DSN; passing "" still strips generic credential heads.
func redactMigrationError(msg, dsn string) string {
	if msg == "" {
		return ""
	}
	if dsn != "" {
		msg = strings.ReplaceAll(msg, dsn, "[redacted]")
	}
	return strings.TrimSpace(migrationCredentialRE.ReplaceAllString(msg, "${1}:***@"))
}

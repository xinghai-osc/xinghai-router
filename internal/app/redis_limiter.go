package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const redisLimitScript = "local current = redis.call('INCR', KEYS[1])\nif current == 1 then\n  redis.call('EXPIRE', KEYS[1], ARGV[1])\nend\nreturn current\n"

const redisPoolSize = 8

type redisConnection struct {
	net.Conn
	reader *bufio.Reader
}

type redisLimiter struct {
	initOnce  sync.Once
	closeOnce sync.Once
	maxConns  int
	gate      chan struct{}
	idle      chan redisConnection
	lifecycle context.Context
	stop      context.CancelFunc
	// scriptSHA is the cached SCRIPT LOAD digest of redisLimitScript.
	scriptSHA atomic.Value
	addr      string
	username  string
	password  string
	db        int
	useTLS    bool
	perMinute int
	keyPrefix string
	timeout   time.Duration
}

func newRedisClient(redisURL string, perMinute int) (*redisLimiter, error) {
	if strings.TrimSpace(redisURL) == "" {
		return nil, errors.New("redis url is empty")
	}
	if perMinute <= 0 {
		perMinute = 60
	}
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, errors.New("invalid redis url")
	}
	switch strings.ToLower(u.Scheme) {
	case "redis", "rediss":
	default:
		return nil, errors.New("unsupported redis scheme")
	}
	host := u.Hostname()
	if host == "" || u.Opaque != "" {
		return nil, errors.New("redis host is required")
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("redis url query and fragment are not supported")
	}
	port := u.Port()
	if port == "" {
		if strings.HasSuffix(u.Host, ":") {
			return nil, errors.New("invalid redis port")
		}
		port = "6379"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return nil, errors.New("invalid redis port")
	}
	db := 0
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		n, convErr := strconv.Atoi(path)
		if convErr != nil || n < 0 {
			return nil, errors.New("invalid redis database")
		}
		db = n
	}
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	return &redisLimiter{
		maxConns:  redisPoolSize,
		addr:      net.JoinHostPort(host, port),
		username:  username,
		password:  password,
		db:        db,
		useTLS:    strings.EqualFold(u.Scheme, "rediss"),
		perMinute: perMinute,
		keyPrefix: "xh:rl:",
		timeout:   2 * time.Second,
	}, nil
}

func newRedisLimiter(redisURL string, perMinute int) (*redisLimiter, error) {
	l, err := newRedisClient(redisURL, perMinute)
	if err != nil {
		return nil, err
	}
	if _, err := l.command(context.Background(), "PING"); err != nil {
		l.close()
		return nil, err
	}
	return l, nil
}

func (l *redisLimiter) init() {
	l.initOnce.Do(func() {
		limit := max(1, l.maxConns)
		l.gate = make(chan struct{}, limit)
		l.idle = make(chan redisConnection, limit)
		l.lifecycle, l.stop = context.WithCancel(context.Background())
	})
}

func (l *redisLimiter) command(ctx context.Context, args ...string) (reply any, err error) {
	l.init()
	timeout := l.timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// When the limiter shuts down, the command's context must be cancelled too:
	// a stalled read is interrupted by closing the connection, which is what
	// lets close() take the gate instead of waiting out the full timeout.
	stopClose := context.AfterFunc(l.lifecycle, cancel)
	defer stopClose()
	if l.lifecycle.Err() != nil {
		return nil, net.ErrClosed
	}
	select {
	case <-ctx.Done():
		if l.lifecycle.Err() != nil {
			return nil, net.ErrClosed
		}
		return nil, ctx.Err()
	case l.gate <- struct{}{}:
	}
	defer func() { <-l.gate }()
	if l.lifecycle.Err() != nil {
		return nil, net.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("redis command is empty")
	}
	var conn redisConnection
	select {
	case conn = <-l.idle:
	default:
	}
	defer func() {
		if l.lifecycle.Err() != nil {
			err = net.ErrClosed
		} else if ctx.Err() != nil {
			err = ctx.Err()
		} else if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			err = context.DeadlineExceeded
		}
		if err == nil {
			l.idle <- conn
		} else {
			reply = nil
			if conn.Conn != nil {
				_ = conn.Close()
			}
		}
	}()
	fresh := conn.Conn == nil
	if fresh {
		dialer := &net.Dialer{}
		if l.useTLS {
			tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostFromAddr(l.addr)}}
			conn.Conn, err = tlsDialer.DialContext(ctx, "tcp", l.addr)
		} else {
			conn.Conn, err = dialer.DialContext(ctx, "tcp", l.addr)
		}
		if err != nil {
			return nil, fmt.Errorf("dial redis: %w", err)
		}
		conn.reader = bufio.NewReader(conn.Conn)
	}
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	interrupted := make(chan struct{})
	stopIO := context.AfterFunc(ctx, func() {
		_ = conn.Close()
		close(interrupted)
	})
	defer func() {
		if !stopIO() {
			<-interrupted
		}
	}()
	if fresh {
		if l.password != "" || l.username != "" {
			auth := []string{"AUTH", l.password}
			if l.username != "" {
				auth = []string{"AUTH", l.username, l.password}
			}
			if _, err := conn.exchange(auth...); err != nil {
				return nil, fmt.Errorf("redis auth: %w", err)
			}
		}
		if l.db != 0 {
			if _, err := conn.exchange("SELECT", strconv.Itoa(l.db)); err != nil {
				return nil, fmt.Errorf("redis select: %w", err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return conn.exchange(args...)
}

func (conn redisConnection) exchange(args ...string) (any, error) {
	if err := writeCommand(conn.Conn, args...); err != nil {
		return nil, err
	}
	return readReply(conn.reader)
}

func hostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func (l *redisLimiter) tryAllow(key string) (bool, error) {
	return l.tryAllowN(key, l.perMinute)
}

// tryAllowN counts the key inside the fixed window. When the server returns a
// valid SCRIPT LOAD digest the script is afterwards invoked via EVALSHA, so
// the per-request payload is a digest instead of the whole script body. Any
// NOSCRIPT reply (for example after a Redis restart) falls back to loading it
// again; a server that never returns a digest keeps using EVAL.
func (l *redisLimiter) tryAllowN(key string, n int) (bool, error) {
	if n <= 0 {
		n = l.perMinute
	}
	redisKey := l.keyPrefix + key
	cached, _ := l.scriptSHA.Load().(string)
	if cached == "" {
		reply, err := l.command(context.Background(), "SCRIPT", "LOAD", redisLimitScript)
		if err != nil {
			return l.tryAllowEval(key, n)
		}
		digest, _ := reply.(string)
		if !validScriptDigest(digest) {
			// The server answered something unexpected; fall back to EVAL,
			// which every Redis speaks.
			return l.tryAllowEval(key, n)
		}
		l.scriptSHA.Store(digest)
		cached = digest
	}
	reply, err := l.command(context.Background(), "EVALSHA", cached, "1", redisKey, "60")
	if err != nil {
		if !isNoScriptError(err) {
			return false, err
		}
		l.scriptSHA.Store("")
		// The server lost its script cache (restart or SCRIPT FLUSH): load it
		// again and retry once, falling back to EVAL when it still misbehaves.
		if loadReply, loadErr := l.command(context.Background(), "SCRIPT", "LOAD", redisLimitScript); loadErr == nil {
			if digest, _ := loadReply.(string); validScriptDigest(digest) {
				l.scriptSHA.Store(digest)
				reply, err = l.command(context.Background(), "EVALSHA", digest, "1", redisKey, "60")
				if err == nil {
					return parseAllowReply(reply, n)
				}
				if !isNoScriptError(err) {
					return false, err
				}
			}
		}
		return l.tryAllowEval(key, n)
	}
	return parseAllowReply(reply, n)
}

func (l *redisLimiter) tryAllowEval(key string, n int) (bool, error) {
	reply, err := l.command(context.Background(), "EVAL", redisLimitScript, "1", l.keyPrefix+key, "60")
	if err != nil {
		return false, err
	}
	return parseAllowReply(reply, n)
}

// validScriptDigest reports whether value looks like the 40-hex-character SHA1
// digest SCRIPT LOAD returns.
func validScriptDigest(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func parseAllowReply(reply any, n int) (bool, error) {
	count, ok := reply.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis reply %T", reply)
	}
	return count <= int64(n), nil
}

func isNoScriptError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT")
}

func (l *redisLimiter) close() {
	l.init()
	l.closeOnce.Do(func() {
		l.stop()
		for range cap(l.gate) {
			l.gate <- struct{}{}
		}
		close(l.idle)
		for conn := range l.idle {
			_ = conn.Close()
		}
	})
}

// cleanup is a no-op for the Redis backend; entry expiry is handled by Redis.
func (l *redisLimiter) cleanup() {}

func writeCommand(w io.Writer, args ...string) error {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, arg := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteString("\r\n")
		b.WriteString(arg)
		b.WriteString("\r\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func readReply(r *bufio.Reader) (any, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return line, nil
	case '-':
		return nil, errors.New(line)
	case ':':
		n, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		items := make([]any, 0, n)
		for range n {
			item, err := readReply(r)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unknown redis reply prefix %q", prefix)
	}
}

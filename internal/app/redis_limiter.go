package app

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const redisLimitScript = "local current = redis.call('INCR', KEYS[1])\nif current == 1 then\n  redis.call('EXPIRE', KEYS[1], ARGV[1])\nend\nreturn current\n"

type redisLimiter struct {
	mu        sync.Mutex
	conn      net.Conn
	reader    *bufio.Reader
	addr      string
	password  string
	db        int
	useTLS    bool
	perMinute int
	keyPrefix string
	timeout   time.Duration
}

func newRedisLimiter(redisURL string, perMinute int) (*redisLimiter, error) {
	if strings.TrimSpace(redisURL) == "" {
		return nil, fmt.Errorf("redis url is empty")
	}
	if perMinute <= 0 {
		perMinute = 60
	}
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "redis", "rediss":
	default:
		return nil, fmt.Errorf("unsupported redis scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("redis host is required")
	}
	port := u.Port()
	if port == "" {
		port = "6379"
	}
	db := 0
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		n, convErr := strconv.Atoi(path)
		if convErr != nil || n < 0 {
			return nil, fmt.Errorf("invalid redis database %q", path)
		}
		db = n
	}
	password := ""
	if u.User != nil {
		if p, ok := u.User.Password(); ok {
			password = p
		}
	}
	l := &redisLimiter{
		addr:      net.JoinHostPort(host, port),
		password:  password,
		db:        db,
		useTLS:    strings.EqualFold(u.Scheme, "rediss"),
		perMinute: perMinute,
		keyPrefix: "xh:rl:",
		timeout:   2 * time.Second,
	}
	if err := l.connect(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *redisLimiter) connect() error {
	dialer := net.Dialer{Timeout: l.timeout}
	var conn net.Conn
	var err error
	if l.useTLS {
		conn, err = tls.DialWithDialer(&dialer, "tcp", l.addr, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostFromAddr(l.addr)})
	} else {
		conn, err = dialer.Dial("tcp", l.addr)
	}
	if err != nil {
		return fmt.Errorf("dial redis: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(l.timeout))
	reader := bufio.NewReader(conn)
	if l.password != "" {
		if err := writeCommand(conn, "AUTH", l.password); err != nil {
			conn.Close()
			return err
		}
		if _, err := readReply(reader); err != nil {
			conn.Close()
			return fmt.Errorf("redis auth: %w", err)
		}
	}
	if l.db != 0 {
		if err := writeCommand(conn, "SELECT", strconv.Itoa(l.db)); err != nil {
			conn.Close()
			return err
		}
		if _, err := readReply(reader); err != nil {
			conn.Close()
			return fmt.Errorf("redis select: %w", err)
		}
	}
	_ = conn.SetDeadline(time.Time{})
	l.conn = conn
	l.reader = reader
	return nil
}

func hostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func (l *redisLimiter) tryAllow(key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		if err := l.connect(); err != nil {
			return false, err
		}
	}
	_ = l.conn.SetDeadline(time.Now().Add(l.timeout))
	defer l.conn.SetDeadline(time.Time{})
	redisKey := l.keyPrefix + key
	if err := writeCommand(l.conn, "EVAL", redisLimitScript, "1", redisKey, "60"); err != nil {
		l.resetConn()
		return false, err
	}
	reply, err := readReply(l.reader)
	if err != nil {
		l.resetConn()
		return false, err
	}
	count, ok := reply.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis reply %T", reply)
	}
	return count <= int64(l.perMinute), nil
}

func (l *redisLimiter) resetConn() {
	if l.conn != nil {
		_ = l.conn.Close()
	}
	l.conn = nil
	l.reader = nil
}

func (l *redisLimiter) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.resetConn()
}

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

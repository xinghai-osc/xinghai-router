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
	"time"
)

const redisLimitScript = "local current = redis.call('INCR', KEYS[1])\nif current == 1 then\n  redis.call('EXPIRE', KEYS[1], ARGV[1])\nend\nreturn current\n"

type redisLimiter struct {
	initOnce  sync.Once
	gate      chan struct{}
	lifecycle context.Context
	stop      context.CancelFunc
	conn      net.Conn
	reader    *bufio.Reader
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
		l.gate = make(chan struct{}, 1)
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
	stopClose := context.AfterFunc(l.lifecycle, cancel)
	defer stopClose()
	if l.lifecycle.Err() != nil {
		return nil, net.ErrClosed
	}
	select {
	case <-ctx.Done():
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
	defer func() {
		if l.lifecycle.Err() != nil {
			err = net.ErrClosed
		} else if ctx.Err() != nil {
			err = ctx.Err()
		} else if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			err = context.DeadlineExceeded
		}
		if err != nil {
			l.resetConn()
			reply = nil
		}
	}()
	fresh := l.conn == nil
	if fresh {
		dialer := &net.Dialer{}
		if l.useTLS {
			tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostFromAddr(l.addr)}}
			l.conn, err = tlsDialer.DialContext(ctx, "tcp", l.addr)
		} else {
			l.conn, err = dialer.DialContext(ctx, "tcp", l.addr)
		}
		if err != nil {
			return nil, fmt.Errorf("dial redis: %w", err)
		}
		l.reader = bufio.NewReader(l.conn)
	}
	conn := l.conn
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
			if _, err := l.exchange(auth...); err != nil {
				return nil, fmt.Errorf("redis auth: %w", err)
			}
		}
		if l.db != 0 {
			if _, err := l.exchange("SELECT", strconv.Itoa(l.db)); err != nil {
				return nil, fmt.Errorf("redis select: %w", err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.exchange(args...)
}

func (l *redisLimiter) exchange(args ...string) (any, error) {
	if err := writeCommand(l.conn, args...); err != nil {
		return nil, err
	}
	return readReply(l.reader)
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

func (l *redisLimiter) tryAllowN(key string, n int) (bool, error) {
	if n <= 0 {
		n = l.perMinute
	}
	redisKey := l.keyPrefix + key
	reply, err := l.command(context.Background(), "EVAL", redisLimitScript, "1", redisKey, "60")
	if err != nil {
		return false, err
	}
	count, ok := reply.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis reply %T", reply)
	}
	return count <= int64(n), nil
}

func (l *redisLimiter) resetConn() {
	if l.conn != nil {
		_ = l.conn.Close()
	}
	l.conn = nil
	l.reader = nil
}

func (l *redisLimiter) close() {
	l.init()
	l.stop()
	l.gate <- struct{}{}
	l.resetConn()
	<-l.gate
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

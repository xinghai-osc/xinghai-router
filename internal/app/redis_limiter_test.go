package app

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteCommandAndReadReplyInteger(t *testing.T) {
	var b strings.Builder
	if err := writeCommand(&b, "INCR", "xh:rl:key"); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.HasPrefix(got, "*2\r\n$4\r\nINCR\r\n") {
		t.Fatalf("unexpected RESP: %q", got)
	}
	reader := bufio.NewReader(strings.NewReader(":3\r\n"))
	reply, err := readReply(reader)
	if err != nil {
		t.Fatal(err)
	}
	if reply.(int64) != 3 {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestReadReplyErrorAndBulk(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("-ERR auth\r\n"))
	if _, err := readReply(reader); err == nil || !strings.Contains(err.Error(), "ERR auth") {
		t.Fatalf("expected redis error, got %v", err)
	}
	reader = bufio.NewReader(strings.NewReader("$5\r\nhello\r\n"))
	reply, err := readReply(reader)
	if err != nil || reply.(string) != "hello" {
		t.Fatalf("bulk reply = %#v %v", reply, err)
	}
}

func TestNewRedisLimiterValidation(t *testing.T) {
	if _, err := newRedisLimiter("http://localhost:6379", 10); err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if _, err := newRedisLimiter("", 10); err == nil {
		t.Fatal("expected empty url error")
	}
	if _, err := newRedisLimiter("redis://localhost:6379/not-a-db", 10); err == nil {
		t.Fatal("expected invalid database error")
	}
}

func TestFallbackLimiterUsesMemoryOnRedisError(t *testing.T) {
	primary := &redisLimiter{addr: "127.0.0.1:1", perMinute: 1, timeout: 50 * time.Millisecond}
	backup := newMemoryLimiter(1)
	l := &fallbackLimiter{primary: primary, backup: backup}
	if !l.allow("k") {
		t.Fatal("first request should succeed via memory fallback")
	}
	if l.allow("k") {
		t.Fatal("second request should hit memory limit")
	}
	l.close()
}

func TestNewRedisClientParseOnly(t *testing.T) {
	client, err := newRedisClient("redis://named-user:fake-password@"+unusedLimiterTestAddress(t)+"/2", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()
	if client.conn != nil || client.perMinute != 60 || client.db != 2 || client.username != "named-user" || client.password != "fake-password" {
		t.Fatalf("unexpected parse-only client configuration: connected=%v perMinute=%d db=%d", client.conn != nil, client.perMinute, client.db)
	}
}

func TestRedisCommandAuthenticationSelectionAndRESP(t *testing.T) {
	var mu sync.Mutex
	var commands [][]string
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		mu.Lock()
		commands = append(commands, args)
		mu.Unlock()
		if args[0] == "EVAL" {
			return "*3\r\n:2\r\n$5\r\nvalue\r\n$-1\r\n"
		}
		return "+OK\r\n"
	})
	client, err := newRedisClient("redis://named-user:fake-password@"+server.listener.Addr().String()+"/2", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()
	reply, err := client.command(context.Background(), "EVAL", "return {2, 'value', false}", "0")
	if err != nil {
		t.Fatal(err)
	}
	items, ok := reply.([]any)
	if !ok || len(items) != 3 || items[0] != int64(2) || items[1] != "value" || items[2] != nil {
		t.Fatalf("unexpected array reply: %#v", reply)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(commands) != 3 || strings.Join(commands[0], " ") != "AUTH named-user fake-password" || strings.Join(commands[1], " ") != "SELECT 2" || commands[2][0] != "EVAL" {
		t.Fatalf("unexpected authentication/selection order: %#v", commands)
	}
}

func TestRedisCommandSetupUsesSameTimeout(t *testing.T) {
	for _, blockedCommand := range []string{"AUTH", "SELECT"} {
		t.Run(blockedCommand, func(t *testing.T) {
			release := make(chan struct{})
			defer close(release)
			var commands atomic.Int64
			server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
				commands.Add(1)
				if args[0] == blockedCommand {
					<-release
				}
				return "+OK\r\n"
			})
			client, err := newRedisClient("redis://:fake-password@"+server.listener.Addr().String()+"/2", 1)
			if err != nil {
				t.Fatal(err)
			}
			defer client.close()
			client.timeout = 40 * time.Millisecond
			start := time.Now()
			if _, err := client.command(context.Background(), "INCR", "key"); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("setup timeout error = %v", err)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("setup exceeded command deadline: %s", elapsed)
			}
			want := int64(1)
			if blockedCommand == "SELECT" {
				want = 2
			}
			if commands.Load() != want {
				t.Fatalf("command was sent after setup failure: commands=%d want=%d", commands.Load(), want)
			}
		})
	}
}

func TestRedisCommandNoRetryAfterAmbiguousWrite(t *testing.T) {
	var attempts atomic.Int64
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		if attempts.Add(1) == 1 {
			return ""
		}
		return ":2\r\n"
	})
	client, err := newRedisClient(server.url(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()
	if reply, err := client.command(context.Background(), "INCR", "key"); err == nil || reply != nil {
		t.Fatalf("ambiguous write should report error: reply=%#v err=%v", reply, err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("command was automatically replayed %d times", attempts.Load())
	}
	reply, err := client.command(context.Background(), "INCR", "key")
	if err != nil || reply != int64(2) || attempts.Load() != 2 {
		t.Fatalf("subsequent command did not reconnect: reply=%#v err=%v attempts=%d", reply, err, attempts.Load())
	}
}

func newStalledRedisClient(t *testing.T) (*redisLimiter, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	conn, peer := net.Pipe()
	client := &redisLimiter{conn: conn, reader: bufio.NewReader(conn), timeout: 5 * time.Second}
	received := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer peer.Close()
		if _, err := readReply(bufio.NewReader(peer)); err != nil {
			return
		}
		close(received)
		_, _ = io.Copy(io.Discard, peer)
	}()
	t.Cleanup(func() {
		client.close()
		_ = peer.Close()
		<-finished
	})
	return client, received, finished
}

func waitRedisTestCommand(t *testing.T, received <-chan struct{}) {
	t.Helper()
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("redis command did not reach test peer")
	}
}

func TestRedisCommandDeadlineAndRecovery(t *testing.T) {
	client, _, _ := newStalledRedisClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := client.command(ctx, "PING"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("I/O ignored context deadline: %s", elapsed)
	}
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string { return "+PONG\r\n" })
	client.addr = server.listener.Addr().String()
	if reply, err := client.command(context.Background(), "PING"); err != nil || reply != "PONG" {
		t.Fatalf("deadline did not reset connection: reply=%#v err=%v", reply, err)
	}
}

func TestRedisCommandCancellationInterruptsIO(t *testing.T) {
	client, received, _ := newStalledRedisClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := client.command(ctx, "PING")
		result <- err
	}()
	waitRedisTestCommand(t, received)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt in-flight redis read")
	}
}

func TestRedisCommandLockWaitUsesOwnDeadline(t *testing.T) {
	client, received, _ := newStalledRedisClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() {
		_, err := client.command(ctx, "PING")
		first <- err
	}()
	waitRedisTestCommand(t, received)
	start := time.Now()
	results := make(chan error, 24)
	for range cap(results) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			_, err := client.command(ctx, "PING")
			results <- err
		}()
	}
	for range cap(results) {
		select {
		case err := <-results:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("queued command error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("queued redis commands formed an unbounded lock convoy")
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("lock wait timeouts serialized: %s", elapsed)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued cancellations disturbed active command: %v", err)
	}
}

func TestRedisCommandCanceledBeforeDial(t *testing.T) {
	client, err := newRedisClient("redis://"+unusedLimiterTestAddress(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.command(ctx, "PING"); !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled command dialed redis: %v", err)
	}
}

func TestRedisCommandDefaultTimeoutIncludesLockWait(t *testing.T) {
	client := &redisLimiter{timeout: 40 * time.Millisecond}
	client.init()
	client.gate <- struct{}{}
	defer func() {
		<-client.gate
		client.close()
	}()
	start := time.Now()
	if _, err := client.command(context.Background(), "PING"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("default timeout error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("default timeout excluded lock wait: %s", elapsed)
	}
}

func TestRedisCommandTLSHandshakeRespectsContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
	}()
	client, err := newRedisClient("rediss://"+listener.Addr().String(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := client.command(ctx, "PING"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TLS handshake error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("TLS handshake ignored context: %s", elapsed)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("TLS dial leaked connection after deadline")
	}
}

func TestRedisCloseInterruptsIOAndIsPermanent(t *testing.T) {
	client, received, _ := newStalledRedisClient(t)
	result := make(chan error, 1)
	go func() {
		_, err := client.command(context.Background(), "PING")
		result <- err
	}()
	waitRedisTestCommand(t, received)
	closed := make(chan struct{})
	go func() {
		client.close()
		client.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not interrupt in-flight command")
	}
	if err := <-result; !errors.Is(err, net.ErrClosed) {
		t.Fatalf("in-flight command close error = %v", err)
	}
	if _, err := client.command(context.Background(), "PING"); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("closed client attempted reconnection: %v", err)
	}
}

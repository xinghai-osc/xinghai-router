package app

import (
	"bufio"
	"strings"
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

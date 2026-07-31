package app

import (
	"sync"
	"testing"
)

func TestGroupLimiterAcquireRelease(t *testing.T) {
	gl := NewGroupLimiter()
	if !gl.acquire("g1", 2) {
		t.Fatal("first acquire should succeed")
	}
	if !gl.acquire("g1", 2) {
		t.Fatal("second acquire should succeed")
	}
	if gl.acquire("g1", 2) {
		t.Fatal("third acquire should be rejected")
	}
	gl.release("g1")
	if !gl.acquire("g1", 2) {
		t.Fatal("acquire after release should succeed")
	}
}

func TestGroupLimiterNoLimit(t *testing.T) {
	gl := NewGroupLimiter()
	for i := 0; i < 100; i++ {
		if !gl.acquire("g1", 0) {
			t.Fatal("zero max should never reject")
		}
	}
	for i := 0; i < 100; i++ {
		if !gl.acquire("g1", -1) {
			t.Fatal("negative max should never reject")
		}
	}
}

func TestGroupLimiterDynamicLimit(t *testing.T) {
	gl := NewGroupLimiter()
	if !gl.acquire("g1", 3) || !gl.acquire("g1", 3) || !gl.acquire("g1", 3) {
		t.Fatal("three acquires should succeed")
	}
	if gl.acquire("g1", 1) {
		t.Fatal("shrinking the limit should reject new acquires")
	}
	gl.release("g1")
	gl.release("g1")
	gl.release("g1")
	if !gl.acquire("g1", 1) {
		t.Fatal("release should free a slot under the new limit")
	}
}

func TestGroupLimiterIsolated(t *testing.T) {
	gl := NewGroupLimiter()
	if !gl.acquire("g1", 1) {
		t.Fatal("g1 acquire should succeed")
	}
	if gl.acquire("g1", 1) {
		t.Fatal("g1 second acquire should be rejected")
	}
	if !gl.acquire("g2", 1) {
		t.Fatal("g2 should be unaffected by g1")
	}
}

func TestGroupLimiterConcurrent(t *testing.T) {
	gl := NewGroupLimiter()
	const max = 10
	var wg sync.WaitGroup
	var held, peak int32
	var mu sync.Mutex
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if gl.acquire("g1", max) {
					mu.Lock()
					held++
					if held > peak {
						peak = held
					}
					mu.Unlock()
					gl.release("g1")
					mu.Lock()
					held--
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if peak > max {
		t.Fatalf("peak concurrency %d exceeds limit %d", peak, max)
	}
}

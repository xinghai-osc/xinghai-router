package app

import "sync"

type groupCounter struct {
	current int32
	max     int32
}

// GroupLimiter counts in-flight requests per key. The same type backs the
// channel, user, and group scopes, and each scope gets its own instance.
type GroupLimiter struct {
	mu       sync.Mutex
	counters map[string]*groupCounter
}

func NewGroupLimiter() *GroupLimiter {
	return &GroupLimiter{counters: make(map[string]*groupCounter)}
}

// acquire admits one request against the key's limit. A max of 0 or less means
// the key has no configured limit: the request is still counted so the console
// can report live concurrency, but it is never rejected.
func (gl *GroupLimiter) acquire(key string, max int) bool {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	c, ok := gl.counters[key]
	if !ok {
		c = &groupCounter{}
		gl.counters[key] = c
	}
	if max > 0 {
		c.max = int32(max)
	} else {
		c.max = 0
	}
	if c.max > 0 && c.current >= c.max {
		return false
	}
	c.current++
	return true
}

// release returns one slot. Idle keys are dropped so the map only keeps entries
// with requests actually in flight.
func (gl *GroupLimiter) release(key string) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	c, ok := gl.counters[key]
	if !ok {
		return
	}
	if c.current > 0 {
		c.current--
	}
	if c.current == 0 {
		delete(gl.counters, key)
	}
}

func (gl *GroupLimiter) snapshot(keys []string) map[string]int {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	out := make(map[string]int, len(keys))
	for _, key := range keys {
		if c, ok := gl.counters[key]; ok {
			out[key] = int(c.current)
		} else {
			out[key] = 0
		}
	}
	return out
}

// active reports every key with requests in flight in this process, whether or
// not it has a configured limit.
func (gl *GroupLimiter) active() map[string]int {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	out := make(map[string]int, len(gl.counters))
	for key, c := range gl.counters {
		if c.current > 0 {
			out[key] = int(c.current)
		}
	}
	return out
}

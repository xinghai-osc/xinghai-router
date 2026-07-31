package app

import "sync"

type groupCounter struct {
	mu      sync.Mutex
	current int32
	max     int32
}

type GroupLimiter struct {
	mu       sync.Mutex
	counters map[string]*groupCounter
}

func NewGroupLimiter() *GroupLimiter {
	return &GroupLimiter{counters: make(map[string]*groupCounter)}
}

func (gl *GroupLimiter) acquire(groupID string, max int) bool {
	if max <= 0 {
		return true
	}
	gl.mu.Lock()
	c, ok := gl.counters[groupID]
	if !ok {
		c = &groupCounter{max: int32(max)}
		gl.counters[groupID] = c
	}
	gl.mu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.max != int32(max) {
		c.max = int32(max)
	}
	if c.current >= c.max {
		return false
	}
	c.current++
	return true
}

func (gl *GroupLimiter) release(groupID string) {
	gl.mu.Lock()
	c, ok := gl.counters[groupID]
	gl.mu.Unlock()
	if ok {
		c.mu.Lock()
		if c.current > 0 {
			c.current--
		}
		c.mu.Unlock()
	}
}

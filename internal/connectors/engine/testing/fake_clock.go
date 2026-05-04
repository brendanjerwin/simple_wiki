package testing

import (
	"sync"
	"time"
)

// FakeClock is a programmable engine.Clock for unit tests. The engine's
// Bind ceremony stamps Binding.BoundAt with clock.Now(); tests assert
// that value equals the configured Now to verify the engine consults
// the clock seam (rather than calling time.Now directly).
//
// Usage:
//
//	fc := testing.NewFakeClock(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
//	engine.NewEngine(..., fc, ...)
//	// fc.Now() returns the configured time; SetNow advances it.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock constructs a FakeClock pinned to the given time.
func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now}
}

// Now returns the currently configured time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// SetNow updates the time the clock reports on subsequent Now calls.
func (c *FakeClock) SetNow(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

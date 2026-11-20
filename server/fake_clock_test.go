package server

import (
	"sync"
	"time"
)

// FakeClock is a controllable Clocker for the server package's tests. It lives
// in package server so both the internal (package server) and external
// (package server_test) test files share one implementation: a duplicate in the
// external package trips revive's confusing-naming rule, which compares
// identifiers across every file in the directory, and Now() cannot be renamed
// because Clocker mandates it.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock returns a FakeClock frozen at t.
func NewFakeClock(t time.Time) *FakeClock { return &FakeClock{now: t} }

// Now returns the frozen time, satisfying Clocker.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

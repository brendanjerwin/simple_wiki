package connectors

import (
	"sync"
	"time"
)

const (
	defaultFailureThreshold = 5
	defaultCooldownDuration = 5 * time.Minute
)

type circuitBreakerState string

const (
	circuitBreakerClosed   circuitBreakerState = "closed"
	circuitBreakerOpen     circuitBreakerState = "open"
	circuitBreakerHalfOpen circuitBreakerState = "half-open"
)

// CircuitBreaker guards a connector kind against repeatedly enqueueing
// sync jobs while the external service is unhealthy. After
// defaultFailureThreshold consecutive failures it opens and blocks new
// jobs for defaultCooldownDuration, then allows a single probe through
// (half-open). A successful probe closes the breaker; a failed probe
// reopens it.
type CircuitBreaker struct {
	mu                  sync.Mutex
	failures            int
	state               circuitBreakerState
	openedAt            time.Time
	failureThreshold    int
	cooldownDuration    time.Duration
	halfOpenProbeActive bool
}

// newCircuitBreaker constructs a breaker with the package defaults.
func newCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:            circuitBreakerClosed,
		failureThreshold: defaultFailureThreshold,
		cooldownDuration: defaultCooldownDuration,
	}
}

// NewCircuitBreakerForTest constructs a breaker with the package defaults.
// Exported for tests in the connectors_test package.
func NewCircuitBreakerForTest() *CircuitBreaker {
	return newCircuitBreaker()
}

// SetCooldown overrides the cool-down duration. Exported for tests.
func (cb *CircuitBreaker) SetCooldown(d time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.cooldownDuration = d
}

// State returns the current breaker state. Exported for tests.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return string(cb.state)
}

// Allow reports whether one new attempt may proceed. When closed it
// always allows. When open it allows only if the cooldown has elapsed,
// transitioning to half-open. When half-open it allows a single probe.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitBreakerClosed:
		return true
	case circuitBreakerOpen:
		if time.Since(cb.openedAt) >= cb.cooldownDuration {
			cb.state = circuitBreakerHalfOpen
			cb.halfOpenProbeActive = true
			return true
		}
		return false
	case circuitBreakerHalfOpen:
		if !cb.halfOpenProbeActive {
			cb.halfOpenProbeActive = true
			return true
		}
		return false
	default:
		return true
	}
}

// RecordSuccess resets the breaker to closed and clears the failure count.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = circuitBreakerClosed
	cb.halfOpenProbeActive = false
}

// RecordFailure increments the failure count and opens the breaker when
// the threshold is reached. In half-open, a single failure reopens the
// breaker immediately.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.halfOpenProbeActive = false
	if cb.state == circuitBreakerHalfOpen || cb.failures >= cb.failureThreshold {
		cb.open()
	}
}

func (cb *CircuitBreaker) open() {
	if cb.state == circuitBreakerOpen {
		return // already open — don't reset openedAt (avoids extending cooldown)
	}
	cb.state = circuitBreakerOpen
	cb.openedAt = time.Now()
}

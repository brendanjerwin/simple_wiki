// Package ulid generates ULIDs (Crockford base32, lexicographically sortable
// by creation time) for use as item identifiers in the wiki.
//
// The Generator interface keeps tests deterministic — production uses
// SystemGenerator (cryptographically random entropy + wall-clock time);
// tests inject a Fixed/Sequence generator that returns predictable values.
package ulid

import (
	cryptorand "crypto/rand"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Generator produces ULID strings.
type Generator interface {
	// NewULID returns a freshly generated ULID encoded as a 26-char string.
	NewULID() string
}

// SystemGenerator generates ULIDs using crypto/rand entropy and wall-clock time.
// Safe for concurrent use.
type SystemGenerator struct {
	mu      sync.Mutex
	entropy *ulid.MonotonicEntropy
}

// NewSystemGenerator constructs a SystemGenerator backed by crypto/rand and a
// monotonic-entropy reader. Monotonic entropy ensures two ULIDs generated in
// the same millisecond still sort by creation order.
func NewSystemGenerator() *SystemGenerator {
	return &SystemGenerator{
		entropy: ulid.Monotonic(cryptorand.Reader, 0),
	}
}

// NewULID returns a freshly generated ULID string.
func (g *SystemGenerator) NewULID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), g.entropy).String()
}

// FixedGenerator returns the same ULID for every call. For tests where the
// exact value doesn't matter — when the test only cares that *some* ULID was
// assigned.
type FixedGenerator string

// NewULID returns the fixed value.
func (f FixedGenerator) NewULID() string { return string(f) }

// SequenceGenerator returns the elements of values in order, wrapping around
// on exhaustion. Useful for tests that need to assert specific UID values.
type SequenceGenerator struct {
	mu     sync.Mutex
	values []string
	idx    int
}

// NewSequenceGenerator constructs a SequenceGenerator over the provided
// values.
func NewSequenceGenerator(values ...string) *SequenceGenerator {
	return &SequenceGenerator{values: values}
}

// NewULID returns the next value in the sequence, wrapping after the last.
func (g *SequenceGenerator) NewULID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.values) == 0 {
		return ""
	}
	v := g.values[g.idx%len(g.values)]
	g.idx++
	return v
}

// NewWithEntropy constructs a Generator that draws entropy from the supplied
// io.Reader. Useful for benchmarks and for callers that want to share a
// single entropy source.
func NewWithEntropy(r io.Reader) Generator {
	return &SystemGenerator{
		entropy: ulid.Monotonic(r, 0),
	}
}

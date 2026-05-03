//revive:disable:dot-imports
package server

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OAuthStateStore", func() {
	var (
		ctx   context.Context
		clock *fakeClock
		store *inMemoryOAuthStateStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		clock = &fakeClock{now: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)}
		store = newInMemoryOAuthStateStoreWithClock(clock.Now)
	})

	AfterEach(func() {
		store.Close()
	})

	It("should exist", func() {
		Expect(store).NotTo(BeNil())
	})

	When("issuing a state token", func() {
		var (
			state string
			err   error
		)

		BeforeEach(func() {
			state, err = store.Issue(ctx, "profile-123", "code-verifier-abc")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-empty state value", func() {
			Expect(state).NotTo(BeEmpty())
		})

		It("should return at least 32 base64url-decoded bytes of entropy", func() {
			// base64url encoding: 32 bytes → 43 chars (no padding).
			Expect(len(state)).To(BeNumerically(">=", 43))
		})
	})

	When("issuing two state tokens", func() {
		var s1, s2 string

		BeforeEach(func() {
			var err error
			s1, err = store.Issue(ctx, "profile-1", "verifier-1")
			Expect(err).NotTo(HaveOccurred())
			s2, err = store.Issue(ctx, "profile-1", "verifier-1")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should generate distinct values", func() {
			Expect(s1).NotTo(Equal(s2))
		})
	})

	When("Issue is called with empty profileID", func() {
		var err error

		BeforeEach(func() {
			_, err = store.Issue(ctx, "", "verifier")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("profileID is required")))
		})
	})

	When("Issue is called with empty codeVerifier", func() {
		var err error

		BeforeEach(func() {
			_, err = store.Issue(ctx, "profile-1", "")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("codeVerifier is required")))
		})
	})

	When("consuming a freshly issued state token", func() {
		var (
			state string
			entry OAuthStateEntry
			err   error
		)

		BeforeEach(func() {
			var issueErr error
			state, issueErr = store.Issue(ctx, "profile-xyz", "verifier-xyz")
			Expect(issueErr).NotTo(HaveOccurred())
			entry, err = store.Consume(ctx, state)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the bound profile ID", func() {
			Expect(entry.ProfileID).To(Equal("profile-xyz"))
		})

		It("should return the bound code verifier", func() {
			Expect(entry.CodeVerifier).To(Equal("verifier-xyz"))
		})

		It("should set CodeChallengeMethod to S256", func() {
			Expect(entry.CodeChallengeMethod).To(Equal("S256"))
		})

		It("should set CreatedAt to the issue time", func() {
			Expect(entry.CreatedAt).To(Equal(clock.now))
		})
	})

	When("consuming the same state token twice", func() {
		var (
			state     string
			firstErr  error
			secondErr error
		)

		BeforeEach(func() {
			var issueErr error
			state, issueErr = store.Issue(ctx, "profile-1", "verifier-1")
			Expect(issueErr).NotTo(HaveOccurred())
			_, firstErr = store.Consume(ctx, state)
			_, secondErr = store.Consume(ctx, state)
		})

		It("should succeed the first time", func() {
			Expect(firstErr).NotTo(HaveOccurred())
		})

		It("should return ErrStateNotFound the second time", func() {
			Expect(secondErr).To(MatchError(ErrStateNotFound))
		})
	})

	When("consuming an unknown state token", func() {
		var err error

		BeforeEach(func() {
			_, err = store.Consume(ctx, "never-issued")
		})

		It("should return ErrStateNotFound", func() {
			Expect(err).To(MatchError(ErrStateNotFound))
		})
	})

	When("consuming an empty state token", func() {
		var err error

		BeforeEach(func() {
			_, err = store.Consume(ctx, "")
		})

		It("should return ErrStateNotFound", func() {
			Expect(err).To(MatchError(ErrStateNotFound))
		})
	})

	When("consuming a state token after the TTL has elapsed", func() {
		var (
			state string
			err   error
		)

		BeforeEach(func() {
			var issueErr error
			state, issueErr = store.Issue(ctx, "profile-1", "verifier-1")
			Expect(issueErr).NotTo(HaveOccurred())
			clock.advance(11 * time.Minute) // > 10-minute TTL
			_, err = store.Consume(ctx, state)
		})

		It("should return ErrStateNotFound", func() {
			Expect(err).To(MatchError(ErrStateNotFound))
		})
	})

	When("the GC sweeps expired entries", func() {
		var state string

		BeforeEach(func() {
			var issueErr error
			state, issueErr = store.Issue(ctx, "profile-1", "verifier-1")
			Expect(issueErr).NotTo(HaveOccurred())
			clock.advance(11 * time.Minute)
			store.gcOnce()
		})

		It("should remove the expired entry from the store", func() {
			_, err := store.Consume(ctx, state)
			Expect(err).To(MatchError(ErrStateNotFound))
		})
	})
})

// fakeClock is a controllable time source used by store tests so
// expiry tests run in microseconds rather than minutes.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

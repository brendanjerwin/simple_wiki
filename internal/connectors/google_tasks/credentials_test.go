//revive:disable:dot-imports
package google_tasks_test

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- fakes ---

type fakeTasksFrontmatterRW struct {
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	readErr  error
	writeErr error
}

func newFakeTasksFrontmatterRW() *fakeTasksFrontmatterRW {
	return &fakeTasksFrontmatterRW{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (f *fakeTasksFrontmatterRW) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if f.readErr != nil {
		return id, nil, f.readErr
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeTasksFrontmatterRW) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.pages[id] = fm
	return nil
}

type stubTasksClock struct{ now time.Time }

func (c stubTasksClock) Now() time.Time { return c.now }

// seedTasksCredentials writes a full credential bundle directly into the
// fake's in-memory map, bypassing the write path being tested.
func seedTasksCredentials(rw *fakeTasksFrontmatterRW, profileID wikipage.PageIdentifier, bundle googletasks.CredentialBundle) {
	GinkgoHelper()
	now := time.Now().UTC()
	sub := map[string]any{
		"refresh_token": bundle.RefreshToken,
	}
	if bundle.Email != "" {
		sub["email"] = bundle.Email
	}
	if !bundle.ConnectedAt.IsZero() {
		sub["connected_at"] = bundle.ConnectedAt.UTC().Format(time.RFC3339)
	} else {
		sub["connected_at"] = now.Format(time.RFC3339)
	}
	if !bundle.LastVerifiedAt.IsZero() {
		sub["last_verified_at"] = bundle.LastVerifiedAt.UTC().Format(time.RFC3339)
	} else {
		sub["last_verified_at"] = now.Format(time.RFC3339)
	}
	rw.pages[profileID] = wikipage.FrontMatter{
		"wiki": map[string]any{
			"connectors": map[string]any{
				"google_tasks": sub,
			},
		},
	}
}

// --- NewFrontmatterCredentialStore ----------------------------------------

var _ = Describe("NewFrontmatterCredentialStore (tasks)", func() {
	It("should return error when pages is nil", func() {
		_, err := googletasks.NewFrontmatterCredentialStore(
			nil, googletasks.SystemClock{}, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should return error when clock is nil", func() {
		_, err := googletasks.NewFrontmatterCredentialStore(
			newFakeTasksFrontmatterRW(), nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should return error when logger is nil", func() {
		_, err := googletasks.NewFrontmatterCredentialStore(
			newFakeTasksFrontmatterRW(), googletasks.SystemClock{}, nil, nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should succeed when all required params are provided", func() {
		store, err := googletasks.NewFrontmatterCredentialStore(
			newFakeTasksFrontmatterRW(), googletasks.SystemClock{}, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(store).NotTo(BeNil())
	})
})

// --- FrontmatterCredentialStore.LoadCredentials ---------------------------

var _ = Describe("FrontmatterCredentialStore (tasks)", func() {
	var (
		rw        *fakeTasksFrontmatterRW
		clk       stubTasksClock
		store     *googletasks.FrontmatterCredentialStore
		profileID = wikipage.PageIdentifier("profile_tasksuser_abc12345")
	)

	BeforeEach(func() {
		rw = newFakeTasksFrontmatterRW()
		clk = stubTasksClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
		var err error
		store, err = googletasks.NewFrontmatterCredentialStore(
			rw, clk, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LoadCredentials", func() {
		When("the page does not exist (os.ErrNotExist)", func() {
			var (
				bundle  googletasks.CredentialBundle
				loadErr error
			)

			BeforeEach(func() {
				// rw returns ErrNotExist for unknown pages
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should return an empty bundle", func() {
				Expect(bundle.IsConfigured()).To(BeFalse())
			})
		})

		When("the page read returns a non-NotExist error", func() {
			var loadErr error

			BeforeEach(func() {
				rw.readErr = errors.New("disk I/O failure")
				_, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should return a wrapped error", func() {
				Expect(loadErr).To(HaveOccurred())
				Expect(loadErr.Error()).To(ContainSubstring("disk I/O failure"))
			})
		})

		When("connected_at has an invalid RFC3339 value", func() {
			var loadErr error

			BeforeEach(func() {
				rw.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt-token",
								"connected_at":  "not-a-date",
							},
						},
					},
				}
				_, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should return a parse error", func() {
				Expect(loadErr).To(HaveOccurred())
				Expect(loadErr.Error()).To(ContainSubstring("connected_at"))
			})
		})

		When("credentials are fully populated", func() {
			var (
				bundle  googletasks.CredentialBundle
				loadErr error
			)

			BeforeEach(func() {
				seedTasksCredentials(rw, profileID, googletasks.CredentialBundle{
					Email:        "alice@example.com",
					RefreshToken: "rt-test",
				})
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should populate the Email field", func() {
				Expect(bundle.Email).To(Equal("alice@example.com"))
			})

			It("should populate the RefreshToken field", func() {
				Expect(bundle.RefreshToken).To(Equal("rt-test"))
			})

			It("should report as configured", func() {
				Expect(bundle.IsConfigured()).To(BeTrue())
			})
		})
	})

	Describe("LoadRefreshToken", func() {
		When("no refresh token is present", func() {
			It("should return ErrCredentialMissing", func() {
				_, err := store.LoadRefreshToken(context.Background(), profileID)
				Expect(errors.Is(err, googletasks.ErrCredentialMissing)).To(BeTrue())
			})
		})

		When("a refresh token is present", func() {
			BeforeEach(func() {
				seedTasksCredentials(rw, profileID, googletasks.CredentialBundle{RefreshToken: "rt-live"})
			})

			It("should return the token", func() {
				token, err := store.LoadRefreshToken(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).To(Equal("rt-live"))
			})
		})
	})

	Describe("PersistRefreshToken", func() {
		When("profileID is empty", func() {
			It("should return an error", func() {
				err := store.PersistRefreshToken(context.Background(), "", "", "rt-token")
				Expect(err).To(HaveOccurred())
			})
		})

		When("refreshToken is empty", func() {
			It("should return an error", func() {
				err := store.PersistRefreshToken(context.Background(), string(profileID), "", "")
				Expect(err).To(HaveOccurred())
			})
		})

		When("the write fails", func() {
			It("should return a wrapped error", func() {
				rw.writeErr = errors.New("disk full")
				err := store.PersistRefreshToken(context.Background(), string(profileID), "alice@example.com", "rt-new")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("disk full"))
			})
		})

		When("persisting a fresh connection (no prior credentials)", func() {
			var persistErr error

			BeforeEach(func() {
				persistErr = store.PersistRefreshToken(
					context.Background(), string(profileID), "alice@example.com", "rt-fresh",
				)
			})

			It("should not error", func() {
				Expect(persistErr).NotTo(HaveOccurred())
			})

			It("should write the refresh_token", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.RefreshToken).To(Equal("rt-fresh"))
			})

			It("should stamp ConnectedAt from the clock", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.ConnectedAt).To(Equal(clk.now))
			})

			It("should stamp LastVerifiedAt from the clock", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.LastVerifiedAt).To(Equal(clk.now))
			})
		})

		When("reconnecting with an existing bundle (preserves ConnectedAt)", func() {
			var originalConnectedAt time.Time

			BeforeEach(func() {
				originalConnectedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				seedTasksCredentials(rw, profileID, googletasks.CredentialBundle{
					Email:        "alice@example.com",
					RefreshToken: "rt-old",
					ConnectedAt:  originalConnectedAt,
				})
				Expect(store.PersistRefreshToken(context.Background(), string(profileID), "", "rt-renewed")).To(Succeed())
			})

			It("should preserve the original ConnectedAt", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.ConnectedAt).To(Equal(originalConnectedAt))
			})
		})

		When("resumeAll hook is set", func() {
			It("should invoke the hook after a successful persist", func() {
				hookCalled := false
				rw2 := newFakeTasksFrontmatterRW()
				s2, err := googletasks.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, pid wikipage.PageIdentifier) error {
						hookCalled = true
						Expect(pid).To(Equal(profileID))
						return nil
					},
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(s2.PersistRefreshToken(context.Background(), string(profileID), "", "rt-hook")).To(Succeed())
				Expect(hookCalled).To(BeTrue())
			})

			It("should not propagate a hook error (best-effort)", func() {
				rw2 := newFakeTasksFrontmatterRW()
				s2, err := googletasks.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, _ wikipage.PageIdentifier) error {
						return errors.New("hook failure")
					},
				)
				Expect(err).NotTo(HaveOccurred())

				// PersistRefreshToken should succeed even when the hook fails.
				Expect(s2.PersistRefreshToken(context.Background(), string(profileID), "", "rt-hookfail")).To(Succeed())
			})
		})
	})

	Describe("ClearCredentials", func() {
		When("no credentials exist (page not found)", func() {
			var (
				bundle   googletasks.CredentialBundle
				clearErr error
			)

			BeforeEach(func() {
				bundle, clearErr = store.ClearCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(clearErr).NotTo(HaveOccurred())
			})

			It("should return an empty bundle", func() {
				Expect(bundle.IsConfigured()).To(BeFalse())
			})
		})

		When("credentials exist", func() {
			var (
				bundle   googletasks.CredentialBundle
				clearErr error
			)

			BeforeEach(func() {
				seedTasksCredentials(rw, profileID, googletasks.CredentialBundle{
					Email:        "alice@example.com",
					RefreshToken: "rt-live",
				})
				bundle, clearErr = store.ClearCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(clearErr).NotTo(HaveOccurred())
			})

			It("should wipe the refresh_token in the returned bundle", func() {
				Expect(bundle.RefreshToken).To(BeEmpty())
			})

			It("should report not configured", func() {
				Expect(bundle.IsConfigured()).To(BeFalse())
			})

			It("should persist the cleared state to the page", func() {
				loaded, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(loaded.RefreshToken).To(BeEmpty())
			})
		})

		When("pauseAll hook is set", func() {
			It("should call the hook with auth_failed reason", func() {
				hookCalled := false
				rw2 := newFakeTasksFrontmatterRW()
				seedTasksCredentials(rw2, profileID, googletasks.CredentialBundle{RefreshToken: "rt-live"})

				s2, err := googletasks.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					func(_ context.Context, pid wikipage.PageIdentifier, reason string) error {
						hookCalled = true
						Expect(pid).To(Equal(profileID))
						Expect(reason).To(Equal("auth_failed"))
						return nil
					},
					nil,
				)
				Expect(err).NotTo(HaveOccurred())

				_, clearErr := s2.ClearCredentials(context.Background(), profileID)
				Expect(clearErr).NotTo(HaveOccurred())
				Expect(hookCalled).To(BeTrue())
			})

			It("should not propagate a hook error (best-effort)", func() {
				rw2 := newFakeTasksFrontmatterRW()
				seedTasksCredentials(rw2, profileID, googletasks.CredentialBundle{RefreshToken: "rt-live"})

				s2, err := googletasks.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					func(_ context.Context, _ wikipage.PageIdentifier, _ string) error {
						return errors.New("pause hook failure")
					},
					nil,
				)
				Expect(err).NotTo(HaveOccurred())

				_, clearErr := s2.ClearCredentials(context.Background(), profileID)
				Expect(clearErr).NotTo(HaveOccurred())
			})
		})
	})
})

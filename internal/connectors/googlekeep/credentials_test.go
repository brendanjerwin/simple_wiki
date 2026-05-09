//revive:disable:dot-imports
package googlekeep_test

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- fakes ---

type fakeKeepFrontmatterRW struct {
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	readErr  error
	writeErr error
}

func newFakeKeepFrontmatterRW() *fakeKeepFrontmatterRW {
	return &fakeKeepFrontmatterRW{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (f *fakeKeepFrontmatterRW) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if f.readErr != nil {
		return id, nil, f.readErr
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeKeepFrontmatterRW) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.pages[id] = fm
	return nil
}

type stubKeepClock struct{ now time.Time }

func (c stubKeepClock) Now() time.Time { return c.now }

// seedKeepCredentials writes a credential bundle directly into the fake's
// in-memory map, bypassing the write path being tested.
func seedKeepCredentials(rw *fakeKeepFrontmatterRW, profileID wikipage.PageIdentifier, bundle googlekeep.CredentialBundle) {
	GinkgoHelper()
	now := time.Now().UTC()
	sub := map[string]any{
		"master_token": bundle.MasterToken,
	}
	if bundle.Email != "" {
		sub["email"] = bundle.Email
	}
	if bundle.AndroidID != "" {
		sub["android_id"] = bundle.AndroidID
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
				"google_keep": sub,
			},
		},
	}
}

// --- NewFrontmatterCredentialStore ----------------------------------------

var _ = Describe("NewFrontmatterCredentialStore (keep)", func() {
	It("should return error when pages is nil", func() {
		_, err := googlekeep.NewFrontmatterCredentialStore(
			nil, googlekeep.SystemClock{}, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should return error when clock is nil", func() {
		_, err := googlekeep.NewFrontmatterCredentialStore(
			newFakeKeepFrontmatterRW(), nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should return error when logger is nil", func() {
		_, err := googlekeep.NewFrontmatterCredentialStore(
			newFakeKeepFrontmatterRW(), googlekeep.SystemClock{}, nil, nil, nil,
		)
		Expect(err).To(HaveOccurred())
	})

	It("should succeed when all required params are provided", func() {
		store, err := googlekeep.NewFrontmatterCredentialStore(
			newFakeKeepFrontmatterRW(), googlekeep.SystemClock{}, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(store).NotTo(BeNil())
	})
})

// --- FrontmatterCredentialStore -------------------------------------------

var _ = Describe("FrontmatterCredentialStore (keep)", func() {
	var (
		rw        *fakeKeepFrontmatterRW
		clk       stubKeepClock
		store     *googlekeep.FrontmatterCredentialStore
		profileID = wikipage.PageIdentifier("profile_keepuser_abc12345")
	)

	BeforeEach(func() {
		rw = newFakeKeepFrontmatterRW()
		clk = stubKeepClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
		var err error
		store, err = googlekeep.NewFrontmatterCredentialStore(
			rw, clk, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LoadCredentials", func() {
		When("the page does not exist", func() {
			var (
				bundle  googlekeep.CredentialBundle
				loadErr error
			)

			BeforeEach(func() {
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
							"google_keep": map[string]any{
								"master_token": "mt-token",
								"connected_at": "not-a-date",
							},
						},
					},
				}
				_, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should return a parse error mentioning connected_at", func() {
				Expect(loadErr).To(HaveOccurred())
				Expect(loadErr.Error()).To(ContainSubstring("connected_at"))
			})
		})

		When("credentials are fully populated", func() {
			var (
				bundle  googlekeep.CredentialBundle
				loadErr error
			)

			BeforeEach(func() {
				seedKeepCredentials(rw, profileID, googlekeep.CredentialBundle{
					Email:       "bob@example.com",
					MasterToken: "mt-test",
					AndroidID:   "a1b2c3d4e5f6a7b8",
				})
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should populate the Email field", func() {
				Expect(bundle.Email).To(Equal("bob@example.com"))
			})

			It("should populate the MasterToken field", func() {
				Expect(bundle.MasterToken).To(Equal("mt-test"))
			})

			It("should populate the AndroidID field", func() {
				Expect(bundle.AndroidID).To(Equal("a1b2c3d4e5f6a7b8"))
			})

			It("should report as configured", func() {
				Expect(bundle.IsConfigured()).To(BeTrue())
			})
		})
	})

	Describe("LoadMasterToken", func() {
		When("no master token is present", func() {
			It("should return ErrCredentialMissing", func() {
				_, err := store.LoadMasterToken(context.Background(), profileID)
				Expect(errors.Is(err, googlekeep.ErrCredentialMissing)).To(BeTrue())
			})
		})

		When("a master token is present but android_id is absent", func() {
			var mtb googlekeep.MasterTokenBundle

			BeforeEach(func() {
				seedKeepCredentials(rw, profileID, googlekeep.CredentialBundle{
					Email:       "bob@example.com",
					MasterToken: "mt-live",
					// AndroidID intentionally empty — should be derived.
				})
				var err error
				mtb, err = store.LoadMasterToken(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the master token", func() {
				Expect(mtb.MasterToken).To(Equal("mt-live"))
			})

			It("should synthesize a deterministic 16-char android_id", func() {
				Expect(len(mtb.AndroidID)).To(Equal(16))
			})
		})

		When("a master token and android_id are both present", func() {
			BeforeEach(func() {
				seedKeepCredentials(rw, profileID, googlekeep.CredentialBundle{
					MasterToken: "mt-live",
					AndroidID:   "a1b2c3d4e5f6a7b8",
				})
			})

			It("should return the stored android_id", func() {
				mtb, err := store.LoadMasterToken(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(mtb.AndroidID).To(Equal("a1b2c3d4e5f6a7b8"))
			})
		})
	})

	Describe("PersistMasterToken", func() {
		When("profileID is empty", func() {
			It("should return an error", func() {
				err := store.PersistMasterToken(context.Background(), "", "mt-token", "", "bob@example.com")
				Expect(err).To(HaveOccurred())
			})
		})

		When("masterToken is empty", func() {
			It("should return an error", func() {
				err := store.PersistMasterToken(context.Background(), profileID, "", "", "bob@example.com")
				Expect(err).To(HaveOccurred())
			})
		})

		When("the write fails", func() {
			It("should return a wrapped error", func() {
				rw.writeErr = errors.New("disk full")
				err := store.PersistMasterToken(context.Background(), profileID, "mt-new", "a1b2c3d4e5f6a7b8", "bob@example.com")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("disk full"))
			})
		})

		When("persisting a fresh connection", func() {
			var persistErr error

			BeforeEach(func() {
				persistErr = store.PersistMasterToken(
					context.Background(), profileID, "mt-fresh", "a1b2c3d4e5f6a7b8", "bob@example.com",
				)
			})

			It("should not error", func() {
				Expect(persistErr).NotTo(HaveOccurred())
			})

			It("should write the master_token", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.MasterToken).To(Equal("mt-fresh"))
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

		When("android_id is empty — should derive from profileID", func() {
			It("should synthesize a 16-char android_id", func() {
				Expect(store.PersistMasterToken(context.Background(), profileID, "mt-derived", "", "")).To(Succeed())
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(bundle.AndroidID)).To(Equal(16))
			})
		})

		When("resumeAll hook is set", func() {
			It("should invoke the hook after a successful persist", func() {
				hookCalled := false
				rw2 := newFakeKeepFrontmatterRW()
				s2, err := googlekeep.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, pid wikipage.PageIdentifier) error {
						hookCalled = true
						Expect(pid).To(Equal(profileID))
						return nil
					},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(s2.PersistMasterToken(context.Background(), profileID, "mt-hook", "", "")).To(Succeed())
				Expect(hookCalled).To(BeTrue())
			})

			It("should not propagate a hook error", func() {
				rw2 := newFakeKeepFrontmatterRW()
				s2, err := googlekeep.NewFrontmatterCredentialStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, _ wikipage.PageIdentifier) error {
						return errors.New("hook failure")
					},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(s2.PersistMasterToken(context.Background(), profileID, "mt-hookfail", "", "")).To(Succeed())
			})
		})
	})

	Describe("ClearCredentials", func() {
		When("no credentials exist", func() {
			var (
				bundle   googlekeep.CredentialBundle
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
				bundle   googlekeep.CredentialBundle
				clearErr error
			)

			BeforeEach(func() {
				seedKeepCredentials(rw, profileID, googlekeep.CredentialBundle{
					Email:       "bob@example.com",
					MasterToken: "mt-live",
					AndroidID:   "a1b2c3d4e5f6a7b8",
				})
				bundle, clearErr = store.ClearCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(clearErr).NotTo(HaveOccurred())
			})

			It("should wipe the master_token", func() {
				Expect(bundle.MasterToken).To(BeEmpty())
			})

			It("should preserve the android_id", func() {
				Expect(bundle.AndroidID).To(Equal("a1b2c3d4e5f6a7b8"))
			})

			It("should report not configured", func() {
				Expect(bundle.IsConfigured()).To(BeFalse())
			})

			It("should persist the cleared state", func() {
				loaded, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(loaded.MasterToken).To(BeEmpty())
			})
		})

		When("pauseAll hook is set", func() {
			It("should call the hook with auth_failed reason", func() {
				hookCalled := false
				rw2 := newFakeKeepFrontmatterRW()
				seedKeepCredentials(rw2, profileID, googlekeep.CredentialBundle{MasterToken: "mt-live"})

				s2, err := googlekeep.NewFrontmatterCredentialStore(
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

			It("should not propagate a hook error", func() {
				rw2 := newFakeKeepFrontmatterRW()
				seedKeepCredentials(rw2, profileID, googlekeep.CredentialBundle{MasterToken: "mt-live"})

				s2, err := googlekeep.NewFrontmatterCredentialStore(
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

	Describe("Connect", func() {
		var fakeVerifier *stubAuthVerifier

		BeforeEach(func() {
			fakeVerifier = &stubAuthVerifier{masterToken: "mt-connected"}
		})

		When("email is empty", func() {
			It("should return an error", func() {
				_, err := store.Connect(context.Background(), profileID, "", "oauth-token", fakeVerifier)
				Expect(err).To(HaveOccurred())
			})
		})

		When("oauthToken is empty", func() {
			It("should return an error", func() {
				_, err := store.Connect(context.Background(), profileID, "bob@example.com", "", fakeVerifier)
				Expect(err).To(HaveOccurred())
			})
		})

		When("verifier is nil", func() {
			It("should return an error", func() {
				_, err := store.Connect(context.Background(), profileID, "bob@example.com", "oauth-token", nil)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the verifier returns an error", func() {
			It("should propagate the error without writing state", func() {
				fakeVerifier.err = errors.New("oauth exchange failed")
				_, err := store.Connect(context.Background(), profileID, "bob@example.com", "oauth-token", fakeVerifier)
				Expect(err).To(MatchError("oauth exchange failed"))
				// No credentials should be written.
				_, ok := rw.pages[profileID]
				Expect(ok).To(BeFalse())
			})
		})

		When("the verifier succeeds", func() {
			var (
				bundle     googlekeep.CredentialBundle
				connectErr error
			)

			BeforeEach(func() {
				bundle, connectErr = store.Connect(
					context.Background(), profileID, "bob@example.com", "oauth-token", fakeVerifier,
				)
			})

			It("should not error", func() {
				Expect(connectErr).NotTo(HaveOccurred())
			})

			It("should return a configured bundle", func() {
				Expect(bundle.IsConfigured()).To(BeTrue())
			})

			It("should persist the master_token", func() {
				Expect(bundle.MasterToken).To(Equal("mt-connected"))
			})

			It("should persist the email", func() {
				Expect(bundle.Email).To(Equal("bob@example.com"))
			})
		})
	})
})

// stubAuthVerifier satisfies googlekeep.AuthVerifier.
type stubAuthVerifier struct {
	masterToken string
	err         error
}

func (s *stubAuthVerifier) VerifyOAuthToken(_ context.Context, _ wikipage.PageIdentifier, _, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.masterToken, nil
}

var _ googlekeep.AuthVerifier = (*stubAuthVerifier)(nil)

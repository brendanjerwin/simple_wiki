//revive:disable:dot-imports
package frontmattercreds_test

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/frontmattercreds"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	testTokenKey = "token"
	testEmailKey = "email"
)

// --- test bundle & codec ---

// testBundle is a minimal credential bundle for testing the Store.
type testBundle struct {
	Email          string
	Token          string
	ConnectedAt    time.Time
	LastVerifiedAt time.Time
}

func (b testBundle) isConfigured() bool { return b.Token != "" }

// testCodec implements frontmattercreds.BundleCodec[testBundle].
type testCodec struct{}

func (testCodec) ConnectorKey() string { return "test_connector" }
func (testCodec) ErrorPrefix() string  { return "test_connector" }

func (testCodec) AssembleBundle(m map[string]any, connectedAt, lastVerifiedAt time.Time) testBundle {
	return testBundle{
		Email:          frontmattercreds.GetStringFromMap(m, testEmailKey),
		Token:          frontmattercreds.GetStringFromMap(m, testTokenKey),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
	}
}

func (testCodec) WriteTokenFields(b testBundle, m map[string]any) {
	if b.Email != "" {
		m[testEmailKey] = b.Email
	} else {
		delete(m, testEmailKey)
	}

	if b.Token != "" {
		m[testTokenKey] = b.Token
	} else {
		delete(m, testTokenKey)
	}
}

func (testCodec) Timestamps(b testBundle) (connectedAt, lastVerifiedAt time.Time) {
	return b.ConnectedAt, b.LastVerifiedAt
}

func (testCodec) IsConfigured(b testBundle) bool { return b.Token != "" }

func (testCodec) ClearForDisconnect(b *testBundle) {
	b.Token = ""
	b.LastVerifiedAt = time.Time{}
}

// --- fake FrontmatterReadWriter ---

type fakeFrontmatterRW struct {
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	readErr  error
	writeErr error
}

func newFakeFrontmatterRW() *fakeFrontmatterRW {
	return &fakeFrontmatterRW{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (f *fakeFrontmatterRW) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if f.readErr != nil {
		return id, nil, f.readErr
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeFrontmatterRW) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter, _ wikipage.Identity) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.pages[id] = fm
	return nil
}

// stubClock is a deterministic clock for testing.
type stubClock struct{ now time.Time }

func (c stubClock) Now() time.Time { return c.now }

// seedCredentials plants a fully-formed frontmatter entry for the test connector,
// bypassing the write path under test.
func seedCredentials(rw *fakeFrontmatterRW, profileID wikipage.PageIdentifier, bundle testBundle) {
	GinkgoHelper()
	now := time.Now().UTC()
	sub := map[string]any{
		testTokenKey: bundle.Token,
	}
	if bundle.Email != "" {
		sub[testEmailKey] = bundle.Email
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
				"test_connector": sub,
			},
		},
	}
}

// --- NewStore ---

var _ = Describe("NewStore", func() {
	var codec testCodec

	It("should error when pages is nil", func() {
		_, err := frontmattercreds.NewStore(
			nil, stubClock{now: time.Now()}, lumber.NewConsoleLogger(lumber.WARN), nil, nil, codec,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("pages"))
	})

	It("should error when clock is nil", func() {
		_, err := frontmattercreds.NewStore(
			newFakeFrontmatterRW(), nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, codec,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("clock"))
	})

	It("should error when logger is nil", func() {
		_, err := frontmattercreds.NewStore(
			newFakeFrontmatterRW(), stubClock{now: time.Now()}, nil, nil, nil, codec,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("logger"))
	})

	It("should succeed with all required params", func() {
		store, err := frontmattercreds.NewStore(
			newFakeFrontmatterRW(), stubClock{now: time.Now()}, lumber.NewConsoleLogger(lumber.WARN), nil, nil, codec,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(store).NotTo(BeNil())
	})
})

// --- Store ---

var _ = Describe("Store", func() {
	var (
		rw        *fakeFrontmatterRW
		clk       stubClock
		store     *frontmattercreds.Store[testBundle]
		profileID = wikipage.PageIdentifier("profile_test_abc12345")
	)

	BeforeEach(func() {
		rw = newFakeFrontmatterRW()
		clk = stubClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
		var err error
		store, err = frontmattercreds.NewStore(
			rw, clk, lumber.NewConsoleLogger(lumber.WARN), nil, nil, testCodec{},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Clock", func() {
		It("should return the injected clock", func() {
			Expect(store.Clock()).To(Equal(clk))
		})
	})

	Describe("LoadCredentials", func() {
		When("the page does not exist", func() {
			var (
				bundle  testBundle
				loadErr error
			)

			BeforeEach(func() {
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should return a zero bundle", func() {
				Expect(bundle.isConfigured()).To(BeFalse())
			})
		})

		When("the page read returns a non-NotExist error", func() {
			var loadErr error

			BeforeEach(func() {
				rw.readErr = errors.New("disk I/O failure")
				_, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should return a wrapped error containing the original message", func() {
				Expect(loadErr).To(HaveOccurred())
				Expect(loadErr.Error()).To(ContainSubstring("disk I/O failure"))
			})
		})

		When("the connector subtree is absent", func() {
			var (
				bundle  testBundle
				loadErr error
			)

			BeforeEach(func() {
				rw.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{},
					},
				}
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should return a zero bundle", func() {
				Expect(bundle.isConfigured()).To(BeFalse())
			})
		})

		When("connected_at has an invalid RFC3339 value", func() {
			var loadErr error

			BeforeEach(func() {
				rw.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"test_connector": map[string]any{
								testTokenKey:   "tok",
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

		When("last_verified_at has an invalid RFC3339 value", func() {
			var loadErr error

			BeforeEach(func() {
				rw.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"test_connector": map[string]any{
								testTokenKey:       "tok",
								"last_verified_at": "not-a-date",
							},
						},
					},
				}
				_, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should return a parse error mentioning last_verified_at", func() {
				Expect(loadErr).To(HaveOccurred())
				Expect(loadErr.Error()).To(ContainSubstring("last_verified_at"))
			})
		})

		When("credentials are fully seeded", func() {
			var (
				bundle  testBundle
				loadErr error
			)

			BeforeEach(func() {
				seedCredentials(rw, profileID, testBundle{
					Email: "user@example.com",
					Token: "tok-live",
				})
				bundle, loadErr = store.LoadCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should populate the Email field", func() {
				Expect(bundle.Email).To(Equal("user@example.com"))
			})

			It("should populate the Token field", func() {
				Expect(bundle.Token).To(Equal("tok-live"))
			})

			It("should report as configured", func() {
				Expect(bundle.isConfigured()).To(BeTrue())
			})
		})
	})

	Describe("WriteCredentials", func() {
		When("writing to a fresh profile", func() {
			var writeErr error

			BeforeEach(func() {
				writeErr = store.WriteCredentials(profileID, testBundle{
					Email:          "user@example.com",
					Token:          "tok-written",
					ConnectedAt:    clk.now,
					LastVerifiedAt: clk.now,
				})
			})

			It("should not error", func() {
				Expect(writeErr).NotTo(HaveOccurred())
			})

			It("should persist the token so it can be loaded back", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.Token).To(Equal("tok-written"))
			})

			It("should persist the email so it can be loaded back", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.Email).To(Equal("user@example.com"))
			})

			It("should persist ConnectedAt so it can be loaded back", func() {
				bundle, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundle.ConnectedAt).To(Equal(clk.now))
			})
		})

		When("write fails", func() {
			It("should return a wrapped error", func() {
				rw.writeErr = errors.New("disk full")
				err := store.WriteCredentials(profileID, testBundle{Token: "tok"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("disk full"))
			})
		})

		When("a zero ConnectedAt is written", func() {
			It("should delete connected_at from frontmatter", func() {
				Expect(store.WriteCredentials(profileID, testBundle{Token: "tok"})).To(Succeed())
				fm := rw.pages[profileID]
				wiki, ok := fm["wiki"].(map[string]any)
				Expect(ok).To(BeTrue())
				conns, ok := wiki["connectors"].(map[string]any)
				Expect(ok).To(BeTrue())
				connector, ok := conns["test_connector"].(map[string]any)
				Expect(ok).To(BeTrue())
				_, exists := connector["connected_at"]
				Expect(exists).To(BeFalse())
			})
		})

		When("preserving unrelated connector subtrees", func() {
			It("should leave other connector entries untouched", func() {
				rw.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"other_connector": map[string]any{"key": "value"},
						},
					},
				}
				Expect(store.WriteCredentials(profileID, testBundle{Token: "tok"})).To(Succeed())
				fm := rw.pages[profileID]
				wiki, ok := fm["wiki"].(map[string]any)
				Expect(ok).To(BeTrue())
				conns, ok := wiki["connectors"].(map[string]any)
				Expect(ok).To(BeTrue())
				other, ok := conns["other_connector"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(other["key"]).To(Equal("value"))
			})
		})
	})

	Describe("ResumeBindings", func() {
		When("resumeAll hook is nil", func() {
			It("should not panic", func() {
				Expect(func() {
					store.ResumeBindings(context.Background(), profileID)
				}).NotTo(Panic())
			})
		})

		When("resumeAll hook is set", func() {
			It("should invoke the hook with the profile ID", func() {
				hookCalled := false
				s2, err := frontmattercreds.NewStore(
					rw, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, pid wikipage.PageIdentifier) error {
						hookCalled = true
						Expect(pid).To(Equal(profileID))
						return nil
					},
					testCodec{},
				)
				Expect(err).NotTo(HaveOccurred())
				s2.ResumeBindings(context.Background(), profileID)
				Expect(hookCalled).To(BeTrue())
			})

			It("should not propagate a hook error", func() {
				s2, err := frontmattercreds.NewStore(
					rw, clk, lumber.NewConsoleLogger(lumber.WARN),
					nil,
					func(_ context.Context, _ wikipage.PageIdentifier) error {
						return errors.New("hook failure")
					},
					testCodec{},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(func() {
					s2.ResumeBindings(context.Background(), profileID)
				}).NotTo(Panic())
			})
		})
	})

	Describe("ClearCredentials", func() {
		When("no credentials exist", func() {
			var (
				bundle   testBundle
				clearErr error
			)

			BeforeEach(func() {
				bundle, clearErr = store.ClearCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(clearErr).NotTo(HaveOccurred())
			})

			It("should return a zero bundle", func() {
				Expect(bundle.isConfigured()).To(BeFalse())
			})
		})

		When("credentials exist", func() {
			var (
				bundle   testBundle
				clearErr error
			)

			BeforeEach(func() {
				seedCredentials(rw, profileID, testBundle{
					Email: "user@example.com",
					Token: "tok-live",
				})
				bundle, clearErr = store.ClearCredentials(context.Background(), profileID)
			})

			It("should not error", func() {
				Expect(clearErr).NotTo(HaveOccurred())
			})

			It("should wipe the token in the returned bundle", func() {
				Expect(bundle.Token).To(BeEmpty())
			})

			It("should report not configured", func() {
				Expect(bundle.isConfigured()).To(BeFalse())
			})

			It("should preserve email in the returned bundle", func() {
				Expect(bundle.Email).To(Equal("user@example.com"))
			})

			It("should persist the cleared state", func() {
				loaded, err := store.LoadCredentials(context.Background(), profileID)
				Expect(err).NotTo(HaveOccurred())
				Expect(loaded.Token).To(BeEmpty())
			})
		})

		When("the write fails", func() {
			It("should return a wrapped error", func() {
				seedCredentials(rw, profileID, testBundle{Token: "tok-live"})
				rw.writeErr = errors.New("disk full")
				_, err := store.ClearCredentials(context.Background(), profileID)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("disk full"))
			})
		})

		When("pauseAll hook is set", func() {
			It("should call the hook with auth_failed reason", func() {
				hookCalled := false
				rw2 := newFakeFrontmatterRW()
				seedCredentials(rw2, profileID, testBundle{Token: "tok-live"})

				s2, err := frontmattercreds.NewStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					func(_ context.Context, pid wikipage.PageIdentifier, reason string) error {
						hookCalled = true
						Expect(pid).To(Equal(profileID))
						Expect(reason).To(Equal(frontmattercreds.PausedReasonAuthFailed))
						return nil
					},
					nil,
					testCodec{},
				)
				Expect(err).NotTo(HaveOccurred())

				_, clearErr := s2.ClearCredentials(context.Background(), profileID)
				Expect(clearErr).NotTo(HaveOccurred())
				Expect(hookCalled).To(BeTrue())
			})

			It("should not propagate a hook error", func() {
				rw2 := newFakeFrontmatterRW()
				seedCredentials(rw2, profileID, testBundle{Token: "tok-live"})

				s2, err := frontmattercreds.NewStore(
					rw2, clk, lumber.NewConsoleLogger(lumber.WARN),
					func(_ context.Context, _ wikipage.PageIdentifier, _ string) error {
						return errors.New("pause hook failure")
					},
					nil,
					testCodec{},
				)
				Expect(err).NotTo(HaveOccurred())

				_, clearErr := s2.ClearCredentials(context.Background(), profileID)
				Expect(clearErr).NotTo(HaveOccurred())
			})
		})
	})
})

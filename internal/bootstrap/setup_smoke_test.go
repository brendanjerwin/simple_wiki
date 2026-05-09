//revive:disable:dot-imports
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
)

// stubSite returns a minimal *server.Site for bootstrap smoke tests.
// FrontmatterIndexQueryer and JobQueueCoordinator are populated; all
// other fields are zero-values (acceptable because setup functions
// only store the site reference during construction without calling
// PageReaderMutator methods).
func stubSite(logger *lumber.ConsoleLogger) *server.Site {
	return &server.Site{
		FrontmatterIndexQueryer: &fakeFrontmatterIndex{},
		JobQueueCoordinator:     jobs.NewJobQueueCoordinator(logger),
	}
}

// buildScheduler creates a SyncScheduler backed by the provided site's
// job queue coordinator.
func buildScheduler(site *server.Site, logger *lumber.ConsoleLogger) *connectors.SyncScheduler {
	scheduler, err := connectors.NewSyncScheduler(site.JobQueueCoordinator, logger)
	if err != nil {
		panic("buildScheduler: " + err.Error())
	}
	return scheduler
}

// --- setupGoogleTasks -------------------------------------------------

var _ = Describe("setupGoogleTasks", func() {
	var logger = lumber.NewConsoleLogger(lumber.WARN)

	When("the required env vars are not configured", func() {
		var (
			wiring *tasksWiring
			err    error
		)

		BeforeEach(func() {
			// Ensure env vars are absent for this test.
			_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID")
			_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET")
			_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI")

			site := stubSite(logger)
			scheduler := buildScheduler(site, logger)
			mutator := checklistmutator.New(site, systemWallClock{}, ulid.NewSystemGenerator())
			leaseTable := connectors.NewLeaseTable()
			wiring, err = setupGoogleTasks(site, scheduler, mutator, leaseTable, logger)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil wiring (connector disabled)", func() {
			Expect(wiring).To(BeNil())
		})
	})

	When("the required env vars are configured", func() {
		var (
			wiring *tasksWiring
			err    error
		)

		BeforeEach(func() {
			DeferCleanup(func() {
				_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID")
				_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET")
				_ = os.Unsetenv("SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI")
			})
			Expect(os.Setenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID", "smoke-test-client-id")).To(Succeed())
			Expect(os.Setenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET", "smoke-test-client-secret")).To(Succeed())
			Expect(os.Setenv("SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI", "https://example.com/oauth/callback")).To(Succeed())

			site := stubSite(logger)
			scheduler := buildScheduler(site, logger)
			mutator := checklistmutator.New(site, systemWallClock{}, ulid.NewSystemGenerator())
			leaseTable := connectors.NewLeaseTable()
			wiring, err = setupGoogleTasks(site, scheduler, mutator, leaseTable, logger)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return non-nil wiring", func() {
			Expect(wiring).NotTo(BeNil())
		})

		It("should have a non-nil engine", func() {
			Expect(wiring.engine).NotTo(BeNil())
		})

		It("should have a non-nil adapter", func() {
			Expect(wiring.adapter).NotTo(BeNil())
		})

		It("should have a non-nil binding store", func() {
			Expect(wiring.bindingStore).NotTo(BeNil())
		})

		It("should have a non-nil credential store", func() {
			Expect(wiring.credentialStore).NotTo(BeNil())
		})

		It("should have a non-nil auth URL builder", func() {
			Expect(wiring.authURLBuilder).NotTo(BeNil())
		})
	})
})

// --- setupGoogleKeep -------------------------------------------------

var _ = Describe("setupGoogleKeep", func() {
	var logger = lumber.NewConsoleLogger(lumber.WARN)

	When("called with valid dependencies", func() {
		var (
			wiring *keepWiring
			err    error
		)

		BeforeEach(func() {
			site := stubSite(logger)
			scheduler := buildScheduler(site, logger)
			mutator := checklistmutator.New(site, systemWallClock{}, ulid.NewSystemGenerator())
			leaseTable := connectors.NewLeaseTable()
			wiring, err = setupGoogleKeep(site, scheduler, mutator, leaseTable, logger)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return non-nil wiring", func() {
			Expect(wiring).NotTo(BeNil())
		})

		It("should have a non-nil engine", func() {
			Expect(wiring.engine).NotTo(BeNil())
		})

		It("should have a non-nil adapter", func() {
			Expect(wiring.adapter).NotTo(BeNil())
		})

		It("should have a non-nil binding store", func() {
			Expect(wiring.bindingStore).NotTo(BeNil())
		})

		It("should have a non-nil credential store", func() {
			Expect(wiring.credentialStore).NotTo(BeNil())
		})

		It("should have a non-nil auth verifier", func() {
			Expect(wiring.authVerifier).NotTo(BeNil())
		})
	})
})

// --- keepAuthVerifierImpl.VerifyOAuthToken ----------------------------

// errTransport is an http.RoundTripper that always returns a network error.
type errTransport struct{ err error }

func (e errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

// allToServerTransport redirects every outbound HTTP request to the
// provided test-server address, stripping the original host/scheme so
// the test server can route by path.
type allToServerTransport struct {
	targetHost string // e.g. "127.0.0.1:PORT" from httptest.Server.Listener.Addr()
}

func (t *allToServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	parsed, _ := url.Parse("http://" + t.targetHost)
	newReq.URL.Scheme = parsed.Scheme
	newReq.URL.Host = parsed.Host
	return http.DefaultTransport.RoundTrip(newReq)
}

var _ = Describe("keepAuthVerifierImpl.VerifyOAuthToken", func() {
	var (
		logger = lumber.NewConsoleLogger(lumber.WARN)
		ctx    = context.Background()
	)

	When("the auth HTTP client returns a network error", func() {
		var (
			masterToken string
			err         error
		)

		BeforeEach(func() {
			verifier := &keepAuthVerifierImpl{
				httpClient:     &http.Client{},
				authHTTPClient: &http.Client{Transport: errTransport{err: errors.New("simulated network failure")}},
				debug:          logger,
			}
			masterToken, err = verifier.VerifyOAuthToken(ctx, "", "user@example.com", "oauth-token", "android-id")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty master token", func() {
			Expect(masterToken).To(BeEmpty())
		})
	})

	When("the master-token exchange succeeds but the bearer exchange fails", func() {
		var (
			masterToken string
			err         error
		)

		BeforeEach(func() {
			var mu sync.Mutex
			requestCount := 0

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requestCount++
				n := requestCount
				mu.Unlock()

				w.WriteHeader(http.StatusOK)
				if n == 1 {
					// ExchangeOAuthTokenForMasterToken: return a master token.
					_, _ = fmt.Fprint(w, "Token=fake-master-token\n")
				} else {
					// ExchangeMasterTokenForBearer: return BadAuthentication.
					_, _ = fmt.Fprint(w, "Error=BadAuthentication\n")
				}
			}))
			DeferCleanup(srv.Close)

			transport := &allToServerTransport{targetHost: srv.Listener.Addr().String()}
			authClient := &http.Client{Transport: transport}

			verifier := &keepAuthVerifierImpl{
				httpClient:     &http.Client{},
				authHTTPClient: authClient,
				debug:          nil,
			}
			masterToken, err = verifier.VerifyOAuthToken(ctx, "", "user@example.com", "oauth-token", "android-id")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty master token", func() {
			Expect(masterToken).To(BeEmpty())
		})
	})

	When("both auth exchanges succeed but the Changes verification call fails", func() {
		var (
			masterToken string
			err         error
		)

		BeforeEach(func() {
			var mu sync.Mutex
			requestCount := 0

			// Single test server handles all three calls:
			//   1. ExchangeOAuthTokenForMasterToken → Token=...
			//   2. ExchangeMasterTokenForBearer → Auth=...
			//   3. client.Changes (path ends in "changes") → 401
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requestCount++
				n := requestCount
				mu.Unlock()

				// Keep Changes call is a POST to .../changes
				if strings.HasSuffix(r.URL.Path, "changes") {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = fmt.Fprint(w, `{"error":{"code":401,"message":"Unauthorized"}}`)
					return
				}
				w.WriteHeader(http.StatusOK)
				switch n {
				case 1:
					_, _ = fmt.Fprint(w, "Token=fake-master-token\n")
				default:
					_, _ = fmt.Fprint(w, "Auth=fake-bearer-token\n")
				}
			}))
			DeferCleanup(srv.Close)

			transport := &allToServerTransport{targetHost: srv.Listener.Addr().String()}
			authClient := &http.Client{Transport: transport}
			keepClient := &http.Client{Transport: transport}

			verifier := &keepAuthVerifierImpl{
				httpClient:     keepClient,
				authHTTPClient: authClient,
				debug:          logger, // exercise the SetDebugLogger branch
			}
			masterToken, err = verifier.VerifyOAuthToken(ctx, "", "user@example.com", "oauth-token", "android-id")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty master token", func() {
			Expect(masterToken).To(BeEmpty())
		})
	})
})

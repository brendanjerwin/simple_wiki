//revive:disable:dot-imports
package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeRefreshTokenPersister captures Persist calls for assertion.
type fakeRefreshTokenPersister struct {
	mu        sync.Mutex
	called    bool
	profileID string
	email     string
	token     string
	err       error
}

func (f *fakeRefreshTokenPersister) PersistRefreshToken(_ context.Context, profileID, email, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called = true
	f.profileID = profileID
	f.email = email
	f.token = token
	return f.err
}

// fakeIdentityResolver returns a fixed profile ID (or error) for the
// "Try again" path.
type fakeIdentityResolver struct {
	profileID string
	err       error
}

func (f *fakeIdentityResolver) ResolveProfileID(_ *http.Request) (string, error) {
	return f.profileID, f.err
}

// fakeAuthURLIssuer returns a fixed URL.
type fakeAuthURLIssuer struct {
	url string
	err error
}

func (f *fakeAuthURLIssuer) BuildAuthURL(_ context.Context, _ string) (string, error) {
	return f.url, f.err
}

// makeTokenServer returns an httptest.Server that mimics Google's
// token endpoint. The test injects a response handler describing the
// desired status + body.
func makeTokenServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

var _ = Describe("OAuthGoogleHandler", func() {
	var (
		ctx          context.Context
		clock        *fakeClock
		stateStore   *inMemoryOAuthStateStore
		persister    *fakeRefreshTokenPersister
		identity     *fakeIdentityResolver
		authIssuer   *fakeAuthURLIssuer
		tokenServer  *httptest.Server
		handler      *OAuthGoogleHandler
		recorder     *httptest.ResponseRecorder
		issuedState  string
		tokenHandler http.HandlerFunc
		// Captured request bytes for assertions on the exchange.
		capturedForm url.Values
	)

	BeforeEach(func() {
		ctx = context.Background()
		_ = ctx
		clock = &fakeClock{now: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)}
		stateStore = newInMemoryOAuthStateStoreWithClock(clock.Now)
		persister = &fakeRefreshTokenPersister{}
		identity = &fakeIdentityResolver{profileID: "user@example.com"}
		authIssuer = &fakeAuthURLIssuer{url: "https://accounts.google.com/o/oauth2/v2/auth?fresh=1"}
		recorder = httptest.NewRecorder()
		capturedForm = nil

		// Default token-server response: success with the expected
		// scope and a refresh token. Per-test BeforeEach can replace
		// tokenHandler before the server is started.
		tokenHandler = func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			parsed, _ := url.ParseQuery(string(body))
			capturedForm = parsed
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "ya29.access",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "1//refresh-token",
				Scope:        "https://www.googleapis.com/auth/tasks",
			})
		}
	})

	AfterEach(func() {
		stateStore.Close()
		if tokenServer != nil {
			tokenServer.Close()
			tokenServer = nil
		}
	})

	startServer := func() {
		tokenServer = makeTokenServer(func(w http.ResponseWriter, r *http.Request) {
			tokenHandler(w, r)
		})
		handler = &OAuthGoogleHandler{
			StateStore:       stateStore,
			TokenPersister:   persister,
			IdentityResolver: identity,
			AuthURLIssuer:    authIssuer,
			HTTPClient:       tokenServer.Client(),
			ClientID:         "client-id",
			ClientSecret:     "client-secret",
			RedirectURI:      "https://wiki.example/oauth/google/callback",
			IssuerExpected:   googleOAuthIssuer,
			TokenEndpoint:    tokenServer.URL,
			RequiredScope:    googleTasksScope,
		}
	}

	issueState := func() string {
		s, err := stateStore.Issue(ctx, "user@example.com", "verifier-abc")
		Expect(err).NotTo(HaveOccurred())
		return s
	}

	makeRequest := func(rawQuery string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/oauth/google/callback?"+rawQuery, nil)
		return req
	}

	Describe("when iss param is missing", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should reject with 400 (RFC 9207 strict policy)", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("should NOT call the token endpoint", func() {
			Expect(capturedForm).To(BeNil())
		})

		It("should NOT persist a refresh token", func() {
			Expect(persister.called).To(BeFalse())
		})

		It("should NOT consume the state token (rejected before state lookup)", func() {
			_, err := stateStore.Consume(ctx, issuedState)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should render a wiki-styled error page", func() {
			Expect(recorder.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(recorder.Body.String()).To(ContainSubstring("Authorization server identification missing"))
		})
	})

	Describe("when iss param is set to a wrong issuer", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{"https://evil.example.com"},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should reject with 400", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("should NOT call the token endpoint", func() {
			Expect(capturedForm).To(BeNil())
		})

		It("should NOT persist a refresh token", func() {
			Expect(persister.called).To(BeFalse())
		})

		It("should NOT consume the state token", func() {
			// State should still be valid for a second attempt.
			_, err := stateStore.Consume(ctx, issuedState)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should render an HTML error page", func() {
			Expect(recorder.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(recorder.Body.String()).To(ContainSubstring("Authorization server mismatch"))
		})
	})

	Describe("when iss param matches the pinned issuer", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should redirect to the success page", func() {
			Expect(recorder.Code).To(Equal(http.StatusFound))
		})

		It("should persist the refresh token", func() {
			Expect(persister.called).To(BeTrue())
		})
	})

	Describe("when the state token is unknown", func() {
		BeforeEach(func() {
			startServer()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{"never-issued"},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render an error page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(ContainSubstring("expired"))
		})

		It("should NOT call the token endpoint", func() {
			Expect(capturedForm).To(BeNil())
		})
	})

	Describe("when the state token has expired", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			clock.advance(11 * time.Minute) // > TTL
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render the same expired-session page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(ContainSubstring("expired"))
		})
	})

	Describe("when the state token is replayed", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))

			// Reset and replay the same state.
			recorder = httptest.NewRecorder()
			capturedForm = nil
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should reject the replay", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("should NOT call the token endpoint on the replay", func() {
			Expect(capturedForm).To(BeNil())
		})
	})

	Describe("when the code parameter is missing", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render an error page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(ContainSubstring("Authorization code missing"))
		})

		It("should consume the state token", func() {
			// State must be consumed even if code is missing,
			// so a second call with the same state fails.
			_, err := stateStore.Consume(ctx, issuedState)
			Expect(err).To(MatchError(ErrStateNotFound))
		})
	})

	Describe("when the user denied consent", func() {
		BeforeEach(func() {
			startServer()
			query := url.Values{
				"error":             []string{"access_denied"},
				"error_description": []string{"user denied"},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render the access-denied page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(ContainSubstring("Authorization was not granted"))
			Expect(recorder.Body.String()).To(ContainSubstring("access_denied"))
		})

		It("should NOT call the token endpoint", func() {
			Expect(capturedForm).To(BeNil())
		})
	})

	Describe("when Google rejects the code (invalid_grant)", func() {
		BeforeEach(func() {
			tokenHandler = func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				parsed, _ := url.ParseQuery(string(body))
				capturedForm = parsed
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(tokenErrorResponse{
					Error:            "invalid_grant",
					ErrorDescription: "Bad code verifier",
				})
			}
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render the bad-gateway page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadGateway))
		})

		It("should mention the underlying error", func() {
			Expect(recorder.Body.String()).To(ContainSubstring("invalid_grant"))
		})

		It("should send code_verifier in the exchange", func() {
			Expect(capturedForm.Get("code_verifier")).To(Equal("verifier-abc"))
		})

		It("should send grant_type=authorization_code", func() {
			Expect(capturedForm.Get("grant_type")).To(Equal("authorization_code"))
		})

		It("should NOT persist any refresh token", func() {
			Expect(persister.called).To(BeFalse())
		})
	})

	Describe("when Google downgrades the granted scope", func() {
		BeforeEach(func() {
			tokenHandler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tokenResponse{
					AccessToken:  "ya29.access",
					TokenType:    "Bearer",
					ExpiresIn:    3600,
					RefreshToken: "1//refresh-token",
					Scope:        "https://www.googleapis.com/auth/tasks.readonly",
				})
			}
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render a 403 scope error page", func() {
			Expect(recorder.Code).To(Equal(http.StatusForbidden))
			Expect(recorder.Body.String()).To(ContainSubstring("Tasks scope was not granted"))
		})

		It("should NOT persist any refresh token", func() {
			Expect(persister.called).To(BeFalse())
		})
	})

	Describe("when Google's response omits the refresh token", func() {
		BeforeEach(func() {
			tokenHandler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tokenResponse{
					AccessToken: "ya29.access",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
					Scope:       googleTasksScope,
					// no RefreshToken
				})
			}
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render an explanatory error page", func() {
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(ContainSubstring("No refresh token"))
		})

		It("should NOT call the persister", func() {
			Expect(persister.called).To(BeFalse())
		})
	})

	Describe("when persistence fails", func() {
		BeforeEach(func() {
			persister.err = errFakePersist
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should render a 500 error page", func() {
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should mention the persist failure", func() {
			Expect(recorder.Body.String()).To(ContainSubstring("Could not save"))
		})
	})

	Describe("when the full happy path runs", func() {
		BeforeEach(func() {
			startServer()
			issuedState = issueState()
			query := url.Values{
				"code":  []string{"auth-code"},
				"state": []string{issuedState},
				"iss":   []string{googleOAuthIssuer},
			}
			handler.serveCallback(recorder, makeRequest(query.Encode()))
		})

		It("should send the wiki's redirect URI in the exchange", func() {
			Expect(capturedForm.Get("redirect_uri")).To(Equal("https://wiki.example/oauth/google/callback"))
		})

		It("should send the wiki's client_id in the exchange", func() {
			Expect(capturedForm.Get("client_id")).To(Equal("client-id"))
		})

		It("should send the wiki's client_secret in the exchange", func() {
			Expect(capturedForm.Get("client_secret")).To(Equal("client-secret"))
		})

		It("should send the authorization code", func() {
			Expect(capturedForm.Get("code")).To(Equal("auth-code"))
		})

		It("should send the PKCE code_verifier from the state entry", func() {
			Expect(capturedForm.Get("code_verifier")).To(Equal("verifier-abc"))
		})

		It("should redirect to the profile page on success", func() {
			Expect(recorder.Code).To(Equal(http.StatusFound))
			Expect(recorder.Header().Get("Location")).To(Equal(oauthSuccessRedirectPath))
		})

		It("should persist the refresh token under the right profile", func() {
			Expect(persister.profileID).To(Equal("user@example.com"))
			Expect(persister.token).To(Equal("1//refresh-token"))
		})
	})

	Describe("scopeIncludes", func() {
		It("should return true when the granted scope contains the required token", func() {
			Expect(scopeIncludes("a b https://www.googleapis.com/auth/tasks c", googleTasksScope)).To(BeTrue())
		})

		It("should return false when the granted scope does not contain the required token", func() {
			Expect(scopeIncludes("a b c", googleTasksScope)).To(BeFalse())
		})

		It("should return false on empty granted scope", func() {
			Expect(scopeIncludes("", googleTasksScope)).To(BeFalse())
		})

		It("should not partial-match (substring of a scope token is not a match)", func() {
			Expect(scopeIncludes("https://www.googleapis.com/auth/tasks.readonly", googleTasksScope)).To(BeFalse())
		})
	})

	Describe("HandleNotConfiguredCallback", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			gctx, _ := gin.CreateTestContext(rec)
			gctx.Request = httptest.NewRequest(http.MethodGet, "/oauth/google/callback", nil)
			HandleNotConfiguredCallback(gctx)
		})

		It("should return 503", func() {
			Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
		})

		It("should render an HTML page mentioning configuration", func() {
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(rec.Body.String()).To(ContainSubstring("not configured"))
		})
	})

	Describe("IsNotConfigured", func() {
		It("should report true for the sentinel", func() {
			Expect(IsNotConfigured(errNotConfigured)).To(BeTrue())
		})

		It("should report false for an unrelated error", func() {
			Expect(IsNotConfigured(io.EOF)).To(BeFalse())
		})

		It("should report false for nil", func() {
			Expect(IsNotConfigured(nil)).To(BeFalse())
		})
	})

	Describe("SetOAuthGoogleHandler / getOAuthGoogleHandler / handleOAuthGoogleCallback", func() {
		AfterEach(func() {
			// Reset the global so other tests start clean.
			SetOAuthGoogleHandler(nil)
		})

		Describe("when no handler has been installed", func() {
			BeforeEach(func() {
				SetOAuthGoogleHandler(nil)
			})

			It("getOAuthGoogleHandler should return nil", func() {
				Expect(getOAuthGoogleHandler()).To(BeNil())
			})
		})

		Describe("when a handler has been installed", func() {
			var installed *OAuthGoogleHandler

			BeforeEach(func() {
				installed = &OAuthGoogleHandler{
					IssuerExpected: "https://installed.example.com",
				}
				SetOAuthGoogleHandler(installed)
			})

			It("getOAuthGoogleHandler should return it", func() {
				Expect(getOAuthGoogleHandler()).To(Equal(installed))
			})
		})

		Describe("handleOAuthGoogleCallback", func() {
			Describe("when no handler is installed", func() {
				var rec *httptest.ResponseRecorder

				BeforeEach(func() {
					SetOAuthGoogleHandler(nil)
					s := &Site{}
					rec = httptest.NewRecorder()
					gctx, _ := gin.CreateTestContext(rec)
					gctx.Request = httptest.NewRequest(http.MethodGet, "/oauth/google/callback", nil)
					s.handleOAuthGoogleCallback(gctx)
				})

				It("should render 503 not-configured page", func() {
					Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
				})
			})

			Describe("when a handler is installed", func() {
				var rec *httptest.ResponseRecorder

				BeforeEach(func() {
					startServer()
					// Install the real handler constructed in startServer.
					SetOAuthGoogleHandler(handler)
					rec = httptest.NewRecorder()
					gctx, _ := gin.CreateTestContext(rec)
					// Build a request that will be rejected by iss validation
					// (no iss param) so we get a deterministic non-500 result
					// without needing a full happy-path token exchange.
					gctx.Request = httptest.NewRequest(http.MethodGet, "/oauth/google/callback", nil)
					s := &Site{}
					s.handleOAuthGoogleCallback(gctx)
				})

				It("should delegate to the installed handler", func() {
					// iss absent → 400 per RFC 9207 strict policy.
					Expect(rec.Code).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Describe("HandleCallback (gin wrapper)", func() {
			Describe("when called via a gin context", func() {
				var rec *httptest.ResponseRecorder

				BeforeEach(func() {
					startServer()
					rec = httptest.NewRecorder()
					gctx, _ := gin.CreateTestContext(rec)
					// No iss param → 400 immediately.
					gctx.Request = httptest.NewRequest(http.MethodGet, "/oauth/google/callback", nil)
					handler.HandleCallback(gctx)
				})

				It("should produce the same result as handleCallback", func() {
					Expect(rec.Code).To(Equal(http.StatusBadRequest))
				})
			})
		})
	})

	Describe("NewOAuthGoogleHandler", func() {
		Describe("when required environment variables are set", func() {
			var (
				h   *OAuthGoogleHandler
				err error
			)

			BeforeEach(func() {
				GinkgoT().Setenv(envGoogleTasksClientID, "cid-test")
				GinkgoT().Setenv(envGoogleTasksClientSecret, "csec-test")
				GinkgoT().Setenv(envGoogleTasksRedirectURI, "https://wiki.example/oauth/google/callback")
				h, err = NewOAuthGoogleHandler(stateStore, persister, identity, authIssuer)
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should populate ClientID from env", func() {
				Expect(h.ClientID).To(Equal("cid-test"))
			})

			It("should populate ClientSecret from env", func() {
				Expect(h.ClientSecret).To(Equal("csec-test"))
			})

			It("should populate RedirectURI from env", func() {
				Expect(h.RedirectURI).To(Equal("https://wiki.example/oauth/google/callback"))
			})

			It("should pin the Google issuer", func() {
				Expect(h.IssuerExpected).To(Equal(googleOAuthIssuer))
			})
		})

		Describe("when CLIENT_ID env var is absent", func() {
			var err error

			BeforeEach(func() {
				GinkgoT().Setenv(envGoogleTasksClientID, "")
				GinkgoT().Setenv(envGoogleTasksClientSecret, "csec")
				GinkgoT().Setenv(envGoogleTasksRedirectURI, "https://redir")
				_, err = NewOAuthGoogleHandler(stateStore, persister, identity, authIssuer)
			})

			It("should return errNotConfigured", func() {
				Expect(IsNotConfigured(err)).To(BeTrue())
			})
		})
	})

	Describe("tryBuildAuthURL", func() {
		Describe("when AuthURLIssuer is nil", func() {
			var result string

			BeforeEach(func() {
				startServer()
				handler.AuthURLIssuer = nil
				result = handler.tryBuildAuthURL(httptest.NewRequest(http.MethodGet, "/", nil))
			})

			It("should return empty string", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Describe("when IdentityResolver is nil", func() {
			var result string

			BeforeEach(func() {
				startServer()
				handler.IdentityResolver = nil
				result = handler.tryBuildAuthURL(httptest.NewRequest(http.MethodGet, "/", nil))
			})

			It("should return empty string", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Describe("when IdentityResolver returns an error", func() {
			var result string

			BeforeEach(func() {
				startServer()
				handler.IdentityResolver = &fakeIdentityResolver{err: io.EOF}
				result = handler.tryBuildAuthURL(httptest.NewRequest(http.MethodGet, "/", nil))
			})

			It("should return empty string", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Describe("when AuthURLIssuer returns an error", func() {
			var result string

			BeforeEach(func() {
				startServer()
				handler.AuthURLIssuer = &fakeAuthURLIssuer{err: io.EOF}
				result = handler.tryBuildAuthURL(httptest.NewRequest(http.MethodGet, "/", nil))
			})

			It("should return empty string", func() {
				Expect(result).To(BeEmpty())
			})
		})
	})
})

// errFakePersist is a sentinel used by the persist-failure test case.
var errFakePersist = stringError("persist exploded")

type stringError string

func (e stringError) Error() string { return string(e) }


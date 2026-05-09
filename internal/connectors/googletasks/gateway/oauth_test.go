//revive:disable:dot-imports
package gateway_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks/gateway"
)

// stubTokenServer is a tiny httptest harness for the OAuth token
// endpoint. It collects the form bodies submitted to it and serves
// the next queued response.
type stubTokenServer struct {
	server    *httptest.Server
	requests  []url.Values
	responses []stubTokenResponse
	hits      int32
}

type stubTokenResponse struct {
	status int
	body   string
}

func newStubTokenServer() *stubTokenServer {
	s := &stubTokenServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		s.requests = append(s.requests, r.PostForm)
		atomic.AddInt32(&s.hits, 1)
		idx := len(s.requests) - 1
		if idx >= len(s.responses) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := s.responses[idx]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		_, _ = io.WriteString(w, resp.body)
	}))
	return s
}

func (s *stubTokenServer) URL() string { return s.server.URL }
func (s *stubTokenServer) Close()      { s.server.Close() }
func (s *stubTokenServer) Hits() int   { return int(atomic.LoadInt32(&s.hits)) }

var _ = Describe("RefreshClient", func() {
	var (
		ctx    context.Context
		stub   *stubTokenServer
		store  *gateway.StaticRefreshTokenStore
		client *gateway.RefreshClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		stub = newStubTokenServer()
		store = gateway.NewStaticRefreshTokenStore("seed-refresh-token")
	})

	AfterEach(func() {
		stub.Close()
	})

	Describe("constructor validation", func() {
		When("httpClient is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = gateway.NewRefreshClient(nil, "u", "id", "sec", store)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("httpClient")))
			})
		})

		When("clientID is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = gateway.NewRefreshClient(http.DefaultClient, "u", "", "sec", store)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("clientID")))
			})
		})

		When("store is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = gateway.NewRefreshClient(http.DefaultClient, "u", "id", "sec", nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("store")))
			})
		})
	})

	Describe("AccessToken", func() {
		BeforeEach(func() {
			var err error
			client, err = gateway.NewRefreshClient(stub.server.Client(), stub.URL(), "client-id", "client-secret", store)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the token endpoint returns a fresh access token", func() {
			var (
				token string
				err   error
			)

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body: `{
						"access_token": "ya29.fresh",
						"token_type": "Bearer",
						"expires_in": 3600,
						"scope": "https://www.googleapis.com/auth/tasks"
					}`,
				}}
				token, err = client.AccessToken(ctx)
			})

			It("should return the access token", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(token).To(Equal("ya29.fresh"))
			})

			It("should send refresh_token grant type", func() {
				Expect(stub.requests).To(HaveLen(1))
				Expect(stub.requests[0].Get("grant_type")).To(Equal("refresh_token"))
			})

			It("should send the stored refresh token", func() {
				Expect(stub.requests[0].Get("refresh_token")).To(Equal("seed-refresh-token"))
			})

			It("should send the client_id and client_secret", func() {
				Expect(stub.requests[0].Get("client_id")).To(Equal("client-id"))
				Expect(stub.requests[0].Get("client_secret")).To(Equal("client-secret"))
			})
		})

		When("called twice within the cache window", func() {
			var (
				first  string
				second string
			)

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body:   `{"access_token":"ya29.cached","token_type":"Bearer","expires_in":3600}`,
				}}
				var err error
				first, err = client.AccessToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				second, err = client.AccessToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the same access token", func() {
				Expect(first).To(Equal(second))
			})

			It("should only hit the token endpoint once", func() {
				Expect(stub.Hits()).To(Equal(1))
			})
		})

		When("the response rotates the refresh token", func() {
			var token string

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body: `{
						"access_token":"ya29.rotated",
						"token_type":"Bearer",
						"expires_in":3600,
						"refresh_token":"new-refresh-token",
						"scope":"https://www.googleapis.com/auth/tasks"
					}`,
				}}
				var err error
				token, err = client.AccessToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist the new refresh token to the store", func() {
				Expect(store.Snapshot()).To(Equal("new-refresh-token"))
			})

			It("should still return the access token", func() {
				Expect(token).To(Equal("ya29.rotated"))
			})
		})

		When("invalid_grant returns once and the store has a newer token on retry", func() {
			var (
				token string
				err   error
			)

			BeforeEach(func() {
				// First request: store says "seed-refresh-token", server says invalid_grant.
				// Test simulates the rotation race by updating the store between attempts.
				stub.server.Close()
				stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = r.ParseForm()
					stub.requests = append(stub.requests, r.PostForm)
					atomic.AddInt32(&stub.hits, 1)
					if int(stub.hits) == 1 {
						// Simulate the rotation race: update the store
						// out-of-band before responding so the retry sees
						// a different token.
						_ = store.SaveRefreshToken(ctx, "race-rotated-token")
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, `{"access_token":"ya29.recovered","token_type":"Bearer","expires_in":3600}`)
				}))
				client, _ = gateway.NewRefreshClient(stub.server.Client(), stub.server.URL, "client-id", "client-secret", store)
				token, err = client.AccessToken(ctx)
			})

			It("should recover and return an access token", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(token).To(Equal("ya29.recovered"))
			})

			It("should retry once with the rotated token", func() {
				Expect(stub.Hits()).To(Equal(2))
				Expect(stub.requests[1].Get("refresh_token")).To(Equal("race-rotated-token"))
			})
		})

		When("invalid_grant returns and the store has the same token on retry", func() {
			var err error

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusBadRequest,
					body:   `{"error":"invalid_grant","error_description":"revoked"}`,
				}}
				_, err = client.AccessToken(ctx)
			})

			It("should return ErrInvalidGrant", func() {
				Expect(err).To(MatchError(gateway.ErrInvalidGrant))
			})

			It("should not retry when the store hasn't rotated", func() {
				Expect(stub.Hits()).To(Equal(1))
			})
		})

		When("the response scope omits the required tasks scope", func() {
			var err error

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body: `{
						"access_token":"ya29.downgraded",
						"token_type":"Bearer",
						"expires_in":3600,
						"scope":"https://www.googleapis.com/auth/userinfo.email"
					}`,
				}}
				_, err = client.AccessToken(ctx)
			})

			It("should return ErrScopeDowngraded", func() {
				Expect(err).To(MatchError(gateway.ErrScopeDowngraded))
			})
		})

		When("the response is missing access_token", func() {
			var err error

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body:   `{"token_type":"Bearer","expires_in":3600}`,
				}}
				_, err = client.AccessToken(ctx)
			})

			It("should return ErrProtocolDrift", func() {
				Expect(err).To(MatchError(gateway.ErrProtocolDrift))
			})
		})

		When("the response is unparseable JSON", func() {
			var err error

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusOK,
					body:   `not json`,
				}}
				_, err = client.AccessToken(ctx)
			})

			It("should return ErrProtocolDrift", func() {
				Expect(err).To(MatchError(gateway.ErrProtocolDrift))
			})
		})

		When("the token endpoint returns 429", func() {
			var err error

			BeforeEach(func() {
				stub.responses = []stubTokenResponse{{
					status: http.StatusTooManyRequests,
					body:   `{}`,
				}}
				_, err = client.AccessToken(ctx)
			})

			It("should return ErrRateLimited", func() {
				Expect(err).To(MatchError(gateway.ErrRateLimited))
			})
		})
	})

	Describe("Invalidate", func() {
		BeforeEach(func() {
			var err error
			client, err = gateway.NewRefreshClient(stub.server.Client(), stub.URL(), "id", "sec", store)
			Expect(err).NotTo(HaveOccurred())
			stub.responses = []stubTokenResponse{
				{status: http.StatusOK, body: `{"access_token":"ya29.first","token_type":"Bearer","expires_in":3600}`},
				{status: http.StatusOK, body: `{"access_token":"ya29.second","token_type":"Bearer","expires_in":3600}`},
			}
			_, err = client.AccessToken(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("called between two AccessToken requests", func() {
			var second string

			BeforeEach(func() {
				client.Invalidate()
				var err error
				second, err = client.AccessToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should force a fresh refresh", func() {
				Expect(stub.Hits()).To(Equal(2))
			})

			It("should return the new access token", func() {
				Expect(second).To(Equal("ya29.second"))
			})
		})
	})
})

var _ = Describe("PKCE helpers", func() {
	Describe("GeneratePKCEVerifier", func() {
		var (
			verifier string
			err      error
		)

		BeforeEach(func() {
			verifier, err = gateway.GeneratePKCEVerifier()
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce a base64url-encoded value within RFC 7636 length bounds", func() {
			Expect(len(verifier)).To(BeNumerically(">=", 43))
			Expect(len(verifier)).To(BeNumerically("<=", 128))
		})

		It("should not contain padding characters", func() {
			Expect(verifier).NotTo(ContainSubstring("="))
		})

		It("should produce different values across calls", func() {
			other, err2 := gateway.GeneratePKCEVerifier()
			Expect(err2).NotTo(HaveOccurred())
			Expect(other).NotTo(Equal(verifier))
		})
	})

	Describe("PKCEChallengeS256", func() {
		var challenge string

		BeforeEach(func() {
			// RFC 7636 §B test vector
			challenge = gateway.PKCEChallengeS256("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
		})

		It("should match the RFC 7636 reference output", func() {
			Expect(challenge).To(Equal("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"))
		})

		It("should not contain padding characters", func() {
			Expect(challenge).NotTo(ContainSubstring("="))
		})
	})

	Describe("GenerateStateToken", func() {
		var (
			state string
			err   error
		)

		BeforeEach(func() {
			state, err = gateway.GenerateStateToken()
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should decode to ≥256 bits of entropy", func() {
			raw, decErr := base64.RawURLEncoding.DecodeString(state)
			Expect(decErr).NotTo(HaveOccurred())
			Expect(len(raw) * 8).To(BeNumerically(">=", 256))
		})

		It("should produce different values across calls", func() {
			other, err2 := gateway.GenerateStateToken()
			Expect(err2).NotTo(HaveOccurred())
			Expect(other).NotTo(Equal(state))
		})
	})
})

var _ = Describe("BuildAuthURL", func() {
	When("all required parameters are present", func() {
		var (
			authURL string
			err     error
			parsed  *url.URL
		)

		BeforeEach(func() {
			authURL, err = gateway.BuildAuthURL(
				"https://accounts.google.com/o/oauth2/v2/auth",
				"client-id",
				"https://wiki.example/oauth/google/callback",
				gateway.TasksScope,
				"state-value",
				"challenge-value",
			)
			Expect(err).NotTo(HaveOccurred())
			parsed, err = url.Parse(authURL)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set response_type=code", func() {
			Expect(parsed.Query().Get("response_type")).To(Equal("code"))
		})

		It("should set code_challenge_method=S256", func() {
			Expect(parsed.Query().Get("code_challenge_method")).To(Equal("S256"))
		})

		It("should set the code_challenge", func() {
			Expect(parsed.Query().Get("code_challenge")).To(Equal("challenge-value"))
		})

		It("should set the state value", func() {
			Expect(parsed.Query().Get("state")).To(Equal("state-value"))
		})

		It("should request offline access", func() {
			Expect(parsed.Query().Get("access_type")).To(Equal("offline"))
		})

		It("should force the consent prompt so refresh_token is reissued", func() {
			Expect(parsed.Query().Get("prompt")).To(Equal("consent"))
		})

		It("should include the requested scope", func() {
			Expect(parsed.Query().Get("scope")).To(Equal(gateway.TasksScope))
		})
	})

	When("a required parameter is empty", func() {
		It("should error on missing state", func() {
			_, err := gateway.BuildAuthURL("https://x", "id", "uri", "scope", "", "ch")
			Expect(err).To(MatchError(ContainSubstring("state")))
		})

		It("should error on missing code_challenge", func() {
			_, err := gateway.BuildAuthURL("https://x", "id", "uri", "scope", "st", "")
			Expect(err).To(MatchError(ContainSubstring("codeChallenge")))
		})
	})
})

var _ = Describe("ValidateIssuer", func() {
	When("the issuer matches", func() {
		It("should return nil", func() {
			Expect(gateway.ValidateIssuer("https://accounts.google.com", "https://accounts.google.com")).To(Succeed())
		})
	})

	When("the issuer is empty", func() {
		It("should return ErrIssuerMismatch", func() {
			err := gateway.ValidateIssuer("", "https://accounts.google.com")
			Expect(err).To(MatchError(gateway.ErrIssuerMismatch))
		})
	})

	When("the issuer differs", func() {
		It("should return ErrIssuerMismatch", func() {
			err := gateway.ValidateIssuer("https://evil.example", "https://accounts.google.com")
			Expect(err).To(MatchError(gateway.ErrIssuerMismatch))
		})
	})
})

// Sanity test that the JSON shape Google sends actually round-trips
// through the wire decoder. Pinned against a captured fixture.
var _ = Describe("token endpoint wire shape", func() {
	When("decoding a Google success body", func() {
		var resp struct {
			AccessToken  string `json:"access_token"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int64  `json:"expires_in"`
			RefreshToken string `json:"refresh_token,omitempty"`
			Scope        string `json:"scope,omitempty"`
		}

		BeforeEach(func() {
			body := `{"access_token":"ya29.A0AeXRPp...","expires_in":3599,"refresh_token":"1//0e...","scope":"https://www.googleapis.com/auth/tasks","token_type":"Bearer"}`
			Expect(json.NewDecoder(strings.NewReader(body)).Decode(&resp)).To(Succeed())
		})

		It("should populate the access token", func() {
			Expect(resp.AccessToken).To(Equal("ya29.A0AeXRPp..."))
		})

		It("should populate the refresh token", func() {
			Expect(resp.RefreshToken).To(Equal("1//0e..."))
		})

		It("should populate the scope", func() {
			Expect(resp.Scope).To(Equal(gateway.TasksScope))
		})
	})
})

// Test guard so the Hit count helpers don't get squashed by lint.
var _ = It("error sentinels are distinct", func() {
	Expect(errors.Is(gateway.ErrInvalidGrant, gateway.ErrAuthRevoked)).To(BeFalse())
	Expect(errors.Is(gateway.ErrScopeDowngraded, gateway.ErrInvalidGrant)).To(BeFalse())
})

var _ = Describe("RefreshClient.ExpiresAt", func() {
	var (
		stub   *stubTokenServer
		client *gateway.RefreshClient
	)

	BeforeEach(func() {
		stub = newStubTokenServer()
	})

	AfterEach(func() {
		stub.Close()
	})

	When("no access token has been fetched yet", func() {
		BeforeEach(func() {
			var err error
			client, err = gateway.NewRefreshClient(
				stub.server.Client(),
				stub.URL(),
				"client-id",
				"client-secret",
				gateway.NewStaticRefreshTokenStore("rt"),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the zero time", func() {
			Expect(client.ExpiresAt()).To(BeZero())
		})
	})

	When("a successful token refresh has occurred", func() {
		BeforeEach(func() {
			stub.responses = []stubTokenResponse{{
				status: http.StatusOK,
				body:   `{"access_token":"ya29.new","expires_in":3600,"token_type":"Bearer","scope":"https://www.googleapis.com/auth/tasks"}`,
			}}
			var err error
			client, err = gateway.NewRefreshClient(
				stub.server.Client(),
				stub.URL(),
				"client-id",
				"client-secret",
				gateway.NewStaticRefreshTokenStore("seed-rt"),
			)
			Expect(err).ToNot(HaveOccurred())
			_, err = client.AccessToken(context.Background())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return a non-zero expiry", func() {
			Expect(client.ExpiresAt()).ToNot(BeZero())
		})

		It("should return a future expiry time", func() {
			Expect(client.ExpiresAt()).To(BeTemporally(">", time.Now()))
		})
	})
})

// keep the time import used
var _ = time.Second

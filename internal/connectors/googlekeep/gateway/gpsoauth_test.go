//revive:disable:dot-imports
package gateway_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
)

func TestGateway(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/connectors/google_keep/gateway")
}

// canonicalBearerSuccess is the Stage 2 success-case body — Auth= carries
// the short-lived service bearer; Token may or may not be present.
const canonicalBearerSuccess = "SID=fakesid\n" +
	"LSID=fakelsid\n" +
	"Auth=ya29.fake-bearer\n" +
	"issueAdvice=auto\n" +
	"services=hist,mail\n"

var _ = Describe("ExchangeMasterTokenForBearer", func() {
	var (
		ctx          context.Context
		auth         *gateway.Authenticator
		fakeServer   *httptest.Server
		lastRequest  *http.Request
		lastBody     string
		responseBody string
		responseCode int
	)

	BeforeEach(func() {
		ctx = context.Background()
		responseBody = canonicalBearerSuccess
		responseCode = http.StatusOK
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastRequest = r
			b, _ := io.ReadAll(r.Body)
			lastBody = string(b)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(responseCode)
			_, _ = io.WriteString(w, responseBody)
		}))
		auth = gateway.NewAuthenticator(fakeServer.Client(), fakeServer.URL, "0123456789abcdef")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("Google returns a successful Auth line", func() {
		var (
			bearer      string
			exchangeErr error
		)

		BeforeEach(func() {
			bearer, exchangeErr = auth.ExchangeMasterTokenForBearer(ctx, "alice@example.com", "oauth2rt_1/fakeMasterToken")
		})

		It("should not error", func() {
			Expect(exchangeErr).ToNot(HaveOccurred())
		})

		It("should return the Auth value as the bearer", func() {
			Expect(bearer).To(Equal("ya29.fake-bearer"))
		})

		It("should request the Keep service scope", func() {
			form, err := url.ParseQuery(lastBody)
			Expect(err).ToNot(HaveOccurred())
			Expect(form.Get("service")).To(Equal(gateway.KeepServiceScope))
		})

		It("should claim the Keep Android app id", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Get("app")).To(Equal("com.google.android.keep"))
		})

		It("should put the master token in EncryptedPasswd verbatim (Stage 2 reuse)", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Get("EncryptedPasswd")).To(Equal("oauth2rt_1/fakeMasterToken"))
		})

		It("should not send add_account (Stage 2 omits it)", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Has("add_account")).To(BeFalse())
		})

		It("should not send callerSig (Stage 2 omits it)", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Has("callerSig")).To(BeFalse())
		})

		It("should send the GoogleAuth user-agent", func() {
			Expect(lastRequest.Header.Get("User-Agent")).To(Equal("GoogleAuth/1.4"))
		})
	})

	When("Google returns BadAuthentication on Stage 2", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "Error=BadAuthentication\n"
			responseCode = http.StatusForbidden
			_, exchangeErr = auth.ExchangeMasterTokenForBearer(ctx, "alice@example.com", "oauth2rt_1/revoked")
		})

		It("should return ErrAuthRevoked (master token no longer valid)", func() {
			// Stage 2 BadAuth means the master token itself is revoked,
			// not a typo. Distinct from Stage 1.
			Expect(exchangeErr).To(MatchError(gateway.ErrAuthRevoked))
		})
	})

	When("Google returns NeedsBrowser on Stage 2", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "Error=NeedsBrowser\n"
			responseCode = http.StatusForbidden
			_, exchangeErr = auth.ExchangeMasterTokenForBearer(ctx, "alice@example.com", "oauth2rt_1/fakeMasterToken")
		})

		It("should return ErrAuthRevoked", func() {
			Expect(exchangeErr).To(MatchError(gateway.ErrAuthRevoked))
		})
	})

	When("Google returns no Auth and no Error on Stage 2 (protocol drift)", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "SomeOtherField=value\n"
			_, exchangeErr = auth.ExchangeMasterTokenForBearer(ctx, "alice@example.com", "oauth2rt_1/fakeMasterToken")
		})

		It("should return ErrProtocolDrift", func() {
			Expect(exchangeErr).To(MatchError(gateway.ErrProtocolDrift))
		})
	})

	When("the server returns HTTP 503 on Stage 2", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "Error=ServiceUnavailable\n"
			responseCode = http.StatusServiceUnavailable
			_, exchangeErr = auth.ExchangeMasterTokenForBearer(ctx, "alice@example.com", "oauth2rt_1/fakeMasterToken")
		})

		It("should return a transport-level error, not a typed auth sentinel", func() {
			Expect(exchangeErr).To(HaveOccurred())
			Expect(exchangeErr).ToNot(MatchError(gateway.ErrInvalidCredentials))
			Expect(exchangeErr).ToNot(MatchError(gateway.ErrAuthRevoked))
		})
	})
})

var _ = Describe("ExchangeOAuthTokenForMasterToken", func() {
	var (
		ctx          context.Context
		auth         *gateway.Authenticator
		fakeServer   *httptest.Server
		responseBody string
		responseCode int
	)

	BeforeEach(func() {
		ctx = context.Background()
		responseBody = "Token=master_token_here\n"
		responseCode = http.StatusOK
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(responseCode)
			_, _ = io.WriteString(w, responseBody)
		}))
		auth = gateway.NewAuthenticator(fakeServer.Client(), fakeServer.URL, "0123456789abcdef")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("Google returns a successful Token", func() {
		var (
			token string
			err   error
		)

		BeforeEach(func() {
			token, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "oauth_token_here")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the Token value as the master token", func() {
			Expect(token).To(Equal("master_token_here"))
		})
	})

	When("Google returns BadAuthentication (wrong credentials)", func() {
		var err error

		BeforeEach(func() {
			responseBody = "Error=BadAuthentication\nErrorDetail=InvalidPasswordError\n"
			responseCode = http.StatusForbidden
			_, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "bad_oauth_token")
		})

		It("should return ErrInvalidCredentials", func() {
			Expect(err).To(MatchError(gateway.ErrInvalidCredentials))
		})
	})

	When("Google returns NeedsBrowser (2FA or security check required)", func() {
		var err error

		BeforeEach(func() {
			responseBody = "Error=NeedsBrowser\n"
			responseCode = http.StatusForbidden
			_, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "needs_browser_token")
		})

		It("should return ErrAuthRevoked", func() {
			Expect(err).To(MatchError(gateway.ErrAuthRevoked))
		})
	})

	When("Google returns no Token and no Error (protocol drift)", func() {
		var err error

		BeforeEach(func() {
			responseBody = "SomeOtherField=value\n"
			_, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "oauth_token")
		})

		It("should return ErrProtocolDrift", func() {
			Expect(err).To(MatchError(gateway.ErrProtocolDrift))
		})
	})

	When("Google returns an unrecognized Error value", func() {
		var err error

		BeforeEach(func() {
			responseBody = "Error=AccountDeleted\n"
			responseCode = http.StatusForbidden
			_, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "oauth_token")
		})

		It("should return ErrAuthRevoked (catch-all for unrecognized errors)", func() {
			Expect(err).To(MatchError(gateway.ErrAuthRevoked))
		})
	})

	When("the auth server returns HTTP 503", func() {
		var err error

		BeforeEach(func() {
			responseBody = "Error=ServiceUnavailable\n"
			responseCode = http.StatusServiceUnavailable
			_, err = auth.ExchangeOAuthTokenForMasterToken(ctx, "alice@example.com", "oauth_token")
		})

		It("should return a transport-level error, not a typed auth sentinel", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(MatchError(gateway.ErrInvalidCredentials))
			Expect(err).ToNot(MatchError(gateway.ErrAuthRevoked))
		})
	})
})

//revive:disable:dot-imports
package protocol_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

func TestProtocol(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/keep/protocol")
}

// canonicalAuthSuccess mirrors the success-case body Google returns from
// /auth — text/plain key=value lines with a Token= line carrying the master
// token. Captured from gkeepapi/gpsoauth fixture documentation; see
// REFERENCE.md "Stage 1".
const canonicalAuthSuccess = "SID=fakesid\n" +
	"LSID=fakelsid\n" +
	"Auth=fakeauth\n" +
	"Token=oauth2rt_1/fakeMasterToken\n" +
	"Email=alice@example.com\n" +
	"services=hist,mail\n"

// canonicalBearerSuccess is the Stage 2 success-case body — Auth= carries
// the short-lived service bearer; Token may or may not be present.
const canonicalBearerSuccess = "SID=fakesid\n" +
	"LSID=fakelsid\n" +
	"Auth=ya29.fake-bearer\n" +
	"issueAdvice=auto\n" +
	"services=hist,mail\n"

var _ = Describe("ExchangeASPForMasterToken", func() {
	var (
		ctx          context.Context
		auth         *protocol.Authenticator
		fakeServer   *httptest.Server
		lastRequest  *http.Request
		lastBody     string
		responseBody string
		responseCode int
	)

	BeforeEach(func() {
		ctx = context.Background()
		responseBody = canonicalAuthSuccess
		responseCode = http.StatusOK
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastRequest = r
			b, _ := io.ReadAll(r.Body)
			lastBody = string(b)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(responseCode)
			_, _ = io.WriteString(w, responseBody)
		}))
		auth = protocol.NewAuthenticator(fakeServer.Client(), fakeServer.URL, "0123456789abcdef")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("Google returns a successful response with a Token line", func() {
		var (
			masterToken string
			exchangeErr error
		)

		BeforeEach(func() {
			masterToken, exchangeErr = auth.ExchangeASPForMasterToken(ctx, "alice@example.com", "abcdefghijklmnop")
		})

		It("should not error", func() {
			Expect(exchangeErr).ToNot(HaveOccurred())
		})

		It("should return the master token from the response Token line", func() {
			Expect(masterToken).To(Equal("oauth2rt_1/fakeMasterToken"))
		})

		It("should POST to the configured auth URL", func() {
			Expect(lastRequest).ToNot(BeNil())
			Expect(lastRequest.Method).To(Equal(http.MethodPost))
		})

		It("should send the GoogleAuth user-agent", func() {
			Expect(lastRequest.Header.Get("User-Agent")).To(Equal("GoogleAuth/1.4"))
		})

		It("should send a form-urlencoded body", func() {
			Expect(lastRequest.Header.Get("Content-Type")).To(HavePrefix("application/x-www-form-urlencoded"))
		})

		It("should include the email in the form body", func() {
			form, err := url.ParseQuery(lastBody)
			Expect(err).ToNot(HaveOccurred())
			Expect(form.Get("Email")).To(Equal("alice@example.com"))
		})

		It("should request the ac2dm service", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Get("service")).To(Equal("ac2dm"))
		})

		It("should send the device id from the constructor", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Get("androidId")).To(Equal("0123456789abcdef"))
		})

		It("should include EncryptedPasswd (RSA-encrypted email+ASP)", func() {
			form, _ := url.ParseQuery(lastBody)
			ep := form.Get("EncryptedPasswd")
			Expect(ep).ToNot(BeEmpty())
			// Construction = 1 version byte + 4 fingerprint bytes + RSA-OAEP
			// ciphertext (1024-bit key → 128 bytes), urlsafe-base64-encoded.
			// Total decoded length is 133 bytes => base64 length ~= 180.
			Expect(len(ep)).To(BeNumerically(">=", 170))
			// urlsafe alphabet only.
			Expect(strings.ContainsAny(ep, "+/")).To(BeFalse())
		})

		It("should not echo the ASP back in the form body", func() {
			Expect(lastBody).ToNot(ContainSubstring("abcdefghijklmnop"))
		})

		It("should pin client_sig to the Play Services 7.3.29 cert", func() {
			form, _ := url.ParseQuery(lastBody)
			Expect(form.Get("client_sig")).To(Equal("38918a453d07199354f8b19af05ec6562ced5788"))
		})
	})

	When("Google returns BadAuthentication", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "Error=BadAuthentication\n"
			responseCode = http.StatusForbidden
			_, exchangeErr = auth.ExchangeASPForMasterToken(ctx, "alice@example.com", "wrongpassword123")
		})

		It("should return ErrInvalidCredentials", func() {
			Expect(exchangeErr).To(MatchError(protocol.ErrInvalidCredentials))
		})
	})

	When("Google returns NeedsBrowser", func() {
		var exchangeErr error

		BeforeEach(func() {
			responseBody = "Error=NeedsBrowser\nUrl=https://accounts.google.com/...\n"
			responseCode = http.StatusForbidden
			_, exchangeErr = auth.ExchangeASPForMasterToken(ctx, "alice@example.com", "abcdefghijklmnop")
		})

		It("should return ErrAuthRevoked", func() {
			Expect(exchangeErr).To(MatchError(protocol.ErrAuthRevoked))
		})
	})

	When("Google returns a 5xx", func() {
		var (
			masterToken string
			exchangeErr error
		)

		BeforeEach(func() {
			responseBody = "Error=ServiceUnavailable\n"
			responseCode = http.StatusServiceUnavailable
			masterToken, exchangeErr = auth.ExchangeASPForMasterToken(ctx, "alice@example.com", "abcdefghijklmnop")
		})

		It("should return an error", func() {
			Expect(exchangeErr).To(HaveOccurred())
		})

		It("should not return a master token", func() {
			Expect(masterToken).To(BeEmpty())
		})
	})
})

var _ = Describe("ExchangeMasterTokenForBearer", func() {
	var (
		ctx          context.Context
		auth         *protocol.Authenticator
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
		auth = protocol.NewAuthenticator(fakeServer.Client(), fakeServer.URL, "0123456789abcdef")
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
			Expect(form.Get("service")).To(Equal(protocol.KeepServiceScope))
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
			Expect(exchangeErr).To(MatchError(protocol.ErrAuthRevoked))
		})
	})
})

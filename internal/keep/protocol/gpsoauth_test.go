//revive:disable:dot-imports
package protocol_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

func TestProtocol(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/keep/protocol")
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

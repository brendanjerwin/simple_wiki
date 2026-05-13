//revive:disable:dot-imports
package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests cover the h2c -> http.Server.Protocols migration (commit
// 8d97c26a). The migration replaced four h2c.NewHandler wraps with a
// cleartextHTTP2Protocols() helper applied via http.Server.Protocols.
// The goal is to assert (a) the helper's protocol set, (b) that an
// *http.Server configured with the helper retains the HTTP/1 + cleartext
// HTTP/2 capability the deprecated h2c wrap used to provide.
var _ = Describe("cleartextHTTP2Protocols", func() {
	var protocols *http.Protocols

	BeforeEach(func() {
		protocols = cleartextHTTP2Protocols()
	})

	It("should return a non-nil Protocols pointer", func() {
		Expect(protocols).NotTo(BeNil())
	})

	It("should enable HTTP/1", func() {
		Expect(protocols.HTTP1()).To(BeTrue())
	})

	It("should enable unencrypted HTTP/2", func() {
		Expect(protocols.UnencryptedHTTP2()).To(BeTrue())
	})

	It("should not enable TLS-side HTTP/2 (helper is cleartext-only)", func() {
		// The helper sets only HTTP1 and UnencryptedHTTP2; HTTP2 (TLS-side)
		// stays false. TLS callers negotiate h2 via ALPN, which the stdlib
		// handles separately when a TLS listener is in use.
		Expect(protocols.HTTP2()).To(BeFalse())
	})
})

// identifiableHandler is a struct-backed http.Handler used to verify that
// newCleartextHTTP2Server stores the caller's handler verbatim. Comparing
// http.HandlerFunc by pointer identity in Gomega is unreliable because
// func values aren't ==-comparable; using a *struct gives us a stable
// identity to assert against.
type identifiableHandler struct{}

func (*identifiableHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

var _ = Describe("newCleartextHTTP2Server", func() {
	// Constructor helper that the per-mode bootstrap functions (SetupPlainHTTP,
	// SetupTailscaleServe, SetupFullTLS) use to build their *http.Server. The
	// SonarCloud new-code-coverage gate flagged the per-site `Protocols:
	// cleartextHTTP2Protocols()` literals because the enclosing bootstrap
	// functions aren't unit-tested; collapsing them through this constructor
	// gives them a single tested seam.
	var (
		handler *identifiableHandler
		srv     *http.Server
	)

	BeforeEach(func() {
		handler = &identifiableHandler{}
		srv = newCleartextHTTP2Server(handler)
	})

	It("should return a non-nil *http.Server", func() {
		Expect(srv).NotTo(BeNil())
	})

	It("should set Handler to the passed handler (identity)", func() {
		// Pointer-identity check: the constructor must not wrap or
		// substitute the handler — its only job is to compose the
		// HTTP/1 + h2c Protocols shape.
		Expect(srv.Handler).To(BeIdenticalTo(handler))
	})

	When("the returned server's Protocols field is inspected", func() {
		It("should be non-nil", func() {
			Expect(srv.Protocols).NotTo(BeNil())
		})

		It("should have UnencryptedHTTP2 enabled", func() {
			Expect(srv.Protocols.UnencryptedHTTP2()).To(BeTrue())
		})

		It("should have HTTP/1 enabled", func() {
			Expect(srv.Protocols.HTTP1()).To(BeTrue())
		})

		It("should not have TLS-side HTTP/2 enabled (cleartext-only)", func() {
			Expect(srv.Protocols.HTTP2()).To(BeFalse())
		})
	})
})

var _ = Describe("http.Server configured with cleartextHTTP2Protocols", func() {
	// This is the strongest assertion: a real *http.Server with
	// Protocols set to cleartextHTTP2Protocols() must serve both HTTP/1.1
	// and unencrypted HTTP/2 on the same TCP listener — the exact
	// capability the deprecated h2c.NewHandler wrap used to provide.
	const echoBody = "wiki-h2c-echo"

	var (
		srv     *httptest.Server
		baseURL string
	)

	BeforeEach(func() {
		echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Proto", r.Proto)
			_, _ = fmt.Fprint(w, echoBody)
		})
		srv = httptest.NewUnstartedServer(echo)
		// Mirror the production wiring: Handler is the bare handler;
		// cleartext HTTP/2 is enabled via Protocols on the *http.Server.
		srv.Config.Handler = echo
		srv.Config.Protocols = cleartextHTTP2Protocols()
		srv.Start()
		baseURL = srv.URL
	})

	AfterEach(func() {
		srv.Close()
	})

	When("the underlying *http.Server's Protocols field is inspected", func() {
		It("should be non-nil", func() {
			Expect(srv.Config.Protocols).NotTo(BeNil())
		})

		It("should have UnencryptedHTTP2 enabled", func() {
			Expect(srv.Config.Protocols.UnencryptedHTTP2()).To(BeTrue())
		})

		It("should have HTTP/1 enabled", func() {
			Expect(srv.Config.Protocols.HTTP1()).To(BeTrue())
		})
	})

	When("an HTTP/1.1 client issues a request", func() {
		var (
			resp *http.Response
			body []byte
		)

		BeforeEach(func() {
			// Default Transport speaks HTTP/1.1 over cleartext TCP.
			client := &http.Client{Timeout: 5 * time.Second}
			var err error
			resp, err = client.Get(baseURL + "/")
			Expect(err).NotTo(HaveOccurred())
			body, err = io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
		})

		It("should succeed with HTTP 200", func() {
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return the echo body (proves the handler ran)", func() {
			Expect(string(body)).To(Equal(echoBody))
		})

		It("should be served over HTTP/1.x", func() {
			Expect(resp.Proto).To(HavePrefix("HTTP/1."))
		})
	})

	When("an unencrypted-HTTP/2 client issues a request", func() {
		// A client whose Transport.Protocols enables ONLY UnencryptedHTTP2
		// will refuse to fall back to HTTP/1.1, so a successful round trip
		// here proves the server upgraded to h2c.
		var (
			resp *http.Response
			body []byte
		)

		BeforeEach(func() {
			clientProtocols := new(http.Protocols)
			clientProtocols.SetUnencryptedHTTP2(true)

			transport := &http.Transport{
				Protocols: clientProtocols,
			}
			client := &http.Client{
				Transport: transport,
				Timeout:   5 * time.Second,
			}
			var err error
			resp, err = client.Get(baseURL + "/")
			Expect(err).NotTo(HaveOccurred())
			body, err = io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
		})

		It("should succeed with HTTP 200 (proves h2c is wired)", func() {
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return the echo body", func() {
			Expect(string(body)).To(Equal(echoBody))
		})

		It("should be served over HTTP/2", func() {
			Expect(resp.ProtoMajor).To(Equal(2))
		})
	})
})

package tailscale

import (
	"net/http"
	"net/http/httptest"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// internalTestLogger creates a quiet logger for internal-package tests,
// mirroring testLogger in the external test helpers.
func internalTestLogger() *lumber.ConsoleLogger {
	return lumber.NewConsoleLogger(lumber.WARN)
}

// internalMockMetrics is a MetricsRecorder for the internal-package Method 0
// tests. It mirrors the external mockMetricsRecorder but lives in-package so
// the tests can share the tailscale package scope.
type internalMockMetrics struct {
	lookupCalls     int
	extractionCalls int
	lastResult      observability.IdentityLookupResult
}

func (m *internalMockMetrics) RecordTailscaleLookup(result observability.IdentityLookupResult) {
	m.lookupCalls++
	m.lastResult = result
}

func (m *internalMockMetrics) RecordHeaderExtraction() {
	m.extractionCalls++
}

// These tests live in the internal (`package tailscale`) test file so they can
// drive Method 0 through the unexported trustedProxySecretOverride seam without
// mutating the process environment (the sync.OnceValue cache makes env-only
// toggling unreliable). They register in the same Ginkgo suite as the
// external-package middleware tests.

var _ = Describe("Method 0 trusted proxy (Gin)", func() {
	var (
		router   *gin.Engine
		recorder *httptest.ResponseRecorder
		metrics  *internalMockMetrics
		identity IdentityValue
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		recorder = httptest.NewRecorder()
		metrics = &internalMockMetrics{}
	})

	AfterEach(func() {
		trustedProxySecretOverride = ""
	})

	buildRouter := func() {
		router = gin.New()
		mw, err := IdentityMiddlewareWithMetrics(nil, internalTestLogger(), metrics)
		Expect(err).NotTo(HaveOccurred())
		router.Use(mw)
		router.GET("/test", func(c *gin.Context) {
			identity = IdentityFromContext(c.Request.Context())
			c.Status(http.StatusOK)
		})
	}

	doRequest := func(req *http.Request) {
		router.ServeHTTP(recorder, req)
	}

	When("valid secret and login are provided", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			buildRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.Header.Set(TrustedProxyNameHeader, "Proxy User")
			req.RemoteAddr = "203.0.113.7:54321" // non-local, so only Method 0 applies
			doRequest(req)
		})

		It("asserts the identity from the proxy headers", func() {
			Expect(identity.IsAnonymous()).To(BeFalse())
		})

		It("records the login name", func() {
			Expect(identity.LoginName()).To(Equal("proxy@example.com"))
		})

		It("records the display name", func() {
			Expect(identity.DisplayName()).To(Equal("Proxy User"))
		})

		It("records a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(1))
		})
	})

	When("the secret is configured but the request presents a wrong secret", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			buildRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set(TrustedProxySecretHeader, "guess")
			req.Header.Set(TrustedProxyLoginHeader, "attacker@example.com")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("leaves the identity anonymous", func() {
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})

	When("the secret is not configured (env unset)", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = ""
			buildRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("ignores the trusted-proxy headers and stays anonymous", func() {
			// No localhost Tailscale headers, no resolver: Method 0 off, other
			// methods cannot fire, so the identity decision falls through.
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})

	When("Method 0 and localhost Tailscale-User-Login both apply", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			buildRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.Header.Set(TrustedProxyNameHeader, "Proxy User")
			req.Header.Set("Tailscale-User-Login", "tailscale@example.com")
			req.Header.Set("Tailscale-User-Name", "Tailscale User")
			req.RemoteAddr = "127.0.0.1:12345"
			doRequest(req)
		})

		It("prefers the trusted-proxy identity over localhost Tailscale headers", func() {
			Expect(identity.LoginName()).To(Equal("proxy@example.com"))
			Expect(identity.DisplayName()).To(Equal("Proxy User"))
		})

		It("records exactly one header extraction", func() {
			Expect(metrics.extractionCalls).To(Equal(1))
		})
	})

	When("a valid secret is present but the login header is empty", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			buildRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("does not establish an identity from Method 0", func() {
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})
})

var _ = Describe("Method 0 trusted proxy (plain http)", func() {
	var (
		recorder *httptest.ResponseRecorder
		metrics  *internalMockMetrics
		identity IdentityValue
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		metrics = &internalMockMetrics{}
	})

	AfterEach(func() {
		trustedProxySecretOverride = ""
	})

	buildHandler := func() http.Handler {
		h, err := IdentityHTTPMiddlewareWithMetrics(nil, internalTestLogger(), metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity = IdentityFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		Expect(err).NotTo(HaveOccurred())
		return h
	}

	doRequest := func(req *http.Request) {
		buildHandler().ServeHTTP(recorder, req)
	}

	When("valid secret and login are provided", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			req, _ := http.NewRequest("GET", "/mcp", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.Header.Set(TrustedProxyNameHeader, "Proxy User")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("asserts the identity from the proxy headers", func() {
			Expect(identity.IsAnonymous()).To(BeFalse())
		})

		It("records the login name", func() {
			Expect(identity.LoginName()).To(Equal("proxy@example.com"))
		})

		It("records the display name", func() {
			Expect(identity.DisplayName()).To(Equal("Proxy User"))
		})

		It("records a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(1))
		})
	})

	When("the secret is configured but the request presents a wrong secret", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			req, _ := http.NewRequest("GET", "/mcp", nil)
			req.Header.Set(TrustedProxySecretHeader, "guess")
			req.Header.Set(TrustedProxyLoginHeader, "attacker@example.com")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("leaves the identity anonymous", func() {
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})

	When("the secret is not configured (env unset)", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = ""
			req, _ := http.NewRequest("GET", "/mcp", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("ignores the trusted-proxy headers and stays anonymous", func() {
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})

	When("Method 0 and localhost Tailscale-User-Login both apply", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			req, _ := http.NewRequest("GET", "/mcp", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.Header.Set(TrustedProxyLoginHeader, "proxy@example.com")
			req.Header.Set(TrustedProxyNameHeader, "Proxy User")
			req.Header.Set("Tailscale-User-Login", "tailscale@example.com")
			req.Header.Set("Tailscale-User-Name", "Tailscale User")
			req.RemoteAddr = "127.0.0.1:12345"
			doRequest(req)
		})

		It("prefers the trusted-proxy identity over localhost Tailscale headers", func() {
			Expect(identity.LoginName()).To(Equal("proxy@example.com"))
			Expect(identity.DisplayName()).To(Equal("Proxy User"))
		})

		It("records exactly one header extraction", func() {
			Expect(metrics.extractionCalls).To(Equal(1))
		})
	})

	When("a valid secret is present but the login header is empty", func() {
		BeforeEach(func() {
			trustedProxySecretOverride = "topsecret"
			req, _ := http.NewRequest("GET", "/mcp", nil)
			req.Header.Set(TrustedProxySecretHeader, "topsecret")
			req.RemoteAddr = "203.0.113.7:54321"
			doRequest(req)
		})

		It("does not establish an identity from Method 0", func() {
			Expect(identity.IsAnonymous()).To(BeTrue())
		})

		It("does not record a header extraction metric", func() {
			Expect(metrics.extractionCalls).To(Equal(0))
		})
	})
})

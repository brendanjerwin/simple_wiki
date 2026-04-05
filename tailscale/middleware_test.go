package tailscale_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("IdentityMiddleware", func() {
	var (
		router   *gin.Engine
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		recorder = httptest.NewRecorder()
	})

	Describe("constructor validation", func() {
		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.IdentityMiddlewareWithMetrics(nil, nil, &mockMetricsRecorder{})
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})

		When("metrics is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.IdentityMiddlewareWithMetrics(nil, testLogger(), nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("metrics is required"))
			})
		})
	})

	Describe("extracting identity from headers", func() {
		When("Tailscale-User-Login header is present from localhost", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
			)

			BeforeEach(func() {
				metrics = &mockMetricsRecorder{}
				router = gin.New()
				middleware, err := tailscale.IdentityMiddlewareWithMetrics(nil, testLogger(), metrics)
				Expect(err).NotTo(HaveOccurred())
				router.Use(middleware)
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Tailscale-User-Login", "user@example.com")
				req.Header.Set("Tailscale-User-Name", "Test User")
				req.RemoteAddr = "127.0.0.1:12345" // Headers only trusted from localhost
				router.ServeHTTP(recorder, req)
			})

			It("should extract the identity from headers", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(capturedIdentity.DisplayName()).To(Equal("Test User"))
			})

			It("should record header extraction metric", func() {
				Expect(metrics.extractionCalls).To(Equal(1))
			})
		})

		When("Tailscale-User-Login header is present without display name from localhost", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				router = gin.New()
				middleware, err := tailscale.IdentityMiddlewareWithMetrics(nil, testLogger(), &mockMetricsRecorder{})
				Expect(err).NotTo(HaveOccurred())
				router.Use(middleware)
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Tailscale-User-Login", "user@example.com")
				req.RemoteAddr = "127.0.0.1:12345" // Headers only trusted from localhost
				router.ServeHTTP(recorder, req)
			})

			It("should extract the identity", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("user@example.com"))
			})

			It("should have empty display name", func() {
				Expect(capturedIdentity.DisplayName()).To(BeEmpty())
			})
		})
	})

	Describe("falling back to WhoIs", func() {
		When("no headers but WhoIs returns identity", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "WhoIs User", "my-laptop"),
					err:      nil,
				}
				metrics = &mockMetricsRecorder{}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddlewareWithMetrics(resolver, testLogger(), metrics)
				Expect(err).NotTo(HaveOccurred())
				router.Use(middleware)
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should get identity from WhoIs", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name from WhoIs", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("whois@example.com"))
			})

			It("should have the node name from WhoIs", func() {
				Expect(capturedIdentity.NodeName()).To(Equal("my-laptop"))
			})

			It("should record success lookup metric", func() {
				Expect(metrics.lookupCalls).To(Equal(1))
			})
		})

		When("no headers and WhoIs returns Anonymous", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.Anonymous,
					err:      nil,
				}
				metrics = &mockMetricsRecorder{}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddlewareWithMetrics(resolver, testLogger(), metrics)
				Expect(err).NotTo(HaveOccurred())
				router.Use(middleware)
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should have anonymous identity in context", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeTrue())
			})

			It("should record not-tailnet lookup metric", func() {
				Expect(metrics.lookupCalls).To(Equal(1))
			})
		})
	})

	Describe("header priority over WhoIs", func() {
		When("headers are present from localhost and WhoIs would return different identity", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "", "my-laptop"),
					err:      nil,
				}
				metrics = &mockMetricsRecorder{}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddlewareWithMetrics(resolver, testLogger(), metrics)
				Expect(err).NotTo(HaveOccurred())
				router.Use(middleware)
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Tailscale-User-Login", "header@example.com")
				req.RemoteAddr = "127.0.0.1:12345" // Headers only trusted from localhost
				router.ServeHTTP(recorder, req)
			})

			It("should prefer header identity over WhoIs", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("header@example.com"))
			})

			It("should record header extraction not WhoIs lookup", func() {
				Expect(metrics.extractionCalls).To(Equal(1))
				Expect(metrics.lookupCalls).To(Equal(0))
			})
		})
	})
})

var _ = Describe("IdentityHTTPMiddlewareWithMetrics", func() {
	Describe("constructor validation", func() {
		When("logger is nil", func() {
			It("should return an error", func() {
				_, err := tailscale.IdentityHTTPMiddlewareWithMetrics(nil, nil, &mockMetricsRecorder{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// No-op test stub — not needed for this test scenario
				}))
				Expect(err).To(MatchError("logger is required"))
			})
		})

		When("metrics is nil", func() {
			It("should return an error", func() {
				_, err := tailscale.IdentityHTTPMiddlewareWithMetrics(nil, testLogger(), nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// No-op test stub — not needed for this test scenario
				}))
				Expect(err).To(MatchError("metrics is required"))
			})
		})

		When("next handler is nil", func() {
			It("should return an error", func() {
				_, err := tailscale.IdentityHTTPMiddlewareWithMetrics(nil, testLogger(), &mockMetricsRecorder{}, nil)
				Expect(err).To(MatchError("next handler is required"))
			})
		})
	})

	Describe("extracting identity from headers", func() {
		When("Tailscale-User-Login header is present from localhost", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
				recorder         *httptest.ResponseRecorder
			)

			BeforeEach(func() {
				metrics = &mockMetricsRecorder{}
				recorder = httptest.NewRecorder()

				handler, err := tailscale.IdentityHTTPMiddlewareWithMetrics(nil, testLogger(), metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedIdentity = tailscale.IdentityFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				}))
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Tailscale-User-Login", "user@example.com")
				req.Header.Set("Tailscale-User-Name", "Test User")
				req.RemoteAddr = "127.0.0.1:12345"
				handler.ServeHTTP(recorder, req)
			})

			It("should extract the identity from headers", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(capturedIdentity.DisplayName()).To(Equal("Test User"))
			})

			It("should record header extraction metric", func() {
				Expect(metrics.extractionCalls).To(Equal(1))
			})
		})
	})

	Describe("falling back to WhoIs", func() {
		When("no headers but WhoIs returns identity", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				metrics          *mockMetricsRecorder
				recorder         *httptest.ResponseRecorder
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "WhoIs User", "my-laptop"),
				}
				metrics = &mockMetricsRecorder{}
				recorder = httptest.NewRecorder()

				handler, err := tailscale.IdentityHTTPMiddlewareWithMetrics(resolver, testLogger(), metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedIdentity = tailscale.IdentityFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				}))
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should get identity from WhoIs", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name from WhoIs", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("whois@example.com"))
			})

			It("should record success lookup metric", func() {
				Expect(metrics.lookupCalls).To(Equal(1))
			})
		})
	})

	Describe("anonymous fallback", func() {
		When("no headers and no resolver", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				recorder         *httptest.ResponseRecorder
			)

			BeforeEach(func() {
				recorder = httptest.NewRecorder()

				handler, err := tailscale.IdentityHTTPMiddlewareWithMetrics(nil, testLogger(), &mockMetricsRecorder{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedIdentity = tailscale.IdentityFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				}))
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should have anonymous identity in context", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeTrue())
			})

			It("should still call the next handler", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
})

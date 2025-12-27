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
				_, err = tailscale.IdentityMiddleware(nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})
	})

	Describe("extracting identity from headers", func() {
		When("Tailscale-User-Login header is present from localhost", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				router = gin.New()
				middleware, err := tailscale.IdentityMiddleware(nil, testLogger())
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
		})

		When("Tailscale-User-Login header is present without display name from localhost", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				router = gin.New()
				middleware, err := tailscale.IdentityMiddleware(nil, testLogger())
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
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "WhoIs User", "my-laptop"),
					err:      nil,
				}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddleware(resolver, testLogger())
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
		})

		When("no headers and WhoIs returns Anonymous", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.Anonymous,
					err:      nil,
				}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddleware(resolver, testLogger())
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
		})
	})

	Describe("header priority over WhoIs", func() {
		When("headers are present from localhost and WhoIs would return different identity", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "", "my-laptop"),
					err:      nil,
				}

				router = gin.New()
				middleware, err := tailscale.IdentityMiddleware(resolver, testLogger())
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
		})
	})
})

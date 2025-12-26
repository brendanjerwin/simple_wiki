package tailscale_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// mockIdentityResolver implements IdentityResolver for testing.
type mockIdentityResolver struct {
	identity *tailscale.Identity
	err      error
}

func (m *mockIdentityResolver) WhoIs(_ context.Context, _ string) (*tailscale.Identity, error) {
	return m.identity, m.err
}

var _ = Describe("IdentityMiddleware", func() {
	var (
		router   *gin.Engine
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		recorder = httptest.NewRecorder()
	})

	Describe("extracting identity from headers", func() {
		When("Tailscale-User-Login header is present from localhost", func() {
			var (
				capturedIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(nil, nil))
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
				Expect(capturedIdentity).NotTo(BeNil())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName).To(Equal("user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(capturedIdentity.DisplayName).To(Equal("Test User"))
			})
		})

		When("Tailscale-User-Login header is present without display name from localhost", func() {
			var (
				capturedIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(nil, nil))
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
				Expect(capturedIdentity).NotTo(BeNil())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName).To(Equal("user@example.com"))
			})

			It("should have empty display name", func() {
				Expect(capturedIdentity.DisplayName).To(BeEmpty())
			})
		})
	})

	Describe("falling back to WhoIs", func() {
		When("no headers but WhoIs returns identity", func() {
			var (
				capturedIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: &tailscale.Identity{
						LoginName:   "whois@example.com",
						DisplayName: "WhoIs User",
						NodeName:    "my-laptop",
					},
					err: nil,
				}

				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(resolver, nil))
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should get identity from WhoIs", func() {
				Expect(capturedIdentity).NotTo(BeNil())
			})

			It("should have the correct login name from WhoIs", func() {
				Expect(capturedIdentity.LoginName).To(Equal("whois@example.com"))
			})

			It("should have the node name from WhoIs", func() {
				Expect(capturedIdentity.NodeName).To(Equal("my-laptop"))
			})
		})

		When("no headers and WhoIs returns nil", func() {
			var (
				capturedIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: nil,
					err:      nil,
				}

				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(resolver, nil))
				router.GET("/test", func(c *gin.Context) {
					capturedIdentity = tailscale.IdentityFromContext(c.Request.Context())
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should have no identity in context", func() {
				Expect(capturedIdentity).To(BeNil())
			})
		})
	})

	Describe("header priority over WhoIs", func() {
		When("headers are present from localhost and WhoIs would return different identity", func() {
			var (
				capturedIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: &tailscale.Identity{
						LoginName: "whois@example.com",
						NodeName:  "my-laptop",
					},
					err: nil,
				}

				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(resolver, nil))
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
				Expect(capturedIdentity.LoginName).To(Equal("header@example.com"))
			})
		})
	})

	Describe("Gin context storage", func() {
		When("identity is extracted", func() {
			var (
				ginIdentity *tailscale.Identity
			)

			BeforeEach(func() {
				router = gin.New()
				router.Use(tailscale.IdentityMiddleware(nil, nil))
				router.GET("/test", func(c *gin.Context) {
					if val, exists := c.Get("tailscale-identity"); exists {
						if identity, ok := val.(*tailscale.Identity); ok {
							ginIdentity = identity
						}
					}
					c.Status(http.StatusOK)
				})

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Tailscale-User-Login", "user@example.com")
				req.RemoteAddr = "127.0.0.1:12345" // Headers only trusted from localhost
				router.ServeHTTP(recorder, req)
			})

			It("should also store identity in Gin context", func() {
				Expect(ginIdentity).NotTo(BeNil())
				Expect(ginIdentity.LoginName).To(Equal("user@example.com"))
			})
		})
	})
})

package tailscale_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("TailnetRedirector", func() {
	var (
		fallbackHandler http.Handler
		recorder        *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fallbackHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fallback"))
		})
		recorder = httptest.NewRecorder()
	})

	Describe("NewRedirectHandler", func() {
		When("creating a new redirect handler with valid parameters", func() {
			var (
				handler *tailscale.TailnetRedirector
				err     error
			)

			BeforeEach(func() {
				handler, err = tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, false, nil)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not be nil", func() {
				Expect(handler).NotTo(BeNil())
			})
		})

		When("tsHostname is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.NewTailnetRedirector("", 443, nil, fallbackHandler, false, nil)
			})

			It("should return an error about empty hostname", func() {
				Expect(err).To(MatchError("tsHostname cannot be empty"))
			})
		})

		When("tlsPort is zero", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 0, nil, fallbackHandler, false, nil)
			})

			It("should return an error about invalid port", func() {
				Expect(err).To(MatchError("tlsPort 0 is invalid: must be between 1 and 65535"))
			})
		})

		When("tlsPort is negative", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", -1, nil, fallbackHandler, false, nil)
			})

			It("should return an error about invalid port", func() {
				Expect(err).To(MatchError("tlsPort -1 is invalid: must be between 1 and 65535"))
			})
		})

		When("tlsPort exceeds 65535", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 70000, nil, fallbackHandler, false, nil)
			})

			It("should return an error about invalid port", func() {
				Expect(err).To(MatchError("tlsPort 70000 is invalid: must be between 1 and 65535"))
			})
		})

		When("fallback handler is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, nil, false, nil)
			})

			It("should return an error about nil fallback", func() {
				Expect(err).To(MatchError("fallback handler cannot be nil"))
			})
		})
	})

	Describe("X-Forwarded-Proto handling", func() {
		When("request has X-Forwarded-Proto: https from localhost", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				req.RemoteAddr = "127.0.0.1:12345"
				handler.ServeHTTP(recorder, req)
			})

			It("should not redirect", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should serve via fallback handler", func() {
				Expect(recorder.Body.String()).To(Equal("fallback"))
			})
		})

		When("request has X-Forwarded-Proto: https from IPv6 localhost", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				req.RemoteAddr = "[::1]:12345"
				handler.ServeHTTP(recorder, req)
			})

			It("should not redirect", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should serve via fallback handler", func() {
				Expect(recorder.Body.String()).To(Equal("fallback"))
			})
		})

		When("request has X-Forwarded-Proto: https from external IP (spoofed header)", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				req.RemoteAddr = "192.168.1.100:12345"
				handler.ServeHTTP(recorder, req)
			})

			It("should ignore the spoofed header and redirect", func() {
				Expect(recorder.Code).To(Equal(http.StatusMovedPermanently))
			})

			It("should redirect to tailnet HTTPS", func() {
				location := recorder.Header().Get("Location")
				Expect(location).To(Equal("https://my-laptop.tailnet.ts.net/test"))
			})
		})

		When("request has X-Forwarded-Proto: http from localhost", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-Proto", "http")
				req.RemoteAddr = "127.0.0.1:12345"
				handler.ServeHTTP(recorder, req)
			})

			It("should redirect to HTTPS", func() {
				Expect(recorder.Code).To(Equal(http.StatusMovedPermanently))
			})
		})
	})

	Describe("ForceRedirectToTailnet mode", func() {
		When("force redirect is enabled", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test?foo=bar", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should redirect to tailnet HTTPS", func() {
				Expect(recorder.Code).To(Equal(http.StatusMovedPermanently))
			})

			It("should redirect to the correct URL", func() {
				location := recorder.Header().Get("Location")
				Expect(location).To(Equal("https://my-laptop.tailnet.ts.net/test?foo=bar"))
			})
		})

		When("force redirect is enabled with non-standard port", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 8443, nil, fallbackHandler, true, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should include the port in the redirect URL", func() {
				location := recorder.Header().Get("Location")
				Expect(location).To(Equal("https://my-laptop.tailnet.ts.net:8443/test"))
			})
		})
	})

	Describe("WhoIs-based redirect", func() {
		When("force redirect is disabled and WhoIs returns identity", func() {
			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: &tailscale.Identity{
						LoginName: "user@example.com",
					},
					err: nil,
				}

				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, resolver, fallbackHandler, false, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should redirect to HTTPS", func() {
				Expect(recorder.Code).To(Equal(http.StatusMovedPermanently))
			})

			It("should redirect to the tailnet hostname", func() {
				location := recorder.Header().Get("Location")
				Expect(location).To(Equal("https://my-laptop.tailnet.ts.net/test"))
			})
		})

		When("force redirect is disabled and WhoIs returns nil", func() {
			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: nil,
					err:      nil,
				}

				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, resolver, fallbackHandler, false, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should not redirect", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should serve via fallback handler", func() {
				Expect(recorder.Body.String()).To(Equal("fallback"))
			})
		})
	})

	Describe("no resolver", func() {
		When("force redirect is disabled and no resolver is configured", func() {
			BeforeEach(func() {
				handler, err := tailscale.NewTailnetRedirector("my-laptop.tailnet.ts.net", 443, nil, fallbackHandler, false, nil)
				Expect(err).NotTo(HaveOccurred())

				req, _ := http.NewRequest("GET", "/test", nil)
				handler.ServeHTTP(recorder, req)
			})

			It("should serve via fallback handler", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal("fallback"))
			})
		})
	})
})

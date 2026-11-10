//revive:disable:dot-imports
package bootstrap_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
	"github.com/brendanjerwin/simple_wiki/server"
)

// headOnGetOnlyRoute pins RFC 9110 section 9.3.2 for the composed handler:
// a server that supports GET on a resource must support HEAD on it with the
// same header fields and no content. The "/" route is registered with
// router.GET only (server/handlers_web.go), and Gin does not synthesize HEAD
// from GET routes, so the fall-through to Gin used to answer HEAD "/" with 404.
var _ = Describe("HTTP handler chain", func() {
	Describe("answering HEAD on a GET-only route", func() {
		var getResponse *http.Response
		var getBody []byte
		var headResponse *http.Response
		var headBody []byte

		BeforeEach(func() {
			tmpDir, err := os.MkdirTemp("", "simple_wiki_head_test")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = os.RemoveAll(tmpDir) })

			site, err := server.NewSite(tmpDir, "testpage", 0, "secret", lumber.NewConsoleLogger(lumber.WARN))
			Expect(err).NotTo(HaveOccurred())

			grpcServer := grpc.NewServer()
			apiv1.RegisterFileStorageServiceServer(grpcServer, nil)
			apiv1.RegisterFrontmatterServer(grpcServer, nil)
			apiv1.RegisterInventoryManagementServiceServer(grpcServer, nil)
			apiv1.RegisterPageImportServiceServer(grpcServer, nil)
			apiv1.RegisterPageManagementServiceServer(grpcServer, nil)
			apiv1.RegisterSearchServiceServer(grpcServer, nil)
			apiv1.RegisterSystemInfoServiceServer(grpcServer, nil)

			handler, err := bootstrap.BuildVanguardTranscoder(grpcServer, site.GinRouter())
			Expect(err).NotTo(HaveOccurred())

			testServer := httptest.NewServer(handler)
			DeferCleanup(testServer.Close)

			// Redirects must be observed, not followed, so both verbs report the
			// status line the server actually produced for "/".
			client := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			getResponse, getBody = request(client, testServer.URL+"/", http.MethodGet)
			headResponse, headBody = request(client, testServer.URL+"/", http.MethodHead)
		})

		It("should answer GET with a redirect", func() {
			Expect(getResponse.StatusCode).To(Equal(http.StatusFound))
		})

		It("should answer HEAD with the same status code as GET", func() {
			Expect(headResponse.StatusCode).To(Equal(getResponse.StatusCode))
		})

		It("should answer HEAD with the same Location as GET", func() {
			Expect(headResponse.Header.Get("Location")).To(Equal(getResponse.Header.Get("Location")))
			Expect(headResponse.Header.Get("Location")).To(Equal("/testpage/view"))
		})

		It("should answer HEAD with the same Content-Type as GET", func() {
			Expect(headResponse.Header.Get("Content-Type")).To(Equal(getResponse.Header.Get("Content-Type")))
		})

		It("should answer GET with a body", func() {
			Expect(getBody).NotTo(BeEmpty())
		})

		It("should answer HEAD with an empty body", func() {
			Expect(headBody).To(BeEmpty())
		})
	})
})

// request performs one request and returns the response with its fully read body.
func request(client *http.Client, url string, method string) (*http.Response, []byte) {
	req, err := http.NewRequest(method, url, nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = resp.Body.Close() })

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, body
}

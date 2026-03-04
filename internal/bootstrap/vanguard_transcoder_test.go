package bootstrap_test

import (
	"net/http"
	"net/http/httptest"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("BuildVanguardTranscoder", func() {
	var grpcServer *grpc.Server
	var ginRouter http.Handler

	BeforeEach(func() {
		grpcServer = grpc.NewServer()
		apiv1.RegisterFileStorageServiceServer(grpcServer, nil)
		apiv1.RegisterFrontmatterServer(grpcServer, nil)
		apiv1.RegisterInventoryManagementServiceServer(grpcServer, nil)
		apiv1.RegisterPageImportServiceServer(grpcServer, nil)
		apiv1.RegisterPageManagementServiceServer(grpcServer, nil)
		apiv1.RegisterSearchServiceServer(grpcServer, nil)
		apiv1.RegisterSystemInfoServiceServer(grpcServer, nil)

		ginRouter = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	It("should create a handler without error", func() {
		handler, err := bootstrap.BuildVanguardTranscoder(grpcServer, ginRouter)

		Expect(err).NotTo(HaveOccurred())
		Expect(handler).NotTo(BeNil())
	})

	When("receiving a reflection request", func() {
		var ginFallbackCalled bool

		BeforeEach(func() {
			ginFallbackCalled = false
			trackingRouter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ginFallbackCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler, err := bootstrap.BuildVanguardTranscoder(grpcServer, trackingRouter)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodPost, "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo", nil)
			req.Header.Set("Content-Type", "application/proto")
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
		})

		It("should not fall through to the gin router", func() {
			Expect(ginFallbackCalled).To(BeFalse())
		})
	})

	When("receiving a non-RPC request", func() {
		var fallbackCalled bool
		var resp *httptest.ResponseRecorder

		BeforeEach(func() {
			fallbackCalled = false
			fallbackRouter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fallbackCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler, err := bootstrap.BuildVanguardTranscoder(grpcServer, fallbackRouter)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/some/page", nil)
			resp = httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
		})

		It("should fall through to the gin router", func() {
			Expect(fallbackCalled).To(BeTrue())
		})

		It("should return 200", func() {
			Expect(resp.Code).To(Equal(http.StatusOK))
		})
	})
})

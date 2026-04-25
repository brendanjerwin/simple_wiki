//revive:disable:dot-imports
package bootstrap_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
)

// transcoder coverage guard
//
// Added after a production bug where two new gRPC services
// (AgentMetadataService, ScheduledTurnService) were registered on the
// grpc.Server but missing from the vanguard transcoder's hard-coded
// serviceNames list. The gRPC transport worked, but Connect / gRPC-Web /
// HTTP requests fell through to the Gin router's 404 handler. Pre-existing
// transcoder tests didn't catch it because they verified routing for ONE
// service at a time, not coverage across the whole server.
//
// The guard takes the registry of "what's been Register'd on the grpc.Server"
// as the source of truth and asserts every one of those services is
// reachable via the transcoder. If a future RPC service is registered with
// the grpc.Server but forgotten in BuildVanguardTranscoder's serviceNames
// list, this test fails loudly.
var _ = Describe("BuildVanguardTranscoder service coverage", func() {
	var (
		grpcServer *grpc.Server
		handler    http.Handler
	)

	BeforeEach(func() {
		grpcServer = grpc.NewServer()
		// Register every api.v1.* service the production server exposes,
		// using the codegen-provided Unimplemented* types so handler
		// invocations return UNIMPLEMENTED rather than panicking on nil.
		apiv1.RegisterAgentMetadataServiceServer(grpcServer, apiv1.UnimplementedAgentMetadataServiceServer{})
		apiv1.RegisterChatServiceServer(grpcServer, apiv1.UnimplementedChatServiceServer{})
		apiv1.RegisterFileStorageServiceServer(grpcServer, apiv1.UnimplementedFileStorageServiceServer{})
		apiv1.RegisterFrontmatterServer(grpcServer, apiv1.UnimplementedFrontmatterServer{})
		apiv1.RegisterInventoryManagementServiceServer(grpcServer, apiv1.UnimplementedInventoryManagementServiceServer{})
		apiv1.RegisterPageImportServiceServer(grpcServer, apiv1.UnimplementedPageImportServiceServer{})
		apiv1.RegisterPageManagementServiceServer(grpcServer, apiv1.UnimplementedPageManagementServiceServer{})
		apiv1.RegisterScheduledTurnServiceServer(grpcServer, apiv1.UnimplementedScheduledTurnServiceServer{})
		apiv1.RegisterSearchServiceServer(grpcServer, apiv1.UnimplementedSearchServiceServer{})
		apiv1.RegisterSystemInfoServiceServer(grpcServer, apiv1.UnimplementedSystemInfoServiceServer{})

		// Fall-through handler returns 404 — our marker for "transcoder
		// didn't route this request to gRPC". Any non-404 response (including
		// 5xx from an Unimplemented handler) means routing worked.
		ginRouter := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, &http.Request{})
		})

		var err error
		handler, err = bootstrap.BuildVanguardTranscoder(grpcServer, ginRouter)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("for every service registered with the grpc.Server", func() {
		It("should route Connect requests to gRPC (not fall through to the 404 handler)", func() {
			info := grpcServer.GetServiceInfo()
			missingServices := []string{}
			for serviceName, svc := range info {
				// The reflection service is intentionally routed via the
				// reflector mux ahead of the transcoder — skip it.
				if strings.HasPrefix(serviceName, "grpc.reflection.") {
					continue
				}
				if len(svc.Methods) == 0 {
					continue
				}
				method := svc.Methods[0].Name
				path := "/" + serviceName + "/" + method
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
				// Connect's HTTP/JSON transport requires this header — without
				// it, vanguard treats the request as not-Connect and
				// (depending on Content-Type) may fall through to the unknown
				// handler.
				req.Header.Set("Connect-Protocol-Version", "1")
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, req)
				if resp.Code == http.StatusNotFound {
					missingServices = append(missingServices, serviceName)
				}
			}
			Expect(missingServices).To(BeEmpty(),
				"these services are registered with the grpc.Server but the vanguard transcoder is not routing to them — likely missing from BuildVanguardTranscoder's serviceNames list: %v",
				missingServices)
		})
	})
})

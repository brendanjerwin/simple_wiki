//revive:disable:dot-imports
package bootstrap_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
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
// The guard has two parts:
//
//   1. A routing test that takes "what's Register'd on grpcServer" as ground
//      truth and asserts every one of those services is reachable through
//      the transcoder.
//
//   2. A parity test that asserts the fixture's manual Unimplemented-stub
//      registrations match what production's v1.Server.RegisterWithServer
//      registers — so a new service in production cannot drift away from
//      the fixture. (The routing test alone wouldn't catch this: if both
//      lists are missing the same service, both stay quietly green.)
//
// Why two registrations instead of using production wiring directly: the
// real Server overrides Unimplemented* methods on every embedded service,
// so a zero-value Server.GetState would dereference its nil keepConnector
// and panic when the routing test actually dispatches a request. The
// fixture uses pure Unimplemented* stubs which return UNIMPLEMENTED
// cleanly (not 404). The parity test bridges the two.
var _ = Describe("BuildVanguardTranscoder service coverage", func() {
	var (
		grpcServer *grpc.Server
		handler    http.Handler
	)

	BeforeEach(func() {
		grpcServer = grpc.NewServer()
		// Manually register Unimplemented* stubs. A new gRPC service must
		// be added here AND to v1.Server.RegisterWithServer; the parity
		// test below catches that drift.
		apiv1.RegisterAgentMetadataServiceServer(grpcServer, apiv1.UnimplementedAgentMetadataServiceServer{})
		apiv1.RegisterChatServiceServer(grpcServer, apiv1.UnimplementedChatServiceServer{})
		apiv1.RegisterChecklistServiceServer(grpcServer, apiv1.UnimplementedChecklistServiceServer{})
		apiv1.RegisterFileStorageServiceServer(grpcServer, apiv1.UnimplementedFileStorageServiceServer{})
		apiv1.RegisterFrontmatterServer(grpcServer, apiv1.UnimplementedFrontmatterServer{})
		apiv1.RegisterInventoryManagementServiceServer(grpcServer, apiv1.UnimplementedInventoryManagementServiceServer{})
		apiv1.RegisterKeepConnectorServiceServer(grpcServer, apiv1.UnimplementedKeepConnectorServiceServer{})
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

	Describe("fixture parity with production wiring", func() {
		It("should register the same service set that v1.Server.RegisterWithServer registers", func() {
			fixtureNames := serviceNames(grpcServer)

			productionServer := grpc.NewServer()
			// A zero-value v1.Server satisfies every apiv1.*ServiceServer
			// interface via its embedded Unimplemented* types. We never
			// invoke a handler here — Register* just stashes the impl.
			(&v1.Server{}).RegisterWithServer(productionServer)
			productionNames := serviceNames(productionServer)

			Expect(fixtureNames).To(Equal(productionNames),
				"Fixture coverage drifted from production. v1.Server.RegisterWithServer registers a service the fixture does not (or vice versa). Sync the fixture above with the RegisterWithServer call.")
		})
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

// serviceNames returns the set of fully-qualified gRPC service names a
// grpc.Server has had Register-d to it, sorted for stable comparison.
// Skips reflection/health meta-services which aren't wired through the
// vanguard transcoder.
func serviceNames(srv *grpc.Server) []string {
	info := srv.GetServiceInfo()
	out := make([]string, 0, len(info))
	for name := range info {
		if strings.HasPrefix(name, "grpc.reflection.") || strings.HasPrefix(name, "grpc.health.") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

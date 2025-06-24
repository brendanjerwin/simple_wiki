package debug

import (
	"context"
	"time"

	debugv1 "github.com/brendanjerwin/simple_wiki/gen/go/server/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server is the implementation of the DebugService.
type Server struct {
	debugv1.UnimplementedDebugServiceServer
	Version   string
	Commit    string
	BuildTime time.Time
}

// NewServer creates a new debug server.
func NewServer(version, commit string, buildTime time.Time) *Server {
	return &Server{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	}
}

// RegisterWithServer registers the debug service with the given gRPC server.
func (s *Server) RegisterWithServer(grpcServer *grpc.Server) {
	debugv1.RegisterDebugServiceServer(grpcServer, s)
}

// GetVersion implements the GetVersion RPC.
func (s *Server) GetVersion(ctx context.Context, req *debugv1.GetVersionRequest) (*debugv1.GetVersionResponse, error) {
	return &debugv1.GetVersionResponse{
		Version:   s.Version,
		Commit:    s.Commit,
		BuildTime: timestamppb.New(s.BuildTime),
	}, nil
}

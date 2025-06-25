package v1

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/common"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

 // Server is the implementation of the gRPC services.
type Server struct {
	apiv1.UnimplementedVersionServer
	apiv1.UnimplementedFrontmatterServer
	Version        string
	Commit         string
	BuildTime      time.Time
	PageReadWriter common.PageReadWriter
}

// ReplaceFrontmatter implements the ReplaceFrontmatter RPC.
func (s *Server) ReplaceFrontmatter(ctx context.Context, req *apiv1.ReplaceFrontmatterRequest) (*apiv1.ReplaceFrontmatterResponse, error) {
	if s.PageReadWriter == nil {
		return nil, status.Error(codes.Internal, "PageReadWriter not available")
	}

	fm := req.Frontmatter.AsMap()

	err := s.PageReadWriter.WriteFrontMatter(req.Page, fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	return &apiv1.ReplaceFrontmatterResponse{
		Frontmatter: req.Frontmatter,
	}, nil
}

// NewServer creates a new debug server.
func NewServer(version, commit string, buildTime time.Time, pageReadWriter common.PageReadWriter) *Server {
	return &Server{
		Version:        version,
		Commit:         commit,
		BuildTime:      buildTime,
		PageReadWriter: pageReadWriter,
	}
}

// RegisterWithServer registers the gRPC services with the given gRPC server.
func (s *Server) RegisterWithServer(grpcServer *grpc.Server) {
	apiv1.RegisterVersionServer(grpcServer, s)
	apiv1.RegisterFrontmatterServer(grpcServer, s)
}

// GetVersion implements the GetVersion RPC.
func (s *Server) GetVersion(ctx context.Context, req *apiv1.GetVersionRequest) (*apiv1.GetVersionResponse, error) {
	return &apiv1.GetVersionResponse{
		Version:   s.Version,
		Commit:    s.Commit,
		BuildTime: timestamppb.New(s.BuildTime),
	}, nil
}

 // GetFrontmatter implements the GetFrontmatter RPC.
func (s *Server) GetFrontmatter(ctx context.Context, req *apiv1.GetFrontmatterRequest) (*apiv1.GetFrontmatterResponse, error) {
	if s.PageReadWriter == nil {
		return nil, status.Error(codes.Internal, "PageReadWriter not available")
	}

	_, fm, err := s.PageReadWriter.ReadFrontMatter(req.Page)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "page not found: %s", req.Page)
	}

	structFm, err := structpb.NewStruct(fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
	}

	return &apiv1.GetFrontmatterResponse{
		Frontmatter: structFm,
	}, nil
}

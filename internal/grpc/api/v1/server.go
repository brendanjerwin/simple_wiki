package v1

import (
	"context"
	"maps"
	"os"
	"reflect"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const pageReadWriterNotAvailableError = "PageReadWriter not available"

// Server is the implementation of the gRPC services.
type Server struct {
	apiv1.UnimplementedVersionServer
	apiv1.UnimplementedFrontmatterServer
	Version        string
	Commit         string
	BuildTime      time.Time
	PageReadWriter wikipage.PageReadWriter
}

// MergeFrontmatter implements the MergeFrontmatter RPC.
func (s *Server) MergeFrontmatter(_ context.Context, req *apiv1.MergeFrontmatterRequest) (resp *apiv1.MergeFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReadWriter)
	if s.PageReadWriter == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	_, existingFm, err := s.PageReadWriter.ReadFrontMatter(req.Page)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to read frontmatter: %v", err)
	}

	if existingFm == nil {
		existingFm = make(map[string]any)
	}

	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		maps.Copy(existingFm, newFm)
	}

	err = s.PageReadWriter.WriteFrontMatter(req.Page, existingFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	mergedFmStruct, err := structpb.NewStruct(existingFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert merged frontmatter to struct: %v", err)
	}

	return &apiv1.MergeFrontmatterResponse{
		Frontmatter: mergedFmStruct,
	}, nil
}

// ReplaceFrontmatter implements the ReplaceFrontmatter RPC.
func (s *Server) ReplaceFrontmatter(_ context.Context, req *apiv1.ReplaceFrontmatterRequest) (resp *apiv1.ReplaceFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReadWriter)
	if s.PageReadWriter == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	var fm map[string]any
	if req.Frontmatter != nil {
		fm = req.Frontmatter.AsMap()
	}

	err = s.PageReadWriter.WriteFrontMatter(req.Page, fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	return &apiv1.ReplaceFrontmatterResponse{
		Frontmatter: req.Frontmatter,
	}, nil
}

// RemoveKeyAtPath implements the RemoveKeyAtPath RPC.
func (s *Server) RemoveKeyAtPath(_ context.Context, req *apiv1.RemoveKeyAtPathRequest) (*apiv1.RemoveKeyAtPathResponse, error) {
	v := reflect.ValueOf(s.PageReadWriter)
	if s.PageReadWriter == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	if len(req.GetKeyPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "key_path cannot be empty")
	}

	_, fm, err := s.PageReadWriter.ReadFrontMatter(req.Page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page not found: %s", req.Page)
		}
		return nil, status.Errorf(codes.Internal, "failed to read frontmatter: %v", err)
	}

	if fm == nil {
		// Attempting to remove from a non-existent frontmatter. The path will not be found.
		fm = make(map[string]any)
	}

	updatedFm, err := removeAtPath(fm, req.GetKeyPath())
	if err != nil {
		return nil, err
	}

	err = s.PageReadWriter.WriteFrontMatter(req.Page, updatedFm.(map[string]any))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	updatedFmStruct, err := structpb.NewStruct(updatedFm.(map[string]any))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert updated frontmatter to struct: %v", err)
	}

	return &apiv1.RemoveKeyAtPathResponse{
		Frontmatter: updatedFmStruct,
	}, nil
}

// removeAtPath recursively traverses the data structure according to the path
// and removes the element at the end of the path. It returns the modified data
// structure. For slices, this may be a new slice instance.
func removeAtPath(data any, path []*apiv1.PathComponent) (any, error) {
	if len(path) == 0 {
		// This should be caught by the public-facing method, but as a safeguard:
		return nil, status.Error(codes.InvalidArgument, "path cannot be empty")
	}

	component := path[0]
	remainingPath := path[1:]

	switch v := data.(type) {
	case map[string]any:
		keyComp, ok := component.Component.(*apiv1.PathComponent_Key)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "path component is not a key for a map: %T", component.Component)
		}
		key := keyComp.Key

		value, exists := v[key]
		if !exists {
			return nil, status.Errorf(codes.NotFound, "key '%s' not found", key)
		}

		if len(remainingPath) == 0 {
			// Base case: remove key from map
			delete(v, key)
			return v, nil // return modified map
		}

		// Recursive step
		newValue, err := removeAtPath(value, remainingPath)
		if err != nil {
			return nil, err
		}
		v[key] = newValue // Update map with potentially modified child.
		return v, nil

	case []any:
		indexComp, ok := component.Component.(*apiv1.PathComponent_Index)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "path component is not an index for a slice: %T", component.Component)
		}
		idx := int(indexComp.Index)

		if idx < 0 || idx >= len(v) {
			return nil, status.Errorf(codes.OutOfRange, "index %d is out of range for slice of length %d", idx, len(v))
		}

		if len(remainingPath) == 0 {
			// Base case: remove item from slice
			newSlice := append(v[:idx], v[idx+1:]...)
			return newSlice, nil // Return the new slice
		}

		// Recursive step
		value := v[idx]
		newValue, err := removeAtPath(value, remainingPath)
		if err != nil {
			return nil, err
		}
		v[idx] = newValue // Update slice with potentially modified child.
		return v, nil

	default:
		// Trying to traverse deeper, but `data` is a primitive.
		return nil, status.Error(codes.InvalidArgument, "path is deeper than data structure")
	}
}

// NewServer creates a new debug server.
func NewServer(version, commit string, buildTime time.Time, pageReadWriter wikipage.PageReadWriter) *Server {
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
func (s *Server) GetVersion(_ context.Context, _ *apiv1.GetVersionRequest) (*apiv1.GetVersionResponse, error) {
	return &apiv1.GetVersionResponse{
		Version:   s.Version,
		Commit:    s.Commit,
		BuildTime: timestamppb.New(s.BuildTime),
	}, nil
}

// GetFrontmatter implements the GetFrontmatter RPC.
func (s *Server) GetFrontmatter(_ context.Context, req *apiv1.GetFrontmatterRequest) (resp *apiv1.GetFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReadWriter)
	if s.PageReadWriter == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	var fm map[string]any
	_, fm, err = s.PageReadWriter.ReadFrontMatter(req.Page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page not found: %s", req.Page)
		}
		return nil, status.Errorf(codes.Internal, "failed to read frontmatter: %v", err)
	}

	var structFm *structpb.Struct
	structFm, err = structpb.NewStruct(fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
	}

	return &apiv1.GetFrontmatterResponse{
		Frontmatter: structFm,
	}, nil
}

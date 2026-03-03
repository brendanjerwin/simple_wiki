package v1

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/filestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UploadFile implements the UploadFile RPC.
func (s *Server) UploadFile(_ context.Context, req *apiv1.UploadFileRequest) (*apiv1.UploadFileResponse, error) {
	if !s.fileUploadsEnabled {
		return nil, status.Error(codes.FailedPrecondition, "file uploads are disabled on this server")
	}
	if len(req.GetContent()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	if req.GetFilename() == "" {
		return nil, status.Error(codes.InvalidArgument, "filename is required")
	}

	info, err := s.fileStorer.Store(bytes.NewReader(req.GetContent()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store file: %v", err)
	}

	uploadURL := "/uploads/" + info.Hash + "?filename=" + url.QueryEscape(req.GetFilename())
	return &apiv1.UploadFileResponse{
		Hash:      info.Hash,
		UploadUrl: uploadURL,
	}, nil
}

// GetFileInfo implements the GetFileInfo RPC.
func (s *Server) GetFileInfo(_ context.Context, req *apiv1.GetFileInfoRequest) (*apiv1.GetFileInfoResponse, error) {
	if !s.fileUploadsEnabled {
		return nil, status.Error(codes.FailedPrecondition, "file uploads are disabled on this server")
	}
	if req.GetHash() == "" {
		return nil, status.Error(codes.InvalidArgument, "hash is required")
	}

	info, err := s.fileStorer.GetInfo(req.GetHash())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "file not found: %s", req.GetHash())
		}
		if errors.Is(err, filestore.ErrInvalidHash) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get file info: %v", err)
	}

	return &apiv1.GetFileInfoResponse{
		Hash:      info.Hash,
		SizeBytes: info.SizeBytes,
	}, nil
}

// DeleteFile implements the DeleteFile RPC.
func (s *Server) DeleteFile(_ context.Context, req *apiv1.DeleteFileRequest) (*apiv1.DeleteFileResponse, error) {
	if !s.fileUploadsEnabled {
		return nil, status.Error(codes.FailedPrecondition, "file uploads are disabled on this server")
	}
	if req.GetHash() == "" {
		return nil, status.Error(codes.InvalidArgument, "hash is required")
	}

	err := s.fileStorer.Delete(req.GetHash())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "file not found: %s", req.GetHash())
		}
		if errors.Is(err, filestore.ErrInvalidHash) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete file: %v", err)
	}

	return &apiv1.DeleteFileResponse{Success: true}, nil
}

//revive:disable:dot-imports
package v1_test

import (
	"context"
	"io"
	"os"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/filestore"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
)

// mockFileStorer is a test double for filestore.FileStorer.
type mockFileStorer struct {
	StoreFunc   func(content io.Reader, filename string) (filestore.FileInfo, error)
	GetInfoFunc func(hash string) (filestore.FileInfo, error)
	DeleteFunc  func(hash string) error
}

func (m *mockFileStorer) Store(content io.Reader, filename string) (filestore.FileInfo, error) {
	return m.StoreFunc(content, filename)
}

func (m *mockFileStorer) GetInfo(hash string) (filestore.FileInfo, error) {
	return m.GetInfoFunc(hash)
}

func (m *mockFileStorer) Delete(hash string) error {
	return m.DeleteFunc(hash)
}

func mustNewServerWithFileStorer(
	fileStorer filestore.FileStorer,
	fileUploadsEnabled bool,
) *v1.Server {
	server, err := v1.NewServer(
		"commit",
		time.Now(),
		noOpPageReaderMutator{},
		noOpBleveIndexQueryer{},
		nil,
		lumber.NewConsoleLogger(lumber.WARN),
		nil,
		nil,
		noOpFrontmatterIndexQueryer{},
		fileStorer,
		fileUploadsEnabled,
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return server
}

var _ = Describe("FileStorageService", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("UploadFile", func() {
		var (
			server     *v1.Server
			mockStorer *mockFileStorer
			req        *apiv1.UploadFileRequest
			resp       *apiv1.UploadFileResponse
			err        error
		)

		BeforeEach(func() {
			mockStorer = &mockFileStorer{
				StoreFunc: func(content io.Reader, filename string) (filestore.FileInfo, error) {
					return filestore.FileInfo{Hash: "sha256-TESTHASH", SizeBytes: 11}, nil
				},
			}
			req = &apiv1.UploadFileRequest{
				Content:  []byte("hello world"),
				Filename: "test.txt",
			}
		})

		JustBeforeEach(func() {
			resp, err = server.UploadFile(ctx, req)
		})

		When("file uploads are disabled", func() {
			BeforeEach(func() {
				server = mustNewServerWithFileStorer(mockStorer, false)
			})

			It("should return FailedPrecondition error", func() {
				Expect(err).To(HaveGrpcStatus(codes.FailedPrecondition, "file uploads are disabled on this server"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("file uploads are enabled", func() {
			BeforeEach(func() {
				server = mustNewServerWithFileStorer(mockStorer, true)
			})

			It("should return the hash and URL", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Hash).To(Equal("sha256-TESTHASH"))
				Expect(resp.UploadUrl).To(ContainSubstring("sha256-TESTHASH"))
				Expect(resp.UploadUrl).To(ContainSubstring("test.txt"))
			})

			When("content is empty", func() {
				BeforeEach(func() {
					req.Content = []byte{}
				})

				It("should return InvalidArgument error", func() {
					Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "content is required"))
				})
			})

			When("filename is empty", func() {
				BeforeEach(func() {
					req.Filename = ""
				})

				It("should return InvalidArgument error", func() {
					Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "filename is required"))
				})
			})
		})
	})

	Describe("GetFileInfo", func() {
		var (
			server     *v1.Server
			mockStorer *mockFileStorer
			req        *apiv1.GetFileInfoRequest
			resp       *apiv1.GetFileInfoResponse
			err        error
		)

		BeforeEach(func() {
			mockStorer = &mockFileStorer{
				GetInfoFunc: func(hash string) (filestore.FileInfo, error) {
					if hash == "sha256-EXISTS" {
						return filestore.FileInfo{Hash: hash, SizeBytes: 42}, nil
					}
					return filestore.FileInfo{}, os.ErrNotExist
				},
			}
			server = mustNewServerWithFileStorer(mockStorer, true)
			req = &apiv1.GetFileInfoRequest{Hash: "sha256-EXISTS"}
		})

		JustBeforeEach(func() {
			resp, err = server.GetFileInfo(ctx, req)
		})

		When("file exists", func() {
			It("should return file info", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Hash).To(Equal("sha256-EXISTS"))
				Expect(resp.SizeBytes).To(Equal(int64(42)))
			})
		})

		When("file does not exist", func() {
			BeforeEach(func() {
				req.Hash = "sha256-MISSING"
			})

			It("should return NotFound error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "file not found: sha256-MISSING"))
			})
		})

		When("hash is empty", func() {
			BeforeEach(func() {
				req.Hash = ""
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "hash is required"))
			})
		})
	})

	Describe("DeleteFile", func() {
		var (
			server     *v1.Server
			mockStorer *mockFileStorer
			req        *apiv1.DeleteFileRequest
			resp       *apiv1.DeleteFileResponse
			err        error
		)

		BeforeEach(func() {
			mockStorer = &mockFileStorer{
				DeleteFunc: func(hash string) error {
					if hash == "sha256-EXISTS" {
						return nil
					}
					return os.ErrNotExist
				},
			}
			server = mustNewServerWithFileStorer(mockStorer, true)
			req = &apiv1.DeleteFileRequest{Hash: "sha256-EXISTS"}
		})

		JustBeforeEach(func() {
			resp, err = server.DeleteFile(ctx, req)
		})

		When("file exists", func() {
			It("should return success", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Success).To(BeTrue())
			})
		})

		When("file does not exist", func() {
			BeforeEach(func() {
				req.Hash = "sha256-MISSING"
			})

			It("should return NotFound error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "file not found: sha256-MISSING"))
			})
		})

		When("hash is empty", func() {
			BeforeEach(func() {
				req.Hash = ""
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "hash is required"))
			})
		})
	})
})

//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/filestore"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
)

// mockFileStorer is a test double for filestore.FileStorer.
type mockFileStorer struct {
	StoreFunc   func(content io.Reader) (filestore.FileInfo, error)
	GetInfoFunc func(hash string) (filestore.FileInfo, error)
	DeleteFunc  func(hash string) error
}

func (m *mockFileStorer) Store(content io.Reader) (filestore.FileInfo, error) {
	return m.StoreFunc(content)
}

func (m *mockFileStorer) GetInfo(hash string) (filestore.FileInfo, error) {
	return m.GetInfoFunc(hash)
}

func (m *mockFileStorer) Delete(hash string) error {
	return m.DeleteFunc(hash)
}

func mustNewServerWithFileStorer(
	fileStorer filestore.FileStorer,
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
		noOpChatBufferManager{}, // chatBufferManager
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
				StoreFunc: func(_ io.Reader) (filestore.FileInfo, error) {
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
				server = mustNewServerWithFileStorer(nil)
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
				server = mustNewServerWithFileStorer(mockStorer)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the hash", func() {
				Expect(resp.Hash).To(Equal("sha256-TESTHASH"))
			})

			It("should return a URL containing the hash", func() {
				Expect(resp.UploadUrl).To(ContainSubstring("sha256-TESTHASH"))
			})

			It("should return a URL containing the filename", func() {
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

			When("the storer returns an error", func() {
				BeforeEach(func() {
					mockStorer.StoreFunc = func(_ io.Reader) (filestore.FileInfo, error) {
						return filestore.FileInfo{}, errors.New("disk full")
					}
				})

				It("should return an Internal error", func() {
					Expect(err).To(HaveGrpcStatus(codes.Internal, "failed to store file: disk full"))
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
			req = &apiv1.GetFileInfoRequest{Hash: "sha256-EXISTS"}
		})

		JustBeforeEach(func() {
			resp, err = server.GetFileInfo(ctx, req)
		})

		When("file uploads are disabled", func() {
			BeforeEach(func() {
				server = mustNewServerWithFileStorer(nil)
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
				server = mustNewServerWithFileStorer(mockStorer)
			})

			When("file exists", func() {
				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the correct hash", func() {
					Expect(resp.Hash).To(Equal("sha256-EXISTS"))
				})

				It("should return the correct size", func() {
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

			When("hash is invalid", func() {
				BeforeEach(func() {
					mockStorer.GetInfoFunc = func(_ string) (filestore.FileInfo, error) {
						return filestore.FileInfo{}, filestore.ErrInvalidHash
					}
					req.Hash = "../../../etc/passwd"
				})

				It("should return InvalidArgument error", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "invalid hash"))
				})
			})

			When("the storer returns an internal error", func() {
				BeforeEach(func() {
					mockStorer.GetInfoFunc = func(_ string) (filestore.FileInfo, error) {
						return filestore.FileInfo{}, errors.New("disk read error")
					}
				})

				It("should return an Internal error", func() {
					Expect(err).To(HaveGrpcStatus(codes.Internal, "failed to get file info: disk read error"))
				})
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
			req = &apiv1.DeleteFileRequest{Hash: "sha256-EXISTS"}
		})

		JustBeforeEach(func() {
			resp, err = server.DeleteFile(ctx, req)
		})

		When("file uploads are disabled", func() {
			BeforeEach(func() {
				server = mustNewServerWithFileStorer(nil)
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
				server = mustNewServerWithFileStorer(mockStorer)
			})

			When("file exists", func() {
				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return success", func() {
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

			When("hash is invalid", func() {
				BeforeEach(func() {
					mockStorer.DeleteFunc = func(_ string) error {
						return filestore.ErrInvalidHash
					}
					req.Hash = "../../../etc/passwd"
				})

				It("should return InvalidArgument error", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "invalid hash"))
				})
			})

			When("the storer returns an internal error", func() {
				BeforeEach(func() {
					mockStorer.DeleteFunc = func(_ string) error {
						return errors.New("disk write error")
					}
				})

				It("should return an Internal error", func() {
					Expect(err).To(HaveGrpcStatus(codes.Internal, "failed to delete file: disk write error"))
				})
			})
		})
	})
})

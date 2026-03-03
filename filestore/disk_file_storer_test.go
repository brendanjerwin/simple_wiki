package filestore_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/filestore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiskFileStorer", func() {
	var (
		tmpDir string
		storer *filestore.DiskFileStorer
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "filestore-test-*")
		Expect(err).NotTo(HaveOccurred())

		storer, err = filestore.NewDiskFileStorer(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("NewDiskFileStorer", func() {
		When("dataDir is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = filestore.NewDiskFileStorer("")
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("dataDir cannot be empty"))
			})
		})

		When("dataDir is valid", func() {
			var (
				s   *filestore.DiskFileStorer
				err error
			)

			BeforeEach(func() {
				s, err = filestore.NewDiskFileStorer(tmpDir)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a storer", func() {
				Expect(s).NotTo(BeNil())
			})
		})
	})

	Describe("Store", func() {
		When("content is provided", func() {
			var (
				info filestore.FileInfo
				err  error
			)

			BeforeEach(func() {
				info, err = storer.Store(strings.NewReader("hello world"))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a hash with sha256 prefix", func() {
				Expect(info.Hash).To(HavePrefix("sha256-"))
			})

			It("should return the correct size", func() {
				Expect(info.SizeBytes).To(Equal(int64(11)))
			})

			It("should create a file on disk", func() {
				_, statErr := os.Stat(filepath.Join(tmpDir, info.Hash+".upload"))
				Expect(statErr).NotTo(HaveOccurred())
			})
		})

		When("the same content is stored twice", func() {
			var (
				info1 filestore.FileInfo
				info2 filestore.FileInfo
			)

			BeforeEach(func() {
				var err error
				info1, err = storer.Store(strings.NewReader("same content"))
				Expect(err).NotTo(HaveOccurred())
				info2, err = storer.Store(strings.NewReader("same content"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce the same hash", func() {
				Expect(info1.Hash).To(Equal(info2.Hash))
			})
		})
	})

	Describe("GetInfo", func() {
		When("file exists", func() {
			var (
				stored filestore.FileInfo
				info   filestore.FileInfo
				err    error
			)

			BeforeEach(func() {
				var storeErr error
				stored, storeErr = storer.Store(strings.NewReader("hello world"))
				Expect(storeErr).NotTo(HaveOccurred())

				info, err = storer.GetInfo(stored.Hash)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct hash", func() {
				Expect(info.Hash).To(Equal(stored.Hash))
			})

			It("should return the correct size", func() {
				Expect(info.SizeBytes).To(Equal(stored.SizeBytes))
			})
		})

		When("file does not exist", func() {
			var err error

			BeforeEach(func() {
				_, err = storer.GetInfo("sha256-NONEXISTENT")
			})

			It("should return os.ErrNotExist", func() {
				Expect(err).To(MatchError(os.ErrNotExist))
			})
		})

		When("hash contains path traversal", func() {
			var err error

			BeforeEach(func() {
				_, err = storer.GetInfo("../../../etc/passwd")
			})

			It("should return ErrInvalidHash", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid hash")))
			})
		})

		When("hash contains null bytes", func() {
			var err error

			BeforeEach(func() {
				_, err = storer.GetInfo("sha256-evil\x00file")
			})

			It("should return ErrInvalidHash", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid hash")))
			})
		})
	})

	Describe("Delete", func() {
		When("file exists", func() {
			var (
				stored  filestore.FileInfo
				deleteErr error
			)

			BeforeEach(func() {
				var storeErr error
				stored, storeErr = storer.Store(strings.NewReader("to be deleted"))
				Expect(storeErr).NotTo(HaveOccurred())

				deleteErr = storer.Delete(stored.Hash)
			})

			It("should not error", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})

			It("should remove the file from disk", func() {
				_, statErr := os.Stat(filepath.Join(tmpDir, stored.Hash+".upload"))
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("file does not exist", func() {
			var err error

			BeforeEach(func() {
				err = storer.Delete("sha256-NONEXISTENT")
			})

			It("should return os.ErrNotExist", func() {
				Expect(err).To(MatchError(os.ErrNotExist))
			})
		})

		When("hash contains path traversal", func() {
			var err error

			BeforeEach(func() {
				err = storer.Delete("../../../etc/passwd")
			})

			It("should return ErrInvalidHash", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid hash")))
			})
		})

		When("hash contains null bytes", func() {
			var err error

			BeforeEach(func() {
				err = storer.Delete("sha256-evil\x00file")
			})

			It("should return ErrInvalidHash", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid hash")))
			})
		})
	})
})

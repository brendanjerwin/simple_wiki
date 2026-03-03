package filestore_test

import (
	"os"
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
			It("should return an error", func() {
				_, err := filestore.NewDiskFileStorer("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dataDir cannot be empty"))
			})
		})

		When("dataDir is valid", func() {
			It("should return a storer", func() {
				s, err := filestore.NewDiskFileStorer(tmpDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(s).NotTo(BeNil())
			})
		})
	})

	Describe("Store", func() {
		When("content is provided", func() {
			It("should store the file and return FileInfo with hash and size", func() {
				content := strings.NewReader("hello world")
				info, err := storer.Store(content, "hello.txt")
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Hash).To(HavePrefix("sha256-"))
				Expect(info.SizeBytes).To(Equal(int64(11)))
			})

			It("should create a file on disk", func() {
				content := strings.NewReader("test content")
				info, err := storer.Store(content, "test.txt")
				Expect(err).NotTo(HaveOccurred())

				_, statErr := os.Stat(tmpDir + "/" + info.Hash + ".upload")
				Expect(statErr).NotTo(HaveOccurred())
			})

			It("should produce the same hash for the same content", func() {
				info1, err := storer.Store(strings.NewReader("same content"), "file1.txt")
				Expect(err).NotTo(HaveOccurred())

				info2, err := storer.Store(strings.NewReader("same content"), "file2.txt")
				Expect(err).NotTo(HaveOccurred())

				Expect(info1.Hash).To(Equal(info2.Hash))
			})
		})
	})

	Describe("GetInfo", func() {
		When("file exists", func() {
			It("should return FileInfo with correct hash and size", func() {
				content := strings.NewReader("hello world")
				stored, err := storer.Store(content, "hello.txt")
				Expect(err).NotTo(HaveOccurred())

				info, err := storer.GetInfo(stored.Hash)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Hash).To(Equal(stored.Hash))
				Expect(info.SizeBytes).To(Equal(stored.SizeBytes))
			})
		})

		When("file does not exist", func() {
			It("should return os.ErrNotExist", func() {
				_, err := storer.GetInfo("sha256-NONEXISTENT")
				Expect(err).To(MatchError(os.ErrNotExist))
			})
		})

		When("hash contains path traversal", func() {
			It("should return an error", func() {
				_, err := storer.GetInfo("../../../etc/passwd")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Delete", func() {
		When("file exists", func() {
			It("should delete the file", func() {
				content := strings.NewReader("to be deleted")
				stored, err := storer.Store(content, "delete-me.txt")
				Expect(err).NotTo(HaveOccurred())

				err = storer.Delete(stored.Hash)
				Expect(err).NotTo(HaveOccurred())

				_, statErr := os.Stat(tmpDir + "/" + stored.Hash + ".upload")
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("file does not exist", func() {
			It("should return os.ErrNotExist", func() {
				err := storer.Delete("sha256-NONEXISTENT")
				Expect(err).To(MatchError(os.ErrNotExist))
			})
		})

		When("hash contains path traversal", func() {
			It("should return an error", func() {
				err := storer.Delete("../../../etc/passwd")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

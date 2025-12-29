//revive:disable:dot-imports
package eager

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileSystemDataDirScanner", func() {
	var (
		scanner     *FileSystemDataDirScanner
		testDataDir string
	)

	BeforeEach(func() {
		var err error
		testDataDir, err = os.MkdirTemp("", "data-dir-scanner-test")
		Expect(err).NotTo(HaveOccurred())
		scanner = NewFileSystemDataDirScanner(testDataDir)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("DataDirExists", func() {
		When("directory exists", func() {
			var exists bool

			BeforeEach(func() {
				exists = scanner.DataDirExists()
			})

			It("should return true", func() {
				Expect(exists).To(BeTrue())
			})
		})

		When("directory does not exist", func() {
			var exists bool

			BeforeEach(func() {
				scanner = NewFileSystemDataDirScanner("/nonexistent/path")
				exists = scanner.DataDirExists()
			})

			It("should return false", func() {
				Expect(exists).To(BeFalse())
			})
		})
	})

	Describe("ListMDFiles", func() {
		When("directory contains MD files", func() {
			var (
				files []string
				err   error
			)

			BeforeEach(func() {
				// Create some test files
				CreateTestFile(testDataDir, "test1.md", "content1")
				CreateTestFile(testDataDir, "test2.md", "content2")
				CreateTestFile(testDataDir, "other.txt", "not md")

				files, err = scanner.ListMDFiles()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return only MD files", func() {
				Expect(files).To(HaveLen(2))
				Expect(files).To(ContainElements("test1.md", "test2.md"))
			})

			It("should not include non-MD files", func() {
				Expect(files).NotTo(ContainElement("other.txt"))
			})
		})

		When("directory is empty", func() {
			var (
				files []string
				err   error
			)

			BeforeEach(func() {
				files, err = scanner.ListMDFiles()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty list", func() {
				Expect(files).To(BeEmpty())
			})
		})

		When("directory contains only non-MD files", func() {
			var (
				files []string
				err   error
			)

			BeforeEach(func() {
				CreateTestFile(testDataDir, "file.txt", "content")
				CreateTestFile(testDataDir, "data.json", "{}")

				files, err = scanner.ListMDFiles()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty list", func() {
				Expect(files).To(BeEmpty())
			})
		})

		When("directory does not exist", func() {
			var err error

			BeforeEach(func() {
				scanner = NewFileSystemDataDirScanner("/nonexistent/path")
				_, err = scanner.ListMDFiles()
			})

			It("should indicate failed to read directory", func() {
				Expect(err.Error()).To(ContainSubstring("failed to read data directory"))
			})
		})
	})

	Describe("ReadMDFile", func() {
		When("file exists", func() {
			var (
				content []byte
				err     error
			)

			BeforeEach(func() {
				CreateTestFile(testDataDir, "test.md", "# Test Content")
				content, err = scanner.ReadMDFile("test.md")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the file content", func() {
				Expect(string(content)).To(Equal("# Test Content"))
			})
		})

		When("file does not exist", func() {
			var err error

			BeforeEach(func() {
				_, err = scanner.ReadMDFile("nonexistent.md")
			})

			It("should return file not found error", func() {
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
	})

	Describe("MDFileExistsByBase32Name", func() {
		When("file exists", func() {
			var exists bool

			BeforeEach(func() {
				CreateTestFile(testDataDir, "testname.md", "content")
				exists = scanner.MDFileExistsByBase32Name("testname")
			})

			It("should return true", func() {
				Expect(exists).To(BeTrue())
			})
		})

		When("file does not exist", func() {
			var exists bool

			BeforeEach(func() {
				exists = scanner.MDFileExistsByBase32Name("nonexistent")
			})

			It("should return false", func() {
				Expect(exists).To(BeFalse())
			})
		})
	})

	Describe("DataDir", func() {
		When("getting the data directory path", func() {
			var path string

			BeforeEach(func() {
				path = scanner.DataDir()
			})

			It("should return the data directory path", func() {
				Expect(path).To(Equal(testDataDir))
			})
		})
	})

	Describe("ListMDFiles with subdirectories", func() {
		When("directory contains subdirectories", func() {
			var (
				files []string
				err   error
			)

			BeforeEach(func() {
				// Create a subdirectory (should be ignored)
				subdir := filepath.Join(testDataDir, "subdir.md")
				errMkdir := os.Mkdir(subdir, 0755)
				Expect(errMkdir).NotTo(HaveOccurred())

				// Create an actual MD file
				CreateTestFile(testDataDir, "real.md", "content")

				files, err = scanner.ListMDFiles()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not include directories even if named .md", func() {
				Expect(files).To(HaveLen(1))
				Expect(files).To(ContainElement("real.md"))
			})
		})
	})
})

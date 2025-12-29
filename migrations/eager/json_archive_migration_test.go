package eager

import (
	"os"
	"path/filepath"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)


var _ = Describe("JSONArchiveMigrationScanJob", func() {
	var (
		tempDir     string
		coordinator *jobs.JobQueueCoordinator
		scanJob     *JSONArchiveMigrationScanJob
		err         error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "json-archive-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Use a silent logger for testing
		coordinator = jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
		scanJob = NewJSONArchiveMigrationScanJob(tempDir, coordinator)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("Execute", func() {
		When("the data directory does not exist", func() {
			BeforeEach(func() {
				_ = os.RemoveAll(tempDir)
				err = scanJob.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate data directory does not exist", func() {
				Expect(err.Error()).To(ContainSubstring("data directory does not exist"))
			})
		})

		When("there are no JSON files", func() {
			BeforeEach(func() {
				err = scanJob.Execute()
			})

			It("should complete without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				// Since we can't check job count directly, we verify no queue stats
				queues := coordinator.GetActiveQueues()
				Expect(queues).To(BeEmpty())
			})
		})

		When("there are JSON files to archive", func() {
			var jsonFiles []string
			var queueStats []*jobs.QueueStats
			var executeErr error

			BeforeEach(func() {
				// Create some test JSON files
				jsonFiles = []string{
					base32tools.EncodeToBase32("page1") + ".json",
					base32tools.EncodeToBase32("page2") + ".json",
					base32tools.EncodeToBase32("page3") + ".json",
				}

				for _, filename := range jsonFiles {
					jsonPath := filepath.Join(tempDir, filename)
					err := os.WriteFile(jsonPath, []byte(`{"test": "data"}`), 0644)
					Expect(err).NotTo(HaveOccurred())
				}

				// Create a non-JSON file that should be ignored
				mdPath := filepath.Join(tempDir, base32tools.EncodeToBase32("page1")+".md")
				err := os.WriteFile(mdPath, []byte("# Test"), 0644)
				Expect(err).NotTo(HaveOccurred())

				executeErr = scanJob.Execute()
				queueStats = coordinator.GetActiveQueues()
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should create an active queue", func() {
				Expect(queueStats).NotTo(BeEmpty())
			})
		})

		When("there are JSON files in both root and __deleted__ directories", func() {
			var queueStats []*jobs.QueueStats
			var executeErr error

			BeforeEach(func() {
				// Create __deleted__ directory with a JSON file
				deletedDir := filepath.Join(tempDir, "__deleted__", "123456789")
				err := os.MkdirAll(deletedDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				jsonPath := filepath.Join(deletedDir, base32tools.EncodeToBase32("deleted")+".json")
				err = os.WriteFile(jsonPath, []byte(`{"test": "data"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create a regular JSON file
				regularPath := filepath.Join(tempDir, base32tools.EncodeToBase32("regular")+".json")
				err = os.WriteFile(regularPath, []byte(`{"test": "data"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				executeErr = scanJob.Execute()
				queueStats = coordinator.GetActiveQueues()
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should create an active queue for all JSON files", func() {
				Expect(queueStats).NotTo(BeEmpty())
			})
		})
	})

	Describe("GetName", func() {
		var name string

		BeforeEach(func() {
			name = scanJob.GetName()
		})

		It("should return the correct name", func() {
			Expect(name).To(Equal("JSON Archive Migration Scan"))
		})
	})
})

var _ = Describe("JSONArchiveMigrationJob", func() {
	var (
		tempDir string
		job     *JSONArchiveMigrationJob
		err     error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "json-archive-job-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("Execute", func() {
		When("archiving a JSON file", func() {
			var (
				jsonFilename   string
				jsonPath       string
				originalTime   time.Time
				originalExists bool
				deletedDirPath string
				deletedEntries []os.DirEntry
				archivedPath   string
				archivedContent []byte
				executeErr     error
			)

			BeforeEach(func() {
				jsonFilename = base32tools.EncodeToBase32("testpage") + ".json"
				jsonPath = filepath.Join(tempDir, jsonFilename)
				
				// Create a JSON file
				err := os.WriteFile(jsonPath, []byte(`{"identifier": "testpage", "text": {"current": "content"}}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Get the current time for checking the archive timestamp
				originalTime = time.Now()

				job = NewJSONArchiveMigrationJob(tempDir, jsonFilename)
				executeErr = job.Execute()

				// Check original file existence
				_, statErr := os.Stat(jsonPath)
				originalExists = !os.IsNotExist(statErr)

				// Check __deleted__ directory
				deletedDirPath = filepath.Join(tempDir, "__deleted__")
				deletedEntries, _ = os.ReadDir(deletedDirPath)

				if len(deletedEntries) > 0 {
					archivedPath = filepath.Join(deletedDirPath, deletedEntries[0].Name(), jsonFilename)
					archivedContent, _ = os.ReadFile(archivedPath)
				}
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should remove the original file", func() {
				Expect(originalExists).To(BeFalse())
			})

			It("should create __deleted__ directory", func() {
				_, err := os.Stat(deletedDirPath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create one timestamp directory", func() {
				Expect(deletedEntries).To(HaveLen(1))
			})

			It("should use timestamp format for directory name", func() {
				timestampDir := deletedEntries[0].Name()
				// Parse timestamp without timezone info
				parsedTime, err := time.ParseInLocation("20060102_150405", timestampDir, time.Local)
				Expect(err).NotTo(HaveOccurred())
				// Compare times allowing for small differences
				Expect(parsedTime).To(BeTemporally("~", originalTime, 5*time.Second))
			})

			It("should preserve file content in archive", func() {
				Expect(string(archivedContent)).To(ContainSubstring("testpage"))
			})
		})

		When("the __deleted__ directory doesn't exist initially", func() {
			var deletedDirExists bool
			var executeErr error

			BeforeEach(func() {
				jsonFilename := base32tools.EncodeToBase32("testpage") + ".json"
				jsonPath := filepath.Join(tempDir, jsonFilename)
				
				// Create a JSON file
				err := os.WriteFile(jsonPath, []byte(`{"test": "data"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Ensure __deleted__ doesn't exist
				deletedDir := filepath.Join(tempDir, "__deleted__")
				_ = os.RemoveAll(deletedDir)

				job = NewJSONArchiveMigrationJob(tempDir, jsonFilename)
				executeErr = job.Execute()

				// Check if __deleted__ was created
				_, statErr := os.Stat(deletedDir)
				deletedDirExists = statErr == nil
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should create the __deleted__ directory", func() {
				Expect(deletedDirExists).To(BeTrue())
			})
		})

		When("the JSON file doesn't exist", func() {
			BeforeEach(func() {
				job = NewJSONArchiveMigrationJob(tempDir, "nonexistent.json")
				err = job.Execute()
			})

			It("should complete without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the destination file already exists", func() {
			var (
				originalJSONPath string
				executeErr       error
				archivedFiles    []string
			)

			BeforeEach(func() {
				jsonFilename := base32tools.EncodeToBase32("duplicate") + ".json"
				originalJSONPath = filepath.Join(tempDir, jsonFilename)
				
				// Create original JSON file
				err := os.WriteFile(originalJSONPath, []byte(`{"test": "original"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create a pre-existing deleted directory with the same filename
				timestamp := time.Now().Format("20060102_150405")
				deletedDir := filepath.Join(tempDir, "__deleted__", timestamp)
				err = os.MkdirAll(deletedDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				existingPath := filepath.Join(deletedDir, jsonFilename)
				err = os.WriteFile(existingPath, []byte(`{"test": "existing"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Execute the migration
				job = NewJSONArchiveMigrationJob(tempDir, jsonFilename)
				executeErr = job.Execute()

				// Check what files exist in the deleted directory
				entries, _ := os.ReadDir(deletedDir)
				archivedFiles = make([]string, 0, len(entries))
				for _, entry := range entries {
					if !entry.IsDir() {
						archivedFiles = append(archivedFiles, entry.Name())
					}
				}
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should create a new file with incremented name", func() {
				// Should have both the original and the new file with incremented name
				Expect(archivedFiles).To(HaveLen(2))
				
				baseFilename := base32tools.EncodeToBase32("duplicate") + ".json"
				incrementedFilename := base32tools.EncodeToBase32("duplicate") + "_1.json"
				
				Expect(archivedFiles).To(ContainElement(baseFilename))
				Expect(archivedFiles).To(ContainElement(incrementedFilename))
			})

			It("should remove the original file", func() {
				_, err := os.Stat(originalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("multiple incremented files already exist", func() {
			var (
				originalJSONPath string
				executeErr       error
				archivedFiles    []string
			)

			BeforeEach(func() {
				jsonFilename := base32tools.EncodeToBase32("multidup") + ".json"
				originalJSONPath = filepath.Join(tempDir, jsonFilename)
				
				// Create original JSON file
				err := os.WriteFile(originalJSONPath, []byte(`{"test": "original"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create a pre-existing deleted directory with multiple incremented files
				timestamp := time.Now().Format("20060102_150405")
				deletedDir := filepath.Join(tempDir, "__deleted__", timestamp)
				err = os.MkdirAll(deletedDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				baseName := base32tools.EncodeToBase32("multidup")
				
				// Create base file
				existingPath := filepath.Join(deletedDir, baseName+".json")
				err = os.WriteFile(existingPath, []byte(`{"test": "existing"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create _1 file
				existing1Path := filepath.Join(deletedDir, baseName+"_1.json")
				err = os.WriteFile(existing1Path, []byte(`{"test": "existing1"}`), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Execute the migration
				job = NewJSONArchiveMigrationJob(tempDir, jsonFilename)
				executeErr = job.Execute()

				// Check what files exist in the deleted directory
				entries, _ := os.ReadDir(deletedDir)
				archivedFiles = make([]string, 0, len(entries))
				for _, entry := range entries {
					if !entry.IsDir() {
						archivedFiles = append(archivedFiles, entry.Name())
					}
				}
			})

			It("should complete without error", func() {
				Expect(executeErr).NotTo(HaveOccurred())
			})

			It("should create file with next available incremented name", func() {
				// Should have base, _1, and _2 files
				Expect(archivedFiles).To(HaveLen(3))
				
				baseName := base32tools.EncodeToBase32("multidup")
				baseFilename := baseName + ".json"
				incremented1Filename := baseName + "_1.json"
				incremented2Filename := baseName + "_2.json"
				
				Expect(archivedFiles).To(ContainElement(baseFilename))
				Expect(archivedFiles).To(ContainElement(incremented1Filename))
				Expect(archivedFiles).To(ContainElement(incremented2Filename))
			})

			It("should remove the original file", func() {
				_, err := os.Stat(originalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
	})

	Describe("GetName", func() {
		var name string

		BeforeEach(func() {
			job = NewJSONArchiveMigrationJob(tempDir, "test.json")
			name = job.GetName()
		})

		It("should return the correct name with filename", func() {
			Expect(name).To(Equal("JSON Archive Migration: test.json"))
		})
	})
})
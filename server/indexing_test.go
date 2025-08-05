package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InitializeIndexing Concurrently", func() {
	var (
		s       *Site
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "indexing-test")
		Expect(err).NotTo(HaveOccurred())

		s = &Site{
			Logger:     lumber.NewConsoleLogger(lumber.INFO),
			PathToData: tempDir,
			MigrationApplicator: &rollingmigrations.DefaultApplicator{},
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("when multiple files exist", func() {
		const numFiles = 5
		var err error

		BeforeEach(func() {
			for i := 0; i < numFiles; i++ {
				pageName := fmt.Sprintf("test-page-%d", i)
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower(pageName))
				jsonPagePath := filepath.Join(s.PathToData, encodedFilename+".json")
				mdPagePath := filepath.Join(s.PathToData, encodedFilename+".md")
				testPageContent := fmt.Sprintf(`{"identifier":"%s","text":{"current":"# %s","history":[]}}`, pageName, pageName)
				fileErr := os.WriteFile(jsonPagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())
				
				// Create .md file with frontmatter containing a title
				mdContent := fmt.Sprintf(`+++
title = "%s"
+++
# %s`, pageName, pageName)
				fileErr = os.WriteFile(mdPagePath, []byte(mdContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())
			}

			// Act
			err = s.InitializeIndexing()
			
			// Give a brief moment for indexing to start but don't wait for completion
			time.Sleep(50 * time.Millisecond)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should initialize FrontmatterIndexQueryer", func() {
			Expect(s.FrontmatterIndexQueryer).NotTo(BeNil())
		})

		It("should initialize BleveIndexQueryer", func() {
			Expect(s.BleveIndexQueryer).NotTo(BeNil())
		})

		It("should initialize IndexingService", func() {
			Expect(s.IndexingService).NotTo(BeNil())
		})
	})
})

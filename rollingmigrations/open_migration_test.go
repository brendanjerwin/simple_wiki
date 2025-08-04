package rollingmigrations_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

var _ = Describe("Rolling Migrations during Open()", func() {
	var (
		testDataDir string
		site        *server.Site
	)

	BeforeEach(func() {
		var err error
		testDataDir, err = os.MkdirTemp("", "wiki_open_migration_test_*")
		Expect(err).NotTo(HaveOccurred())

		// Initialize a minimal site with rolling migrations
		migrationApplicator := rollingmigrations.NewApplicator()
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		site = &server.Site{
			PathToData:          testDataDir,
			MigrationApplicator: migrationApplicator,
			Logger:              logger,
		}
	})

	AfterEach(func() {
		if testDataDir != "" {
			os.RemoveAll(testDataDir)
		}
	})

	Context("when opening a file with inventory.container frontmatter", func() {
		var (
			identifier    string
			fileContent   string
			filePath      string
			openedContent string
			err           error
		)

		BeforeEach(func() {
			identifier = "garage_unit_3_shelf_a"
			fileContent = `+++
identifier = "garage_unit_3_shelf_a"
title = "Garage Unit 3, Shelf A"
inventory.container = "GarageInventory"
+++
# {{or .Title .Identifier }}
### Goes in: {{LinkTo .Inventory.Container }}

## Contents
{{ ShowInventoryContentsOf .Identifier }}`

			// Create the file with the old format
			mungedIdentifier := wikiidentifiers.MungeIdentifier(identifier)
			encodedFilename := base32tools.EncodeToBase32(mungedIdentifier) + ".md"  
			filePath = filepath.Join(testDataDir, encodedFilename)
			

			err := os.WriteFile(filePath, []byte(fileContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify file was created
			_, err = os.Stat(filePath)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when calling Open()", func() {
			BeforeEach(func() {
				openedPage, openErr := site.Open(identifier)
				Expect(openErr).NotTo(HaveOccurred())
				openedContent = openedPage.Text.GetCurrent()
				if openedPage.WasLoadedFromDisk {
					err = nil
				} else {
					err = errors.New("page was not loaded from disk")
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the file content", func() {
				Expect(openedContent).NotTo(BeEmpty())
			})

			It("should have applied the inventory.container migration", func() {
				// The content should no longer have dotted notation
				Expect(openedContent).NotTo(ContainSubstring("inventory.container"), 
					"Content should not contain dotted notation after migration")

				// The content should have the [inventory] section
				Expect(openedContent).To(ContainSubstring("[inventory]"),
					"Content should contain [inventory] section after migration")
				
				// The container value should still be there
				Expect(openedContent).To(ContainSubstring(`container = "garage_inventory"`),
					"Content should still contain the container value")
			})

			Context("when checking if file on disk was updated", func() {
				var diskContent string

				BeforeEach(func() {
					// Read the file from disk to see if it was updated
					diskBytes, readErr := os.ReadFile(filePath)
					Expect(readErr).NotTo(HaveOccurred())
					diskContent = string(diskBytes)
				})

				It("should have updated the file on disk", func() {
					// The file on disk should also be migrated
					Expect(diskContent).NotTo(ContainSubstring("inventory.container"),
						"File on disk should not contain dotted notation after migration")
					Expect(diskContent).To(ContainSubstring("[inventory]"),
						"File on disk should contain [inventory] section after migration")
				})

				It("should have the same content as what Open() returned", func() {
					Expect(diskContent).To(Equal(openedContent),
						"Content on disk should match what Open() returned")
				})
			})
		})

		Context("when migration applicator is nil", func() {
			BeforeEach(func() {
				// Test with no migration applicator
				site.MigrationApplicator = nil
				_, openErr := site.Open(identifier)
				err = openErr // Capture the error for testing
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("migration applicator not configured"))
			})
		})
	})

	Context("when opening a file that doesn't need migration", func() {
		var (
			identifier    string
			fileContent   string
			openedContent string
			err           error
		)

		BeforeEach(func() {
			identifier = "already_migrated_page"
			fileContent = `+++
identifier = "already_migrated_page"
title = "Already Migrated Page"

[inventory]
container = "already_migrated"
+++
# Already migrated content`

			// Create the file
			mungedIdentifier := wikiidentifiers.MungeIdentifier(identifier)
			encodedFilename := base32tools.EncodeToBase32(mungedIdentifier) + ".md"  
			filePath := filepath.Join(testDataDir, encodedFilename)
			
			writeErr := os.WriteFile(filePath, []byte(fileContent), 0644)
			Expect(writeErr).NotTo(HaveOccurred())

			// Open it
			openedPage, openErr := site.Open(identifier)
			Expect(openErr).NotTo(HaveOccurred())
			openedContent = openedPage.Text.GetCurrent()
			if openedPage.WasLoadedFromDisk {
				err = nil
			} else {
				err = errors.New("page was not loaded from disk")
			}
		})

		It("should not modify already migrated content", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(openedContent).To(Equal(fileContent),
				"Already migrated content should remain unchanged")
		})
	})
})
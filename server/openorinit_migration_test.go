package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

var _ = Describe("OpenOrInit Migration Test", func() {
	var (
		testDataDir string
		site        *server.Site
	)

	BeforeEach(func() {
		var err error
		testDataDir, err = os.MkdirTemp("", "wiki_openorinit_migration_test_*")
		Expect(err).NotTo(HaveOccurred())

		// Initialize a site like the real application would
		migrationApplicator := lazy.NewApplicator()
		logger := lumber.NewConsoleLogger(lumber.INFO) // More verbose for debugging
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

	Context("when opening existing file with inventory.container via OpenOrInit", func() {
		var (
			identifier   string
			fileContent  string
			filePath     string
			openedPage   *server.Page
			err          error
			req          *http.Request
		)

		BeforeEach(func() {
			identifier = "garage_unit_3_shelf_a"
			fileContent = `+++
identifier = 'garage_unit_3_shelf_a'
title = 'Garage Unit 3, Shelf A'
inventory.container = 'GarageInventory'
+++
# {{or .Title .Identifier }}
### Goes in: {{LinkTo .Inventory.Container }}

## Contents
{{ ShowInventoryContentsOf .Identifier }}`

			// Create the file exactly as it would exist on disk
			mungedIdentifier := wikiidentifiers.MungeIdentifier(identifier)
			encodedFilename := base32tools.EncodeToBase32(mungedIdentifier) + ".md"  
			filePath = filepath.Join(testDataDir, encodedFilename)
			

			err := os.WriteFile(filePath, []byte(fileContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create a mock HTTP request like would come from /garage_unit_3_shelf_a/edit
			req = &http.Request{
				URL: &url.URL{
					Path:     "/garage_unit_3_shelf_a/edit",
					RawQuery: "",
				},
			}
		})

		Context("when calling OpenOrInit", func() {
			BeforeEach(func() {
				openedPage, err = site.OpenOrInit(identifier, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a page", func() {
				Expect(openedPage).NotTo(BeNil())
			})

			It("should have loaded the page from disk", func() {
				Expect(openedPage.WasLoadedFromDisk).To(BeTrue(), "Page should be loaded from disk")
				Expect(openedPage.IsNew()).To(BeFalse(), "Page should not be new")
			})

			It("should have applied the inventory.container migration", func() {
				content := openedPage.Text.GetCurrent()

				// Check for migration
				Expect(content).NotTo(ContainSubstring("inventory.container"), 
					"Content should not contain dotted notation after migration")
				Expect(content).To(ContainSubstring("[inventory]"),
					"Content should contain [inventory] section after migration")
				Expect(content).To(ContainSubstring(`container = 'garage_inventory'`),
					"Content should contain the munged container value")
			})

			Context("when checking file on disk", func() {
				var diskContent string

				BeforeEach(func() {
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
			})
		})
	})

	Context("when file doesn't exist (new page)", func() {
		var (
			identifier string
			openedPage *server.Page
			err        error
			req        *http.Request
		)

		BeforeEach(func() {
			identifier = "nonexistent_page"
			
			// Create a mock HTTP request
			req = &http.Request{
				URL: &url.URL{
					Path:     "/nonexistent_page/edit",
					RawQuery: "",
				},
			}

			openedPage, err = site.OpenOrInit(identifier, req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a new page", func() {
			Expect(openedPage).NotTo(BeNil())
			Expect(openedPage.IsNew()).To(BeTrue(), "Page should be new")
			Expect(openedPage.WasLoadedFromDisk).To(BeFalse(), "Page should not be loaded from disk")
		})
	})
})
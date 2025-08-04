package wikiidentifiers_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

var _ = Describe("File Lifecycle for lab_wallbins_L3", func() {
	var testDataDir string

	BeforeEach(func() {
		// Create a temporary directory for test files
		var err error
		testDataDir, err = os.MkdirTemp("", "wiki_test_data_*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Clean up test directory
		if testDataDir != "" {
			os.RemoveAll(testDataDir)
		}
	})

	Describe("PascalCase detection behavior", func() {
		Context("when identifier contains mixed case with numbers", func() {
			var (
				originalIdentifier string
				mungedIdentifier   string
				encodedFilename    string
				filePath           string
			)

			BeforeEach(func() {
				originalIdentifier = "lab_wallbins_L3"
				mungedIdentifier = wikiidentifiers.MungeIdentifier(originalIdentifier)
				encodedFilename = base32tools.EncodeToBase32(mungedIdentifier) + ".md"
				filePath = filepath.Join(testDataDir, encodedFilename)
			})

			It("should munge L3 to l3", func() {
				Expect(mungedIdentifier).To(Equal("lab_wallbins_l3"))
			})

			It("should encode to expected base32", func() {
				Expect(encodedFilename).To(Equal("NRQWEX3XMFWGYYTJNZZV63BT.md"))
			})

			Context("when file is created with munged identifier", func() {
				BeforeEach(func() {
					// Create test file
					testContent := fmt.Sprintf(`+++
identifier = "%s"
title = "Lab Wall Bins L3"

[inventory]
container = "LabInventory"
items = []
+++

# Lab Wall Bins L3
`, mungedIdentifier)
					
					err := os.WriteFile(filePath, []byte(testContent), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should exist on filesystem", func() {
					_, err := os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be findable by munged identifier", func() {
					expectedPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("lab_wallbins_l3")+".md")
					Expect(expectedPath).To(Equal(filePath))
					
					_, err := os.Stat(expectedPath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should NOT be findable by original mixed case identifier", func() {
					// This demonstrates the problem - original case doesn't match munged case
					wrongPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("lab_wallbins_L3")+".md")
					Expect(wrongPath).NotTo(Equal(filePath))
					
					_, err := os.Stat(wrongPath)
					Expect(os.IsNotExist(err)).To(BeTrue(), "File should not exist at original case path")
				})
			})
		})

		Context("comparing similar identifiers with different cases", func() {
			var testCases = []struct {
				identifier string
				expected   string
				description string
			}{
				{"lab_wallbins_L3", "lab_wallbins_l3", "uppercase L followed by number"},
				{"lab_wallbins_l3", "lab_wallbins_l3", "already lowercase"},
				{"lab_wallbins_L", "lab_wallbins_l", "uppercase L at end"},
				{"lab_WallBins_L3", "lab_wall_bins_l3", "mixed PascalCase with uppercase L and number"},
				{"labWallbinsL3", "lab_wallbins_l3", "camelCase with uppercase L and number"},
			}

			It("should consistently munge different case variations", func() {
				for _, tc := range testCases {
					munged := wikiidentifiers.MungeIdentifier(tc.identifier)
					Expect(munged).To(Equal(tc.expected), 
						fmt.Sprintf("Failed for %s (%s): got %s, expected %s", 
							tc.identifier, tc.description, munged, tc.expected))
				}
			})

			It("should show base32 encoding differences", func() {
				for _, tc := range testCases {
					munged := wikiidentifiers.MungeIdentifier(tc.identifier)
					encoded := base32tools.EncodeToBase32(munged)
					fmt.Printf("Identifier: %s -> Munged: %s -> Encoded: %s\n", 
						tc.identifier, munged, encoded)
				}
			})
		})
	})

	Describe("File creation and deletion patterns", func() {
		Context("when creating files with similar names", func() {
			var createdFiles []string

			BeforeEach(func() {
				createdFiles = []string{}
			})

			AfterEach(func() {
				// Track which files still exist
				fmt.Print("\nFiles remaining after test:\n")
				for _, file := range createdFiles {
					if _, err := os.Stat(file); err == nil {
						fmt.Printf("  EXISTS: %s\n", filepath.Base(file))
					} else if os.IsNotExist(err) {
						fmt.Printf("  DELETED: %s\n", filepath.Base(file))
					} else {
						fmt.Printf("  ERROR: %s - %v\n", filepath.Base(file), err)
					}
				}
			})

			It("should track creation of lab_wallbins_L3 variants", func() {
				variants := []string{
					"lab_wallbins_L3",
					"lab_wallbins_l3", 
					"lab_WallBins_L3",
					"labWallbinsL3",
				}

				for _, variant := range variants {
					munged := wikiidentifiers.MungeIdentifier(variant)
					encoded := base32tools.EncodeToBase32(munged)
					filePath := filepath.Join(testDataDir, encoded+".md")
					
					content := fmt.Sprintf(`+++
identifier = "%s"
title = "%s"
+++

# %s
`, munged, strings.ToTitle(strings.ReplaceAll(variant, "_", " ")), variant)

					err := os.WriteFile(filePath, []byte(content), 0644)
					Expect(err).NotTo(HaveOccurred())
					
					createdFiles = append(createdFiles, filePath)
					
					fmt.Printf("Created: %s -> %s -> %s\n", variant, munged, encoded+".md")
				}

				// All variants should create at least one file
				Expect(len(createdFiles)).To(BeNumerically(">", 0))
				
				// Check how many unique files were created
				uniqueFiles := make(map[string]bool)
				for _, file := range createdFiles {
					uniqueFiles[filepath.Base(file)] = true
				}
				
				// Note: Not all variants create the same filename - this demonstrates the issue
				// Some identifiers with different PascalCase patterns create different files
				fmt.Printf("Created %d unique files from %d variants\n", len(uniqueFiles), len(variants))
			})
		})
	})
})
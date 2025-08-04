package wikiidentifiers_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

var _ = Describe("File Shadowing Analysis", func() {
	var testDataDir string

	BeforeEach(func() {
		var err error
		testDataDir, err = os.MkdirTemp("", "wiki_shadowing_test_*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if testDataDir != "" {
			os.RemoveAll(testDataDir)
		}
	})

	Context("when different identifier variations create different files", func() {
		It("should show how PascalCase detection affects file naming", func() {
			variations := []struct {
				identifier       string
				description      string
				expectedMunged   string
				expectedEncoding string
			}{
				{
					"lab_wallbins_L3", 
					"underscore with uppercase L3",
					"lab_wallbins_l3",
					"NRQWEX3XMFWGYYTJNZZV63BT",
				},
				{
					"lab_WallBins_L3", 
					"PascalCase WallBins with L3",
					"lab_wall_bins_l3", // Extra underscore inserted!
					"NRQWEX3XMFWGYX3CNFXHGX3MGM======",
				},
				{
					"labWallbinsL3", 
					"camelCase version",
					"lab_wallbins_l3",
					"NRQWEX3XMFWGYYTJNZZV63BT",
				},
			}

			fmt.Print("\n=== Identifier Munging Analysis ===\n")
			for _, v := range variations {
				munged := wikiidentifiers.MungeIdentifier(v.identifier)
				encoded := base32tools.EncodeToBase32(munged)
				
				fmt.Printf("Input: %s (%s)\n", v.identifier, v.description)
				fmt.Printf("  Expected Munged: %s\n", v.expectedMunged)
				fmt.Printf("  Actual Munged:   %s\n", munged)
				fmt.Printf("  Expected Encoded: %s\n", v.expectedEncoding)
				fmt.Printf("  Actual Encoded:   %s\n", encoded)
				fmt.Printf("  Match: %t\n\n", munged == v.expectedMunged && encoded == v.expectedEncoding)
				
				Expect(munged).To(Equal(v.expectedMunged), fmt.Sprintf("Munging failed for %s", v.identifier))
				Expect(encoded).To(Equal(v.expectedEncoding), fmt.Sprintf("Encoding failed for %s", v.identifier))
			}
		})

		It("should demonstrate file shadowing behavior", func() {
			// Create a file with the first identifier
			identifier1 := "lab_wallbins_L3"
			munged1 := wikiidentifiers.MungeIdentifier(identifier1)
			encoded1 := base32tools.EncodeToBase32(munged1)
			filePath1 := filepath.Join(testDataDir, encoded1+".md")
			
			content1 := fmt.Sprintf(`+++
identifier = "%s"
title = "Original File"
+++

# Original content for %s
`, munged1, identifier1)

			err := os.WriteFile(filePath1, []byte(content1), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify first file exists
			_, err = os.Stat(filePath1)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("Created file 1: %s -> %s\n", identifier1, encoded1+".md")

			// Now create a file with a different identifier that might shadow it
			identifier2 := "lab_WallBins_L3" 
			munged2 := wikiidentifiers.MungeIdentifier(identifier2)
			encoded2 := base32tools.EncodeToBase32(munged2)
			filePath2 := filepath.Join(testDataDir, encoded2+".md")
			
			content2 := fmt.Sprintf(`+++
identifier = "%s"
title = "Different File"
+++

# Different content for %s
`, munged2, identifier2)

			err = os.WriteFile(filePath2, []byte(content2), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify second file exists
			_, err = os.Stat(filePath2)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("Created file 2: %s -> %s\n", identifier2, encoded2+".md")

			// The key insight: These should be DIFFERENT files
			Expect(filePath1).NotTo(Equal(filePath2), "Different identifiers should create different files")
			Expect(encoded1).NotTo(Equal(encoded2), "Different munging should create different encodings")

			// Both files should exist simultaneously
			_, err1 := os.Stat(filePath1)
			_, err2 := os.Stat(filePath2)
			Expect(err1).NotTo(HaveOccurred(), "First file should still exist")
			Expect(err2).NotTo(HaveOccurred(), "Second file should also exist")

			fmt.Print("\nResult: Two different files created:\n")
			fmt.Printf("  File 1: %s (from %s)\n", filepath.Base(filePath1), identifier1)
			fmt.Printf("  File 2: %s (from %s)\n", filepath.Base(filePath2), identifier2)
		})

		It("should show potential for file conflicts with similar identifiers", func() {
			// Test identifiers that could be confused
			conflictingIDs := []string{
				"lab_wallbins_L3",
				"labWallbinsL3", 
				"lab_wall_bins_L3", // This one is different!
				"Lab_Wallbins_L3",
			}

			fileMap := make(map[string][]string) // encoded filename -> list of identifiers that create it

			for _, id := range conflictingIDs {
				munged := wikiidentifiers.MungeIdentifier(id)
				encoded := base32tools.EncodeToBase32(munged)
				
				fileMap[encoded] = append(fileMap[encoded], id)
				fmt.Printf("%s -> %s -> %s\n", id, munged, encoded)
			}

			fmt.Print("\n=== File Collision Analysis ===\n")
			for encoded, identifiers := range fileMap {
				if len(identifiers) > 1 {
					fmt.Printf("COLLISION: File %s would be created by:\n", encoded)
					for _, id := range identifiers {
						fmt.Printf("  - %s\n", id)
					}
				} else {
					fmt.Printf("UNIQUE: File %s created by: %s\n", encoded, identifiers[0])
				}
			}

			// This demonstrates the potential for confusion:
			// Multiple identifiers might map to the same file, OR
			// Similar identifiers might create different files
		})
	})
})
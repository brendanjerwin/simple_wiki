//revive:disable:dot-imports
package wikiidentifiers

import (
	"strings"
	"testing"
	
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWikiIdentifiers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WikiIdentifiers Suite")
}

var _ = Describe("MungeIdentifier", func() {
	Describe("when identifier contains UUID", func() {
		It("should return lowercase identifier", func() {
			identifier := "Page-12345678-1234-1234-A123-123456789ABC"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("page-12345678-1234-1234-a123-123456789abc"))
		})
	})

	Describe("when identifier is regular text", func() {
		It("should convert to snake_case and lowercase", func() {
			identifier := "MyPage"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("my_page"))
		})

		It("should handle CamelCase", func() {
			identifier := "SomeCamelCaseIdentifier"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("some_camel_case_identifier"))
		})

		It("should handle mixed case and numbers", func() {
			identifier := "lab_wallbins_L3"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("lab_wallbins_l3"))
		})

		It("should handle already snake_case identifiers", func() {
			identifier := "already_snake_case"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("already_snake_case"))
		})

		It("should handle identifiers with numbers", func() {
			identifier := "TestPage123"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("test_page123"))
		})

		It("should handle identifiers starting with numbers", func() {
			identifier := "123TestPage"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("123test_page"))
		})
	})

	Describe("when identifier has special characters", func() {
		It("should handle underscores", func() {
			identifier := "test_identifier_with_underscores"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("test_identifier_with_underscores"))
		})

		It("should handle hyphens", func() {
			identifier := "test-identifier-with-hyphens"
			result := MungeIdentifier(identifier)
			Expect(result).To(Equal("test_identifier_with_hyphens"))
		})
	})

	Describe("Full pipeline tests", func() {
		It("should show the full munging and base32 encoding pipeline for lab_wallbins_L3", func() {
			original := "lab_wallbins_L3"
			munged := MungeIdentifier(original)
			encoded := base32tools.EncodeToBase32(munged)
			
			Expect(munged).To(Equal("lab_wallbins_l3"))
			Expect(encoded).NotTo(BeEmpty())
			
			// Log the full pipeline for debugging
			GinkgoWriter.Printf("Original: %s\n", original)
			GinkgoWriter.Printf("Munged: %s\n", munged)
			GinkgoWriter.Printf("Base32 encoded: %s\n", encoded)
		})

		It("should decode existing filenames to understand what they are", func() {
			// Some existing filenames from the data directory
			existingFiles := []string{
				"NRQWEX3DMFRGS3TFOQZA====",
				"NRQWEX3XMFWGYX3CNFXHG===",
				"NRQWEX3TNVQWY3C7OBQXE5DT",
			}
			
			for _, filename := range existingFiles {
				// Remove the .json/.md extension if present
				base32Part := strings.Split(filename, ".")[0]
				decoded, err := base32tools.DecodeFromBase32(base32Part)
				Expect(err).NotTo(HaveOccurred())
				
				GinkgoWriter.Printf("Base32: %s -> Decoded: '%s'\n", base32Part, decoded)
			}
		})
	})
})
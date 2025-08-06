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
		var (
			identifier string
			result     string
		)

		BeforeEach(func() {
			identifier = "Page-12345678-1234-1234-A123-123456789ABC"
			result = MungeIdentifier(identifier)
		})

		It("should return lowercase identifier", func() {
			Expect(result).To(Equal("page-12345678-1234-1234-a123-123456789abc"))
		})
	})

	Describe("when identifier is regular text", func() {
		Describe("when converting MyPage", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "MyPage"
				result = MungeIdentifier(identifier)
			})

			It("should convert to snake_case and lowercase", func() {
				Expect(result).To(Equal("my_page"))
			})
		})

		Describe("when handling CamelCase", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "SomeCamelCaseIdentifier"
				result = MungeIdentifier(identifier)
			})

			It("should handle CamelCase", func() {
				Expect(result).To(Equal("some_camel_case_identifier"))
			})
		})

		Describe("when handling mixed case and numbers", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "lab_wallbins_L3"
				result = MungeIdentifier(identifier)
			})

			It("should handle mixed case and numbers", func() {
				Expect(result).To(Equal("lab_wallbins_l3"))
			})
		})

		Describe("when handling already snake_case identifiers", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "already_snake_case"
				result = MungeIdentifier(identifier)
			})

			It("should handle already snake_case identifiers", func() {
				Expect(result).To(Equal("already_snake_case"))
			})
		})

		Describe("when handling identifiers with numbers", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "TestPage123"
				result = MungeIdentifier(identifier)
			})

			It("should handle identifiers with numbers", func() {
				Expect(result).To(Equal("test_page123"))
			})
		})

		Describe("when handling identifiers starting with numbers", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "123TestPage"
				result = MungeIdentifier(identifier)
			})

			It("should handle identifiers starting with numbers", func() {
				Expect(result).To(Equal("123test_page"))
			})
		})
	})

	Describe("when identifier has special characters", func() {
		Describe("when handling underscores", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "test_identifier_with_underscores"
				result = MungeIdentifier(identifier)
			})

			It("should handle underscores", func() {
				Expect(result).To(Equal("test_identifier_with_underscores"))
			})
		})

		Describe("when handling hyphens", func() {
			var (
				identifier string
				result     string
			)

			BeforeEach(func() {
				identifier = "test-identifier-with-hyphens"
				result = MungeIdentifier(identifier)
			})

			It("should handle hyphens", func() {
				Expect(result).To(Equal("test_identifier_with_hyphens"))
			})
		})
	})

	Describe("Full pipeline tests", func() {
		Describe("when showing the full munging and base32 encoding pipeline for lab_wallbins_L3", func() {
			var (
				original string
				munged   string
				encoded  string
			)

			BeforeEach(func() {
				original = "lab_wallbins_L3"
				munged = MungeIdentifier(original)
				encoded = base32tools.EncodeToBase32(munged)
				
				// Log the full pipeline for debugging
				GinkgoWriter.Printf("Original: %s\n", original)
				GinkgoWriter.Printf("Munged: %s\n", munged)
				GinkgoWriter.Printf("Base32 encoded: %s\n", encoded)
			})

			It("should munge to lowercase", func() {
				Expect(munged).To(Equal("lab_wallbins_l3"))
			})

			It("should encode to non-empty base32", func() {
				Expect(encoded).NotTo(BeEmpty())
			})
		})

		Describe("when decoding existing filenames", func() {
			var (
				existingFiles []string
				decodedFiles  map[string]string
			)

			BeforeEach(func() {
				// Some existing filenames from the data directory
				existingFiles = []string{
					"NRQWEX3DMFRGS3TFOQZA====",
					"NRQWEX3XMFWGYX3CNFXHG===",
					"NRQWEX3TNVQWY3C7OBQXE5DT",
				}
				
				decodedFiles = make(map[string]string)
				for _, filename := range existingFiles {
					// Remove the .json/.md extension if present
					base32Part := strings.Split(filename, ".")[0]
					decoded, err := base32tools.DecodeFromBase32(base32Part)
					Expect(err).NotTo(HaveOccurred())
					decodedFiles[base32Part] = decoded
					
					GinkgoWriter.Printf("Base32: %s -> Decoded: '%s'\n", base32Part, decoded)
				}
			})

			It("should decode all existing filenames without errors", func() {
				Expect(decodedFiles).To(HaveLen(len(existingFiles)))
				for filename := range decodedFiles {
					Expect(decodedFiles[filename]).NotTo(BeEmpty())
				}
			})
		})
	})
})
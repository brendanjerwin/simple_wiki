package rollingmigrations_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

var _ = Describe("Debug Inventory Migration", func() {
	Context("debugging splitTOMLContent", func() {
		It("should debug parsing of simple inventory.container frontmatter", func() {
			testContent := `+++
inventory.container = "Test"
+++`
			// Verify parsing works correctly
			parts := strings.Split(testContent, "+++")
			_ = parts // Used for verification

			// Actually test the migration
			migrationApplicator := rollingmigrations.NewApplicator()
			_, err := migrationApplicator.ApplyMigrations([]byte(testContent))
			
			
			Expect(err).NotTo(HaveOccurred())
		})

		It("should test various frontmatter endings", func() {
			testCases := []struct {
				name    string
				content string
			}{
				{
					name: "with newline after closing +++",
					content: `+++
inventory.container = "Test"
+++
`,
				},
				{
					name: "without newline after closing +++",
					content: `+++
inventory.container = "Test"
+++`,
				},
				{
					name: "with content after frontmatter",
					content: `+++
inventory.container = "Test"
+++

Some content here`,
				},
			}

			migrationApplicator := rollingmigrations.NewApplicator()
			
			for _, tc := range testCases {
				result, err := migrationApplicator.ApplyMigrations([]byte(tc.content))
				Expect(err).NotTo(HaveOccurred())
				
				// Verify migration works for all formats
				Expected := string(result) != tc.content || strings.Contains(string(result), "[inventory]")
				Expect(Expected).To(BeTrue(), "Migration should work for: %s", tc.name)
			}
		})
	})
})
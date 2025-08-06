//revive:disable:dot-imports
package server_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/server"
)

var _ = Describe("BuildFrontmatterFromURLParams", func() {
	var (
		identifier string
		params     url.Values
		result     map[string]any
		err        error
	)

	BeforeEach(func() {
		identifier = "test_page"
		params = url.Values{}
	})

	JustBeforeEach(func() {
		result, err = server.BuildFrontmatterFromURLParams(identifier, params)
	})

	It("should exist", func() {
		// Basic existence test
		Expect(err).To(BeNil())
	})

	Describe("when params are empty", func() {
		It("should return a frontmatter with only identifier", func() {
			Expect(result).To(HaveKeyWithValue("identifier", "test_page"))
			Expect(result).To(HaveLen(1))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when params contain simple key-value pairs", func() {
		BeforeEach(func() {
			params.Set("title", "My Test Page")
			params.Set("description", "A test page")
		})

		It("should include identifier", func() {
			Expect(result).To(HaveKeyWithValue("identifier", "test_page"))
		})

		It("should include title parameter", func() {
			Expect(result).To(HaveKeyWithValue("title", "My Test Page"))
		})

		It("should include description parameter", func() {
			Expect(result).To(HaveKeyWithValue("description", "A test page"))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when params contain dotted keys", func() {
		BeforeEach(func() {
			params.Set("inventory.container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1")
			params.Set("inventory.location", "Lab A")
		})

		It("should create nested inventory structure", func() {
			Expect(result).To(HaveKey("inventory"))
		})

		It("should create inventory as map type", func() {
			_, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue(), "inventory should be a map[string]any")
		})

		It("should include container in inventory", func() {
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1"))
		})

		It("should include location in inventory", func() {
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("location", "Lab A"))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when params contain deeply nested dotted keys", func() {
		BeforeEach(func() {
			params.Set("metadata.author.name", "John Doe")
			params.Set("metadata.author.email", "john@example.com")
			params.Set("metadata.version", "1.0.0")
		})

		It("should create metadata structure", func() {
			Expect(result).To(HaveKey("metadata"))
		})

		It("should create metadata as map type", func() {
			_, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue(), "metadata should be a map[string]any")
		})

		It("should create author structure in metadata", func() {
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(metadata).To(HaveKey("author"))
		})

		It("should create author as map type", func() {
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			_, ok = metadata["author"].(map[string]any)
			Expect(ok).To(BeTrue(), "author should be a map[string]any")
		})

		It("should include author name", func() {
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			author, ok := metadata["author"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(author).To(HaveKeyWithValue("name", "John Doe"))
		})

		It("should include author email", func() {
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			author, ok := metadata["author"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(author).To(HaveKeyWithValue("email", "john@example.com"))
		})

		It("should include version in metadata", func() {
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(metadata).To(HaveKeyWithValue("version", "1.0.0"))
		})
	})

	Describe("when params contain array values", func() {
		BeforeEach(func() {
			params["tags"] = []string{"one", "two", "three"}
		})

		It("should have tags key", func() {
			Expect(result).To(HaveKey("tags"))
		})

		It("should preserve array type", func() {
			_, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue(), "tags should be a []string")
		})

		It("should contain correct array values", func() {
			tags, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue())
			Expect(tags).To(Equal([]string{"one", "two", "three"}))
		})
	})

	Describe("when params contain special parameters to filter", func() {
		BeforeEach(func() {
			params.Set("tmpl", "inv_item")
			params.Set("title", "My Item")
			params.Set("_internal", "should_be_filtered")
		})

		It("should filter out tmpl parameter", func() {
			Expect(result).NotTo(HaveKey("tmpl"))
		})

		It("should filter out parameters starting with underscore", func() {
			Expect(result).NotTo(HaveKey("_internal"))
		})

		It("should include non-special parameters", func() {
			Expect(result).To(HaveKeyWithValue("title", "My Item"))
		})
	})

	Describe("when params contain mixed dotted and simple keys", func() {
		BeforeEach(func() {
			params.Set("title", "Test Page")
			params.Set("inventory.container", "Container1")
			params.Set("tags", "tag1")
			params.Add("tags", "tag2")
		})

		It("should include title parameter", func() {
			Expect(result).To(HaveKeyWithValue("title", "Test Page"))
		})

		It("should include inventory structure", func() {
			Expect(result).To(HaveKey("inventory"))
		})

		It("should create inventory as map type", func() {
			_, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
		})

		It("should include container in inventory", func() {
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("container", "Container1"))
		})

		It("should include tags array", func() {
			Expect(result).To(HaveKey("tags"))
		})

		It("should create tags as array type", func() {
			_, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue())
		})

		It("should contain both tag values", func() {
			tags, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue())
			Expect(tags).To(ConsistOf("tag1", "tag2"))
		})
	})

	Describe("when params would create invalid TOML structure", func() {
		BeforeEach(func() {
			// This creates an invalid TOML structure - cannot have both a string value and a table with the same key
			params.Set("inventory", "simple_value")
			params.Set("inventory.container", "Container1")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return a descriptive error message", func() {
			Expect(err.Error()).To(ContainSubstring("inventory"))
			Expect(err.Error()).To(ContainSubstring("cannot be both"))
		})

		It("should return nil result", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when identifier parameter is passed in URL", func() {
		BeforeEach(func() {
			params.Set("identifier", "url_identifier")
		})

		It("should use the provided identifier parameter, not URL value", func() {
			// The function should prefer the identifier passed as argument
			Expect(result).To(HaveKeyWithValue("identifier", "test_page"))
		})
	})
})
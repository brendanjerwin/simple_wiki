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

		It("should include all simple parameters", func() {
			Expect(result).To(HaveKeyWithValue("identifier", "test_page"))
			Expect(result).To(HaveKeyWithValue("title", "My Test Page"))
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

		It("should create nested map structure", func() {
			Expect(result).To(HaveKey("inventory"))
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue(), "inventory should be a map[string]any")
			Expect(inventory).To(HaveKeyWithValue("container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1"))
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

		It("should create deeply nested map structure", func() {
			Expect(result).To(HaveKey("metadata"))
			metadata, ok := result["metadata"].(map[string]any)
			Expect(ok).To(BeTrue(), "metadata should be a map[string]any")
			
			Expect(metadata).To(HaveKey("author"))
			author, ok := metadata["author"].(map[string]any)
			Expect(ok).To(BeTrue(), "author should be a map[string]any")
			
			Expect(author).To(HaveKeyWithValue("name", "John Doe"))
			Expect(author).To(HaveKeyWithValue("email", "john@example.com"))
			Expect(metadata).To(HaveKeyWithValue("version", "1.0.0"))
		})
	})

	Describe("when params contain array values", func() {
		BeforeEach(func() {
			params["tags"] = []string{"one", "two", "three"}
		})

		It("should preserve array values", func() {
			Expect(result).To(HaveKey("tags"))
			tags, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue(), "tags should be a []string")
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

		It("should handle both correctly", func() {
			Expect(result).To(HaveKeyWithValue("title", "Test Page"))
			Expect(result).To(HaveKey("inventory"))
			
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("container", "Container1"))
			
			// Multiple values become an array
			Expect(result).To(HaveKey("tags"))
			tags, ok := result["tags"].([]string)
			Expect(ok).To(BeTrue())
			Expect(tags).To(ConsistOf("tag1", "tag2"))
		})
	})

	Describe("when params would override existing nested structure", func() {
		BeforeEach(func() {
			// This creates a potential conflict
			params.Set("inventory", "simple_value")
			params.Set("inventory.container", "Container1")
		})

		It("should prefer the nested structure", func() {
			Expect(result).To(HaveKey("inventory"))
			inventory, ok := result["inventory"].(map[string]any)
			Expect(ok).To(BeTrue(), "inventory should be a map, not a simple value")
			Expect(inventory).To(HaveKeyWithValue("container", "Container1"))
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
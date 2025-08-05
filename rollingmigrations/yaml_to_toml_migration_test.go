package rollingmigrations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("YAMLToTOMLMigration", func() {
	var migration *YAMLToTOMLMigration

	BeforeEach(func() {
		migration = NewYAMLToTOMLMigration()
	})

	Describe("SupportedTypes", func() {
		var supportedTypes []FrontmatterType

		BeforeEach(func() {
			supportedTypes = migration.SupportedTypes()
		})

		It("should return only YAML frontmatter type", func() {
			Expect(supportedTypes).To(Equal([]FrontmatterType{FrontmatterYAML}))
		})
	})

	Describe("AppliesTo", func() {
		Describe("when content has YAML frontmatter", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`---
title: Test Page
author: John Doe
---

# Test Content`)
				applies = migration.AppliesTo(content)
			})

			It("should return true", func() {
				Expect(applies).To(BeTrue())
			})
		})

		Describe("when content has TOML frontmatter", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
title = "Test Page"
author = "John Doe"
+++

# Test Content`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has JSON frontmatter", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`{
  "title": "Test Page",
  "author": "John Doe"
}

# Test Content`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has no frontmatter", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`# Test Content

Just markdown content without frontmatter.`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content is empty", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte("")
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has malformed YAML frontmatter", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`---
title: Test Page
invalid yaml: [unclosed
---

# Test Content`)
				applies = migration.AppliesTo(content)
			})

			It("should return true", func() {
				Expect(applies).To(BeTrue())
			})
		})
	})

	Describe("Apply", func() {
		Describe("when converting simple YAML frontmatter", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`---
title: Test Page
author: John Doe
published: true
---

# Test Content

This is the markdown body.`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert to TOML frontmatter", func() {
				expectedContent := `+++
author = 'John Doe'
published = true
title = 'Test Page'
+++

# Test Content

This is the markdown body.`
				Expect(string(convertedContent)).To(Equal(expectedContent))
			})

			It("should not apply to the converted content", func() {
				Expect(migration.AppliesTo(convertedContent)).To(BeFalse())
			})
		})

		Describe("when converting YAML with nested structures", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`---
title: Complex Page
metadata:
  author: John Doe
  date: 2025-01-01
tags:
  - go
  - testing
  - yaml
---

# Complex Content`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use TOML frontmatter delimiters", func() {
				Expect(string(convertedContent)).To(ContainSubstring("+++"))
			})

			It("should convert title to TOML format", func() {
				Expect(string(convertedContent)).To(ContainSubstring(`title = 'Complex Page'`))
			})

			It("should convert nested metadata to TOML table", func() {
				Expect(string(convertedContent)).To(ContainSubstring("[metadata]"))
				Expect(string(convertedContent)).To(ContainSubstring(`author = 'John Doe'`))
			})

			It("should convert arrays to TOML format", func() {
				Expect(string(convertedContent)).To(ContainSubstring(`tags = ['go', 'testing', 'yaml']`))
			})

			It("should preserve the markdown content", func() {
				Expect(string(convertedContent)).To(ContainSubstring("# Complex Content"))
			})
		})

		Describe("when converting YAML with arrays", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`---
title: Array Test
numbers:
  - 1
  - 2
  - 3
items:
  - name: first
    value: 10
  - name: second
    value: 20
---

# Array Content`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use TOML frontmatter delimiters", func() {
				Expect(string(convertedContent)).To(ContainSubstring("+++"))
			})

			It("should convert simple arrays to TOML format", func() {
				Expect(string(convertedContent)).To(ContainSubstring("numbers = [1, 2, 3]"))
			})

			It("should convert array of tables to TOML format", func() {
				Expect(string(convertedContent)).To(ContainSubstring("[[items]]"))
			})
		})

		Describe("when content has malformed YAML", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`---
title: Test Page
invalid yaml: [unclosed
---

# Test Content`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return original content", func() {
				Expect(convertedContent).To(Equal(originalContent))
			})
		})

		Describe("when content has no YAML frontmatter", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`# Just Markdown

No frontmatter here.`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return content unchanged", func() {
				Expect(convertedContent).To(Equal(originalContent))
			})
		})

		Describe("when content is empty", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte("")
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty content", func() {
				Expect(convertedContent).To(Equal(originalContent))
			})
		})

		Describe("when YAML frontmatter has no closing delimiter", func() {
			var originalContent []byte
			var convertedContent []byte
			var err error

			BeforeEach(func() {
				originalContent = []byte(`---
title: Test Page
author: John Doe

# Test Content`)
				convertedContent, err = migration.Apply(originalContent)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return original content", func() {
				Expect(convertedContent).To(Equal(originalContent))
			})
		})
	})
})
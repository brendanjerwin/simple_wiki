package rollingmigrations_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

var _ = Describe("TOMLTableSpacingMigration", func() {
	var migration *rollingmigrations.TOMLTableSpacingMigration

	BeforeEach(func() {
		migration = rollingmigrations.NewTOMLTableSpacingMigration()
	})

	It("should exist", func() {
		Expect(migration).NotTo(BeNil())
	})

	Describe("SupportedTypes", func() {
		var supportedTypes []rollingmigrations.FrontmatterType

		BeforeEach(func() {
			supportedTypes = migration.SupportedTypes()
		})

		It("should only support TOML frontmatter", func() {
			Expect(supportedTypes).To(HaveLen(1))
			Expect(supportedTypes[0]).To(Equal(rollingmigrations.FrontmatterTOML))
		})
	})

	Describe("AppliesTo", func() {
		Describe("when content has table header without blank line above", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
title = "test"
[inventory]
items = []
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return true", func() {
				Expect(applies).To(BeTrue())
			})
		})

		Describe("when content has table header with blank line above", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
title = "test"

[inventory]
items = []
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when table header is the first line", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
[inventory]
items = []
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has multiple table headers with mixed spacing", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
title = "test"

[inventory]
items = []
status = "active"
[game]
level = 1
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return true", func() {
				Expect(applies).To(BeTrue())
			})
		})

		Describe("when content has no table headers", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
title = "test"
status = "active"
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has no frontmatter", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`just content without frontmatter`)
				applies = migration.AppliesTo(content)
			})

			It("should return false", func() {
				Expect(applies).To(BeFalse())
			})
		})
	})

	Describe("Apply", func() {
		Describe("when content has single table header without blank line", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
title = "test"
[inventory]
items = []
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add blank line before table header", func() {
				expected := []byte(`+++
title = "test"

[inventory]
items = []
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content has multiple table headers without blank lines", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
title = "test"
[inventory]
items = []
status = "active"
[game]
level = 1
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add blank lines before all table headers", func() {
				expected := []byte(`+++
title = "test"

[inventory]
items = []
status = "active"

[game]
level = 1
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content has nested table headers", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
title = "test"
[game.inventory]
items = []
[game.player]
name = "hero"
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add blank lines before nested table headers", func() {
				expected := []byte(`+++
title = "test"

[game.inventory]
items = []

[game.player]
name = "hero"
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content already has proper spacing", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
title = "test"

[inventory]
items = []

[game]
level = 1
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not modify the content", func() {
				Expect(result).To(Equal(content))
			})
		})

		Describe("when table header is the first line", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
[inventory]
items = []
title = "test"
[game]
level = 1
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not add blank line before first table header but should add before second", func() {
				expected := []byte(`+++
[inventory]
items = []
title = "test"

[game]
level = 1
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})
	})
})
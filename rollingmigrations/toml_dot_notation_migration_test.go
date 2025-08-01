package rollingmigrations_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

var _ = Describe("TOMLDotNotationMigration", func() {
	var migration *rollingmigrations.TOMLDotNotationMigration

	BeforeEach(func() {
		migration = rollingmigrations.NewTOMLDotNotationMigration()
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
		Describe("when content has dot notation with conflicting table syntax", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
inventory.container = "fireplace_cabinet_right"
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

		Describe("when content has dot notation without conflicting table", func() {
			var applies bool

			BeforeEach(func() {
				content := []byte(`+++
inventory.container = "fireplace_cabinet_right"
title = "test"
+++
content here`)
				applies = migration.AppliesTo(content)
			})

			It("should return true", func() {
				Expect(applies).To(BeTrue())
			})
		})

		Describe("when content has table but no conflicting dot notation", func() {
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
		Describe("when content has simple dot notation conflict", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
inventory.container = "fireplace_cabinet_right"
[inventory]
items = []
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve the conflict by converting dot notation to table syntax", func() {
				expected := []byte(`+++
[inventory]
container = "fireplace_cabinet_right"
items = []
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content has multiple dot notation conflicts", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
inventory.container = "fireplace_cabinet_right"
inventory.location = "living_room"
[inventory]
items = []
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve all conflicts by merging into table syntax", func() {
				expected := []byte(`+++
[inventory]
container = "fireplace_cabinet_right"
location = "living_room"
items = []
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content has nested dot notation conflicts", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
game.inventory.container = "fireplace_cabinet_right"
[game.inventory]
items = []
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve nested conflicts correctly", func() {
				expected := []byte(`+++
[game.inventory]
container = "fireplace_cabinet_right"
items = []
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})

		Describe("when content has dot notation but no existing table", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
inventory.container = "fireplace_cabinet_right"
title = "test"
+++
content here`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert dot notation to table syntax", func() {
				expected := []byte(`+++
title = "test"
[inventory]
container = "fireplace_cabinet_right"
+++
content here`)
				Expect(result).To(Equal(expected))
			})
		})
	})
})
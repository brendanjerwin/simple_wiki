//revive:disable:dot-imports
package lazy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
)

var _ = Describe("InventoryContainerMungingMigration", func() {
	var migration *lazy.InventoryContainerMungingMigration

	BeforeEach(func() {
		migration = lazy.NewInventoryContainerMungingMigration()
	})

	It("should exist", func() {
		Expect(migration).NotTo(BeNil())
	})

	Describe("SupportedTypes", func() {
		var supportedTypes []lazy.FrontmatterType

		BeforeEach(func() {
			supportedTypes = migration.SupportedTypes()
		})

		It("should contain exactly one type", func() {
			Expect(supportedTypes).To(HaveLen(1))
		})

		It("should support TOML frontmatter", func() {
			Expect(supportedTypes[0]).To(Equal(lazy.FrontmatterTOML))
		})
	})

	Describe("AppliesTo", func() {
		When("TOML has inventory.container with unmunged value", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[inventory]
container = 'GarageInventory'
items = ['hammer', 'screwdriver']
+++

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should apply", func() {
				Expect(applies).To(BeTrue())
			})
		})

		When("TOML has inventory.container with already munged value", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[inventory]
container = 'garage_inventory'
items = ['hammer', 'screwdriver']
+++

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		When("TOML has no inventory section", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'
+++

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		When("TOML has inventory but no container", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[inventory]
items = ['hammer', 'screwdriver']
+++

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		When("content is not TOML", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`---
identifier: test_page
inventory:
  container: GarageInventory
---

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		When("container produces empty result after munging", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'

[inventory]
container = '///'
+++`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply because MungeIdentifier fails", func() {
				Expect(applies).To(BeFalse())
			})
		})
	})

	Describe("Apply", func() {
		When("inventory.container needs munging", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[inventory]
container = 'GarageInventory'
items = ['hammer', 'screwdriver']
+++

# Test Page Content`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should munge the container value", func() {
				Expect(string(result)).To(ContainSubstring(`container = 'garage_inventory'`))
			})

			It("should not change GarageInventory in the content", func() {
				Expect(string(result)).NotTo(ContainSubstring(`container = 'GarageInventory'`))
			})

			It("should preserve other fields", func() {
				Expect(string(result)).To(ContainSubstring(`identifier = 'test_page'`))
				Expect(string(result)).To(ContainSubstring(`title = 'Test Page'`))
				Expect(string(result)).To(ContainSubstring(`items = ['hammer', 'screwdriver']`))
			})

			It("should preserve body content", func() {
				Expect(string(result)).To(ContainSubstring("# Test Page Content"))
			})
		})

		DescribeTable("munging various casing patterns",
			func(input, expected string) {
				content := []byte("+++\n[inventory]\ncontainer = '" + input + "'\n+++\n")
				result, err := migration.Apply(content)

				Expect(err).NotTo(HaveOccurred())
				Expect(string(result)).To(ContainSubstring("container = '" + expected + "'"))
			},
			Entry("KitchenCabinet -> kitchen_cabinet", "KitchenCabinet", "kitchen_cabinet"),
			Entry("MixedCASEExample -> mixed_case_example", "MixedCASEExample", "mixed_case_example"),
			Entry("SimpleTest -> simple_test", "SimpleTest", "simple_test"),
			Entry("ALLCAPS -> allcaps", "ALLCAPS", "allcaps"),
		)

		When("inventory.container is already munged", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[inventory]
container = 'garage_inventory'
items = ['hammer', 'screwdriver']
+++

# Test Page Content`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return unchanged content", func() {
				Expect(result).To(Equal(content))
			})
		})

		When("handling edge cases", func() {
			When("container value contains UUID", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
[inventory]
container = 'LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should lowercase the identifier with UUID", func() {
					Expect(string(result)).To(ContainSubstring(`container = 'labtub_61c0030e-00e3-47b5-a797-1ac01f8d05b1'`))
				})
			})

			When("container value is empty", func() {
				var content []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
[inventory]
container = ''
+++
`)
					_, err = migration.Apply(content)
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should include context about the empty container", func() {
					Expect(err.Error()).To(ContainSubstring("cannot munge container"))
				})
			})

			When("frontmatter has multiple sections", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'test_page'
title = 'Test Page'

[metadata]
author = 'John Doe'
date = '2024-01-01'

[inventory]
container = 'StorageRoom'
location = 'Building A'

[tags]
primary = ['tools', 'hardware']
+++

# Content`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge only the container value", func() {
					Expect(string(result)).To(ContainSubstring(`container = 'storage_room'`))
				})

				It("should preserve metadata section", func() {
					Expect(string(result)).To(ContainSubstring("[metadata]"))
					Expect(string(result)).To(ContainSubstring(`author = 'John Doe'`))
				})

				It("should preserve tags section", func() {
					Expect(string(result)).To(ContainSubstring("[tags]"))
					Expect(string(result)).To(ContainSubstring(`primary = ['tools', 'hardware']`))
				})

				It("should preserve other inventory fields", func() {
					Expect(string(result)).To(ContainSubstring(`location = 'Building A'`))
				})
			})
		})
	})
})
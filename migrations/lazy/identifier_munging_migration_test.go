//revive:disable:dot-imports
package lazy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
)

var _ = Describe("IdentifierMungingMigration", func() {
	var migration *lazy.IdentifierMungingMigration

	BeforeEach(func() {
		migration = lazy.NewIdentifierMungingMigration()
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
		Describe("when TOML has identifier with unmunged value", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'lab_smallparts_1A6'
title = 'Lab Small Parts Bin 1A6'
+++

# Lab Small Parts Content`)
				applies = migration.AppliesTo(content)
			})

			It("should apply", func() {
				Expect(applies).To(BeTrue())
			})
		})

		Describe("when TOML has identifier with already munged value", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'lab_smallparts_1a6'
title = 'Lab Small Parts Bin 1A6'
+++

# Lab Small Parts Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when TOML has no identifier field", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
title = 'Test Page'
author = 'John Doe'
+++

# Test Page Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content is not TOML", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`---
identifier: lab_smallparts_1A6
title: Lab Small Parts Bin 1A6
---

# Lab Small Parts Content`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})

		Describe("when content has malformed TOML", func() {
			var content []byte
			var applies bool

			BeforeEach(func() {
				content = []byte(`+++
identifier = "unclosed_string
title = 'Test'
+++`)
				applies = migration.AppliesTo(content)
			})

			It("should not apply", func() {
				Expect(applies).To(BeFalse())
			})
		})
	})

	Describe("Apply", func() {
		Describe("when identifier needs munging (primary test case)", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'lab_smallparts_1A6'
title = 'Lab Small Parts Bin 1A6'
+++

# Lab Small Parts Content`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should munge the identifier value", func() {
				Expect(string(result)).To(ContainSubstring(`identifier = 'lab_smallparts_1a6'`))
			})

			It("should not change the original casing in identifier", func() {
				Expect(string(result)).NotTo(ContainSubstring(`identifier = 'lab_smallparts_1A6'`))
			})

			It("should preserve other fields", func() {
				Expect(string(result)).To(ContainSubstring(`title = 'Lab Small Parts Bin 1A6'`))
			})

			It("should preserve body content", func() {
				Expect(string(result)).To(ContainSubstring("# Lab Small Parts Content"))
			})
		})

		Describe("when testing various casing patterns", func() {
			Describe("when identifier is lab_smallparts_1A6", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'lab_smallparts_1A6'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge to lab_smallparts_1a6", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'lab_smallparts_1a6'`))
				})
			})

			Describe("when identifier is KitchenCabinet", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'KitchenCabinet'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge to kitchen_cabinet", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'kitchen_cabinet'`))
				})
			})

			Describe("when identifier is MixedCASEExample", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'MixedCASEExample'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge to mixed_case_example", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'mixed_case_example'`))
				})
			})

			Describe("when identifier is SimpleTest", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'SimpleTest'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge to simple_test", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'simple_test'`))
				})
			})

			Describe("when identifier is ALLCAPS", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'ALLCAPS'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should munge to allcaps", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'allcaps'`))
				})
			})
		})

		Describe("when identifier is already munged", func() {
			var content []byte
			var result []byte
			var err error

			BeforeEach(func() {
				content = []byte(`+++
identifier = 'lab_smallparts_1a6'
title = 'Lab Small Parts Bin 1A6'
+++

# Lab Small Parts Content`)
				result, err = migration.Apply(content)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return unchanged content", func() {
				Expect(result).To(Equal(content))
			})
		})

		Describe("when handling edge cases", func() {
			Describe("when identifier value contains UUID", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1'
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should lowercase the identifier with UUID", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'labtub_61c0030e-00e3-47b5-a797-1ac01f8d05b1'`))
				})
			})

			Describe("when identifier value is empty", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = ''
+++
`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return unchanged content", func() {
					Expect(result).To(Equal(content))
				})
			})

			Describe("when frontmatter has multiple sections", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = 'TestPageID'
title = 'Test Page'

[metadata]
author = 'John Doe'
date = '2024-01-01'

[inventory]
container = 'storage_room'
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

				It("should munge only the identifier value", func() {
					Expect(string(result)).To(ContainSubstring(`identifier = 'test_page_id'`))
				})

				It("should preserve metadata section", func() {
					Expect(string(result)).To(ContainSubstring("[metadata]"))
					Expect(string(result)).To(ContainSubstring(`author = 'John Doe'`))
				})

				It("should preserve inventory section", func() {
					Expect(string(result)).To(ContainSubstring("[inventory]"))
					Expect(string(result)).To(ContainSubstring(`container = 'storage_room'`))
				})

				It("should preserve tags section", func() {
					Expect(string(result)).To(ContainSubstring("[tags]"))
					Expect(string(result)).To(ContainSubstring(`primary = ['tools', 'hardware']`))
				})
			})

			Describe("when no identifier field exists", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
title = 'Test Page'
author = 'John Doe'
+++

# Content`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return unchanged content", func() {
					Expect(result).To(Equal(content))
				})
			})

			Describe("when content has malformed TOML", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`+++
identifier = "unclosed_string
title = 'Test'
+++`)
					result, err = migration.Apply(content)
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should return original content", func() {
					Expect(result).To(Equal(content))
				})
			})

			Describe("when content has invalid format", func() {
				var content []byte
				var result []byte
				var err error

				BeforeEach(func() {
					content = []byte(`no frontmatter delimiters here`)
					result, err = migration.Apply(content)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return unchanged content", func() {
					Expect(result).To(Equal(content))
				})
			})
		})
	})
})
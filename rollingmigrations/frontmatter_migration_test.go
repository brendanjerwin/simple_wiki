package rollingmigrations_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

var _ = Describe("Frontmatter Migration for inventory.container", func() {
	var migrationApplicator *rollingmigrations.DefaultApplicator

	BeforeEach(func() {
		migrationApplicator = rollingmigrations.NewApplicator()
	})

	Describe("ApplyMigrations", func() {
		Describe("when content has inventory.container in dotted notation", func() {
			var (
				originalContent string
				migratedContent string
				err             error
			)

			BeforeEach(func() {
				originalContent = `+++
identifier = 'garage_unit_3_shelf_a'
title = 'Garage Unit 3, Shelf A'
inventory.container = "GarageInventory"
+++

# Garage Unit 3, Shelf A

This is the content.`

				var migratedBytes []byte
				migratedBytes, err = migrationApplicator.ApplyMigrations([]byte(originalContent))
				migratedContent = string(migratedBytes)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create [inventory] section", func() {
				Expect(migratedContent).To(ContainSubstring("[inventory]"))
			})

			It("should convert container value to munged format", func() {
				Expect(migratedContent).To(ContainSubstring(`container = 'garage_inventory'`))
			})

			It("should remove dotted notation", func() {
				Expect(migratedContent).NotTo(ContainSubstring("inventory.container"))
			})

			It("should preserve identifier field", func() {
				Expect(migratedContent).To(ContainSubstring(`identifier = 'garage_unit_3_shelf_a'`))
			})

			It("should preserve title field", func() {
				Expect(migratedContent).To(ContainSubstring(`title = 'Garage Unit 3, Shelf A'`))
			})

			It("should preserve body header", func() {
				Expect(migratedContent).To(ContainSubstring("# Garage Unit 3, Shelf A"))
			})

			It("should preserve body text", func() {
				Expect(migratedContent).To(ContainSubstring("This is the content."))
			})
		})

		Describe("when applying migration to basic inventory.container", func() {
			var (
				input  string
				result string
				err    error
			)

			BeforeEach(func() {
				input = `+++
identifier = "test1"
inventory.container = "TestContainer"
+++`

				var resultBytes []byte
				resultBytes, err = migrationApplicator.ApplyMigrations([]byte(input))
				result = string(resultBytes)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should migrate the content", func() {
				Expect(result).NotTo(Equal(input))
			})

			It("should contain [inventory] section", func() {
				Expect(result).To(ContainSubstring("[inventory]"))
			})

			It("should not contain dotted notation", func() {
				Expect(result).NotTo(ContainSubstring("inventory.container"))
			})
		})

		Describe("when applying migration to inventory.container with other inventory fields", func() {
			var (
				input  string
				result string
				err    error
			)

			BeforeEach(func() {
				input = `+++
identifier = "test2"
inventory.container = "TestContainer"
inventory.items = ['item1', 'item2']
+++`

				var resultBytes []byte
				resultBytes, err = migrationApplicator.ApplyMigrations([]byte(input))
				result = string(resultBytes)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should migrate the content", func() {
				Expect(result).NotTo(Equal(input))
			})

			It("should contain [inventory] section", func() {
				Expect(result).To(ContainSubstring("[inventory]"))
			})

			It("should not contain dotted notation", func() {
				Expect(result).NotTo(ContainSubstring("inventory.container"))
			})

			It("should preserve other inventory fields", func() {
				Expect(result).To(ContainSubstring(`items = ['item1', 'item2']`))
			})
		})

		Describe("when content is already in migrated format", func() {
			var (
				input  string
				result string
				err    error
			)

			BeforeEach(func() {
				input = `+++
identifier = "test3"

[inventory]
container = "TestContainer"
+++`

				var resultBytes []byte
				resultBytes, err = migrationApplicator.ApplyMigrations([]byte(input))
				result = string(resultBytes)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should contain [inventory] section", func() {
				Expect(result).To(ContainSubstring("[inventory]"))
			})
		})

		Describe("when content has mixed format with existing sections", func() {
			var (
				input  string
				result string
				err    error
			)

			BeforeEach(func() {
				input = `+++
identifier = "test4"
title = "Test"
inventory.container = "TestContainer"

[other]
field = 'value'
+++`

				var resultBytes []byte
				resultBytes, err = migrationApplicator.ApplyMigrations([]byte(input))
				result = string(resultBytes)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should migrate the content", func() {
				Expect(result).NotTo(Equal(input))
			})

			It("should contain [inventory] section", func() {
				Expect(result).To(ContainSubstring("[inventory]"))
			})

			It("should not contain dotted notation", func() {
				Expect(result).NotTo(ContainSubstring("inventory.container"))
			})

			It("should preserve other section header", func() {
				Expect(result).To(ContainSubstring("[other]"))
			})

			It("should preserve other section field", func() {
				Expect(result).To(ContainSubstring(`field = 'value'`))
			})
		})

		Describe("when migration is registered", func() {
			var (
				testContent string
				err         error
			)

			BeforeEach(func() {
				testContent = `+++
inventory.container = "Test"
+++`

				_, err = migrationApplicator.ApplyMigrations([]byte(testContent))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
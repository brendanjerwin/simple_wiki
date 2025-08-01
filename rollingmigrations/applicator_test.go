package rollingmigrations_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)


var _ = Describe("DefaultApplicator", func() {
	var applicator rollingmigrations.FrontmatterMigrationApplicator

	BeforeEach(func() {
		// Create empty applicator without default migrations for unit testing
		applicator = rollingmigrations.NewEmptyApplicator()
	})

	It("should exist", func() {
		Expect(applicator).NotTo(BeNil())
	})

	Describe("when applying migrations to content with no frontmatter", func() {
		var content []byte
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("just some content without frontmatter")
			result, err = applicator.ApplyMigrations(content)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the original content unchanged", func() {
			Expect(result).To(Equal(content))
		})
	})

	Describe("when applying migrations to YAML frontmatter", func() {
		var content []byte
		var migration *rollingmigrations.MockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("---\ntitle: test\n---\ncontent")
			migration = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterYAML},
				AppliesToResult:      true,
				ApplyResult:          []byte("---\ntitle: migrated\n---\ncontent"),
			}
			
			registry, ok := applicator.(rollingmigrations.FrontmatterMigrationRegistry)
			Expect(ok).To(BeTrue())
			registry.RegisterMigration(migration)
			
			result, err = applicator.ApplyMigrations(content)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the migrated content", func() {
			Expect(result).To(Equal(migration.ApplyResult))
		})
	})

	Describe("when applying migrations to TOML frontmatter", func() {
		var content []byte
		var migration *rollingmigrations.MockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				AppliesToResult:      true,
				ApplyResult:          []byte("+++\ntitle = \"migrated\"\n+++\ncontent"),
			}
			
			registry, ok := applicator.(rollingmigrations.FrontmatterMigrationRegistry)
			Expect(ok).To(BeTrue())
			registry.RegisterMigration(migration)
			
			result, err = applicator.ApplyMigrations(content)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the migrated content", func() {
			Expect(result).To(Equal(migration.ApplyResult))
		})
	})

	Describe("when migration doesn't apply to content", func() {
		var content []byte
		var migration *rollingmigrations.MockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				AppliesToResult:      false, // Migration doesn't apply
			}
			
			registry, ok := applicator.(rollingmigrations.FrontmatterMigrationRegistry)
			Expect(ok).To(BeTrue())
			registry.RegisterMigration(migration)
			
			result, err = applicator.ApplyMigrations(content)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the original content unchanged", func() {
			Expect(result).To(Equal(content))
		})
	})

	Describe("when migration fails", func() {
		var content []byte
		var migration *rollingmigrations.MockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				AppliesToResult:      true,
				ApplyError:           errors.New("migration failed"),
			}
			
			registry, ok := applicator.(rollingmigrations.FrontmatterMigrationRegistry)
			Expect(ok).To(BeTrue())
			registry.RegisterMigration(migration)
			
			result, err = applicator.ApplyMigrations(content)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("migration failed"))
		})

		It("should return the original content", func() {
			Expect(result).To(Equal(content))
		})
	})

	Describe("when migration doesn't support the frontmatter type", func() {
		var content []byte
		var migration *rollingmigrations.MockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterYAML}, // Only supports YAML
				AppliesToResult:      true,
				ApplyResult:          []byte("should not be applied"),
			}
			
			registry, ok := applicator.(rollingmigrations.FrontmatterMigrationRegistry)
			Expect(ok).To(BeTrue())
			registry.RegisterMigration(migration)
			
			result, err = applicator.ApplyMigrations(content)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the original content unchanged", func() {
			Expect(result).To(Equal(content))
		})
	})
})
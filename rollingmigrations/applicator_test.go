package rollingmigrations_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

type mockMigration struct {
	supportedTypes []rollingmigrations.FrontmatterType
	appliesTo      bool
	applyResult    []byte
	applyError     error
}

func (m *mockMigration) SupportedTypes() []rollingmigrations.FrontmatterType {
	return m.supportedTypes
}

func (m *mockMigration) AppliesTo(content []byte) bool {
	return m.appliesTo
}

func (m *mockMigration) Apply(content []byte) ([]byte, error) {
	if m.applyError != nil {
		return nil, m.applyError
	}
	return m.applyResult, nil
}

var _ = Describe("DefaultApplicator", func() {
	var applicator rollingmigrations.FrontmatterMigrationApplicator

	BeforeEach(func() {
		applicator = rollingmigrations.NewDefaultApplicator()
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
		var migration *mockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("---\ntitle: test\n---\ncontent")
			migration = &mockMigration{
				supportedTypes: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterYAML},
				appliesTo:      true,
				applyResult:    []byte("---\ntitle: migrated\n---\ncontent"),
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
			Expect(result).To(Equal(migration.applyResult))
		})
	})

	Describe("when applying migrations to TOML frontmatter", func() {
		var content []byte
		var migration *mockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &mockMigration{
				supportedTypes: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				appliesTo:      true,
				applyResult:    []byte("+++\ntitle = \"migrated\"\n+++\ncontent"),
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
			Expect(result).To(Equal(migration.applyResult))
		})
	})

	Describe("when migration doesn't apply to content", func() {
		var content []byte
		var migration *mockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &mockMigration{
				supportedTypes: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				appliesTo:      false, // Migration doesn't apply
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
		var migration *mockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &mockMigration{
				supportedTypes: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				appliesTo:      true,
				applyError:     errors.New("migration failed"),
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
		var migration *mockMigration
		var result []byte
		var err error

		BeforeEach(func() {
			content = []byte("+++\ntitle = \"test\"\n+++\ncontent")
			migration = &mockMigration{
				supportedTypes: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterYAML}, // Only supports YAML
				appliesTo:      true,
				applyResult:    []byte("should not be applied"),
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
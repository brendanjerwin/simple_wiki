package rollingmigrations_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
)

var _ = Describe("FrontmatterType", func() {
	It("should exist", func() {
		var fmType rollingmigrations.FrontmatterType
		Expect(fmType).To(Equal(rollingmigrations.FrontmatterUnknown))
	})

	Describe("constants", func() {
		It("should define frontmatter type constants", func() {
			Expect(rollingmigrations.FrontmatterUnknown).To(Equal(rollingmigrations.FrontmatterType(0)))
			Expect(rollingmigrations.FrontmatterYAML).To(Equal(rollingmigrations.FrontmatterType(1)))
			Expect(rollingmigrations.FrontmatterTOML).To(Equal(rollingmigrations.FrontmatterType(2)))
			Expect(rollingmigrations.FrontmatterJSON).To(Equal(rollingmigrations.FrontmatterType(3)))
		})
	})
})

var _ = Describe("FrontmatterMigration", func() {
	It("should exist as an interface", func() {
		var migration rollingmigrations.FrontmatterMigration
		Expect(migration).To(BeNil())
	})
})

var _ = Describe("FrontmatterMigrationRegistry", func() {
	It("should exist as an interface", func() {
		var registry rollingmigrations.FrontmatterMigrationRegistry
		Expect(registry).To(BeNil())
	})
})

var _ = Describe("FrontmatterMigrationApplicator", func() {
	It("should exist as an interface", func() {
		var applicator rollingmigrations.FrontmatterMigrationApplicator
		Expect(applicator).To(BeNil())
	})
})
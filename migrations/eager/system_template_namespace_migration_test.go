//revive:disable:dot-imports
package eager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("SystemTemplateNamespaceMigrationJob", func() {
	var (
		store *fakeReaderMutator
		job   *eager.SystemTemplateNamespaceMigrationJob
	)

	When("the page has only the legacy system flag", func() {
		var migrationErr error

		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"my_page": {
					"identifier": "my_page",
					"system":     true,
				},
			})
			job = eager.NewSystemTemplateNamespaceMigrationJob(store, "my_page")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should remove the legacy top-level system key", func() {
			Expect(store.pages["my_page"]).NotTo(HaveKey("system"))
		})

		It("should set wiki.system = true", func() {
			wikiSubtree := asMap(store.pages["my_page"]["wiki"])
			Expect(wikiSubtree["system"]).To(Equal(true))
		})

		It("should stamp wiki.migrated_namespaces = true", func() {
			wikiSubtree := asMap(store.pages["my_page"]["wiki"])
			Expect(wikiSubtree["migrated_namespaces"]).To(Equal(true))
		})
	})

	When("the page has only the legacy template flag", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"tmpl": {
					"identifier": "tmpl",
					"template":   true,
				},
			})
			job = eager.NewSystemTemplateNamespaceMigrationJob(store, "tmpl")
			Expect(job.Execute()).To(Succeed())
		})

		It("should remove the legacy top-level template key", func() {
			Expect(store.pages["tmpl"]).NotTo(HaveKey("template"))
		})

		It("should set wiki.template = true", func() {
			wikiSubtree := asMap(store.pages["tmpl"]["wiki"])
			Expect(wikiSubtree["template"]).To(Equal(true))
		})
	})

	When("the page has both legacy flags", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"both": {
					"identifier": "both",
					"system":     true,
					"template":   true,
				},
			})
			job = eager.NewSystemTemplateNamespaceMigrationJob(store, "both")
			Expect(job.Execute()).To(Succeed())
		})

		It("should migrate both into wiki.*", func() {
			wikiSubtree := asMap(store.pages["both"]["wiki"])
			Expect(wikiSubtree["system"]).To(Equal(true))
			Expect(wikiSubtree["template"]).To(Equal(true))
		})

		It("should remove both legacy keys", func() {
			Expect(store.pages["both"]).NotTo(HaveKey("system"))
			Expect(store.pages["both"]).NotTo(HaveKey("template"))
		})
	})

	When("the new key disagrees with the legacy key", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"conflict": {
					"identifier": "conflict",
					"system":     false,
					"wiki": map[string]any{
						"system": true,
					},
				},
			})
			job = eager.NewSystemTemplateNamespaceMigrationJob(store, "conflict")
			Expect(job.Execute()).To(Succeed())
		})

		It("should keep the new key (no surprise downgrade)", func() {
			wikiSubtree := asMap(store.pages["conflict"]["wiki"])
			Expect(wikiSubtree["system"]).To(Equal(true))
		})

		It("should still delete the legacy key", func() {
			Expect(store.pages["conflict"]).NotTo(HaveKey("system"))
		})
	})
})


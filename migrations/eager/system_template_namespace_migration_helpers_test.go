//revive:disable:dot-imports
package eager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pageNeedsNamespaceMigration", func() {
	When("a page has no legacy keys and is already migrated", func() {
		It("should be skipped by the scan", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"system":              true,
					"migrated_namespaces": true,
				},
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeFalse())
		})
	})

	When("a page has a legacy system key", func() {
		It("should be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"system":     true,
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeTrue())
		})
	})

	When("a page has a legacy template key", func() {
		It("should be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"template":   true,
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeTrue())
		})
	})

	When("a brand-new page has only wiki.system (no migration marker, no legacy)", func() {
		It("should be skipped", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"system": true,
				},
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeFalse())
		})
	})
})

package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("IsTemplatePage", func() {
	When("wiki.template is bool true", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": true}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.template is bool false", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": false}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("wiki.template is the string 'true' (mixed case)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": "TrUe"}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.template is a non-zero int64", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": int64(1)}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.template is a non-zero float64", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": 1.5}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.template key is missing", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": true}}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the wiki field is not a map (defensive)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": "garbage"}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("only the legacy top-level template key is set (true)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"template": true}
			result = wikipage.IsTemplatePage(fm)
		})

		It("should be false (legacy top-level flag is ignored; eager migration moves it)", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the frontmatter is nil", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsTemplatePage(nil)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})
})

var _ = Describe("IsSystemPage", func() {
	When("wiki.system is bool true", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": true}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.system is bool false", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": false}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("wiki.system is the string 'true' (mixed case)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": "TRUE"}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.system is a non-zero int64", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": int64(1)}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.system is a non-zero float64", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"system": 1.5}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("wiki.system key is missing", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": map[string]any{"template": true}}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the wiki field is not a map (defensive)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"wiki": "garbage"}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("only the legacy top-level system key is set (true)", func() {
		var result bool

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"system": true}
			result = wikipage.IsSystemPage(fm)
		})

		It("should be false (legacy top-level flag is ignored; eager migration moves it)", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the frontmatter is nil", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsSystemPage(nil)
		})

		It("should be false", func() {
			Expect(result).To(BeFalse())
		})
	})
})

var _ = Describe("ApplyTemplate", func() {
	When("src has only non-reserved keys", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{}
			src := wikipage.FrontMatter{
				"title":       "Hello",
				"description": "world",
			}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should copy title", func() {
			Expect(dest["title"]).To(Equal("Hello"))
		})

		It("should copy description", func() {
			Expect(dest["description"]).To(Equal("world"))
		})
	})

	When("src contains the identifier key", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{"identifier": "newpage"}
			src := wikipage.FrontMatter{
				"identifier": "templatepage",
				"title":      "Title",
			}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should not overwrite the destination identifier", func() {
			Expect(dest["identifier"]).To(Equal("newpage"))
		})

		It("should copy non-reserved keys", func() {
			Expect(dest["title"]).To(Equal("Title"))
		})
	})

	When("src contains a wiki.* reserved subtree", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{}
			src := wikipage.FrontMatter{
				"title": "Hi",
				"wiki": map[string]any{
					"template": true,
					"system":   true,
				},
			}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should not copy the wiki subtree", func() {
			Expect(dest).NotTo(HaveKey("wiki"))
		})

		It("should still copy non-reserved keys", func() {
			Expect(dest["title"]).To(Equal("Hi"))
		})
	})

	When("src contains an agent.* reserved subtree", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{}
			src := wikipage.FrontMatter{
				"title": "Hi",
				"agent": map[string]any{"schedules": []any{"sched"}},
			}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should not copy the agent subtree", func() {
			Expect(dest).NotTo(HaveKey("agent"))
		})
	})

	When("dest already has a reserved-namespace value", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{
				"wiki": map[string]any{"existing": "value"},
			}
			src := wikipage.FrontMatter{
				"title": "T",
				"wiki":  map[string]any{"template": true},
			}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should preserve the existing reserved value untouched", func() {
			wikiSubtree, ok := dest["wiki"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(wikiSubtree["existing"]).To(Equal("value"))
			Expect(wikiSubtree).NotTo(HaveKey("template"))
		})
	})

	When("src has a non-reserved key that dest also has", func() {
		var dest wikipage.FrontMatter

		BeforeEach(func() {
			dest = wikipage.FrontMatter{"title": "Old"}
			src := wikipage.FrontMatter{"title": "New"}
			wikipage.ApplyTemplate(dest, src)
		})

		It("should overwrite the destination value", func() {
			Expect(dest["title"]).To(Equal("New"))
		})
	})
})

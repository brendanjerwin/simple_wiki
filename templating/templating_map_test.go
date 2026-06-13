//revive:disable:dot-imports
package templating_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("BuildMap", func() {
	var (
		mapFunc func(string) string
		result  string
	)

	BeforeEach(func() {
		mapFunc = templating.BuildMap(templating.TemplateContext{
			Identifier: "garden_plan",
			Map:        map[string]any{},
		})
	})

	It("should exist", func() {
		Expect(mapFunc).NotTo(BeNil())
	})

	Describe("when rendering a map", func() {
		BeforeEach(func() {
			result = mapFunc("yard")
		})

		It("should return a wiki-map element", func() {
			Expect(result).To(ContainSubstring("<wiki-map"))
		})

		It("should include the name attribute", func() {
			Expect(result).To(ContainSubstring(`name="yard"`))
		})

		It("should include the page attribute", func() {
			Expect(result).To(ContainSubstring(`page="garden_plan"`))
		})

		It("should close the wiki-map element", func() {
			Expect(result).To(ContainSubstring("</wiki-map>"))
		})
	})

	Describe("when map name and page contain special HTML characters", func() {
		BeforeEach(func() {
			mapFunc = templating.BuildMap(templating.TemplateContext{
				Identifier: `page&"name"`,
				Map:        map[string]any{},
			})
			result = mapFunc(`map&"name"`)
		})

		It("should escape the name attribute", func() {
			Expect(result).To(ContainSubstring(`name="map&amp;&#34;name&#34;"`))
		})

		It("should escape the page attribute", func() {
			Expect(result).To(ContainSubstring(`page="page&amp;&#34;name&#34;"`))
		})
	})

	Describe("when used in full template execution", func() {
		var (
			mockSite   *mockPageReader
			mockIndex  *mockFrontmatterIndex
			execResult []byte
			execErr    error
		)

		BeforeEach(func() {
			mockSite = &mockPageReader{
				pages: map[string]wikipage.FrontMatter{},
			}
			mockIndex = &mockFrontmatterIndex{
				index:  map[string]map[string][]wikipage.PageIdentifier{},
				values: map[string]map[string]string{},
			}

			execResult, execErr = templating.ExecuteTemplate(
				`{{ Map "yard" }}`,
				wikipage.FrontMatter{
					"identifier": "garden_plan",
					"title":      "Garden Plan",
				},
				mockSite,
				mockIndex,
			)
		})

		It("should not return an error", func() {
			Expect(execErr).NotTo(HaveOccurred())
		})

		It("should render the wiki-map element", func() {
			Expect(string(execResult)).To(Equal(`<wiki-map name="yard" page="garden_plan"></wiki-map>`))
		})
	})
})

var _ = Describe("ValidateTemplate with Map", func() {
	Describe("when template uses Map macro", func() {
		It("should not return an error", func() {
			err := templating.ValidateTemplate(`{{ Map "yard" }}`)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when template uses lowercase map", func() {
		It("should return an error suggesting Map", func() {
			err := templating.ValidateTemplate(`{{ map "yard" }}`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`"Map"`))
		})
	})

	Describe("when template uses the legacy MapEmbed macro", func() {
		It("should not return an error", func() {
			err := templating.ValidateTemplate(`{{ MapEmbed "https://www.google.com/maps/embed?pb=abc" }}`)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

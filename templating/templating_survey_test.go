//revive:disable:dot-imports
package templating_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/templating"
)

var _ = Describe("BuildSurvey", func() {
	var (
		surveyFunc func(string) string
		result     string
	)

	BeforeEach(func() {
		surveyFunc = templating.BuildSurvey(templating.TemplateContext{
			Identifier: "my_page",
			Map:        map[string]any{},
		})
	})

	It("should exist", func() {
		Expect(surveyFunc).NotTo(BeNil())
	})

	Describe("when there is no survey config in frontmatter", func() {
		BeforeEach(func() {
			result = surveyFunc("my_survey")
		})

		It("should return a wiki-survey element", func() {
			Expect(result).To(ContainSubstring("<wiki-survey"))
		})

		It("should include the name attribute", func() {
			Expect(result).To(ContainSubstring(`name="my_survey"`))
		})

		It("should include the page attribute", func() {
			Expect(result).To(ContainSubstring(`page="my_page"`))
		})

		It("should close the wiki-survey element", func() {
			Expect(result).To(ContainSubstring("</wiki-survey>"))
		})
	})

	Describe("when the survey has a question but no responses", func() {
		BeforeEach(func() {
			surveyFunc = templating.BuildSurvey(templating.TemplateContext{
				Identifier: "pastry_project",
				Map: map[string]any{
					"surveys": map[string]any{
						"pastry_2026_w15": map[string]any{
							"question": "Rate this week's pastries (1-5) and any notes",
							"fields": []any{
								map[string]any{"name": "rating", "type": "number", "min": 1, "max": 5},
								map[string]any{"name": "notes", "type": "text"},
							},
						},
					},
				},
			})
			result = surveyFunc("pastry_2026_w15")
		})

		It("should render the question in the fallback", func() {
			Expect(result).To(ContainSubstring("Rate this week&#39;s pastries"))
		})

		It("should include the name attribute", func() {
			Expect(result).To(ContainSubstring(`name="pastry_2026_w15"`))
		})

		It("should include the page attribute", func() {
			Expect(result).To(ContainSubstring(`page="pastry_project"`))
		})
	})

	Describe("when the survey has responses", func() {
		BeforeEach(func() {
			surveyFunc = templating.BuildSurvey(templating.TemplateContext{
				Identifier: "pastry_project",
				Map: map[string]any{
					"surveys": map[string]any{
						"pastry_2026_w15": map[string]any{
							"question": "Rate this week's pastries",
							"fields": []any{
								map[string]any{"name": "rating", "type": "number", "min": 1, "max": 5},
							},
							"responses": []any{
								map[string]any{
									"user":         "brendan",
									"anonymous":    false,
									"submitted_at": "2026-04-18T14:22:00Z",
									"values":       map[string]any{"rating": 4},
								},
							},
						},
					},
				},
			})
			result = surveyFunc("pastry_2026_w15")
		})

		It("should render the response user in the fallback", func() {
			Expect(result).To(ContainSubstring("brendan"))
		})

		It("should render the response date in the fallback", func() {
			Expect(result).To(ContainSubstring("2026-04-18"))
		})
	})

	Describe("when survey name contains special HTML characters", func() {
		BeforeEach(func() {
			surveyFunc = templating.BuildSurvey(templating.TemplateContext{
				Identifier: `page&"name"`,
				Map:        map[string]any{},
			})
			result = surveyFunc(`survey&"name"`)
		})

		It("should escape the name attribute", func() {
			Expect(result).To(ContainSubstring(`name="survey&amp;&#34;name&#34;"`))
		})

		It("should escape the page attribute", func() {
			Expect(result).To(ContainSubstring(`page="page&amp;&#34;name&#34;"`))
		})
	})
})

var _ = Describe("ValidateTemplate with Survey", func() {
	Describe("when template uses Survey macro", func() {
		It("should not return an error", func() {
			err := templating.ValidateTemplate(`{{Survey "my_survey"}}`)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when template uses lowercase survey", func() {
		It("should return an error suggesting Survey", func() {
			err := templating.ValidateTemplate(`{{survey "my_survey"}}`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`"Survey"`))
		})
	})
})

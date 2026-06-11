//revive:disable:dot-imports
package eager_test

import (
	"errors"

	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SurveyDataModelMigrationJob", func() {
	var (
		store        *fakeReaderMutator
		job          *eager.SurveyDataModelMigrationJob
		migrationErr error
	)

	When("the page has legacy survey data outside the wiki namespace", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": {
					"identifier": "weekly_menu",
					"surveys": map[string]any{
						"meal": map[string]any{
							"question": "Dinner?",
							"fields": []any{
								map[string]any{"name": "choice", "type": "text"},
							},
							"responses": []any{
								map[string]any{
									"user":         "alice@example.com",
									"submitted_at": "2026-06-09T19:00:00Z",
									"values":       map[string]any{"choice": "pasta"},
								},
							},
						},
					},
				},
			})
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should remove the legacy surveys subtree", func() {
			Expect(store.pages["weekly_menu"]).NotTo(HaveKey("surveys"))
		})

		It("should move survey configuration into wiki.surveys", func() {
			meal := migratedSurvey(store, "weekly_menu", "meal")
			Expect(meal).To(HaveKeyWithValue("question", "Dinner?"))
			Expect(asAnySlice(meal["fields"])).To(HaveLen(1))
		})

		It("should move survey responses into wiki.surveys", func() {
			meal := migratedSurvey(store, "weekly_menu", "meal")
			responses := asAnySlice(meal["responses"])
			Expect(asMap(responses[0])).To(HaveKeyWithValue("user", "alice@example.com"))
		})

		It("should stamp the migrated data model flag", func() {
			meal := migratedSurvey(store, "weekly_menu", "meal")
			Expect(meal).To(HaveKeyWithValue("migrated_data_model", true))
		})
	})

	When("the page is already migrated", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": {
					"identifier": "weekly_menu",
					"wiki": map[string]any{
						"surveys": map[string]any{
							"meal": map[string]any{
								"question":            "Dinner?",
								"migrated_data_model": true,
							},
						},
					},
				},
			})
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should not write frontmatter", func() {
			Expect(store.writeCount).To(Equal(0))
		})
	})

	When("a namespaced survey already exists with the same name", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": {
					"identifier": "weekly_menu",
					"surveys": map[string]any{
						"meal": map[string]any{
							"question": "Legacy dinner?",
							"responses": []any{
								map[string]any{"user": "legacy@example.com"},
							},
						},
						"snack": map[string]any{
							"question": "Snack?",
						},
					},
					"wiki": map[string]any{
						"surveys": map[string]any{
							"meal": map[string]any{
								"question":            "Namespaced dinner?",
								"migrated_data_model": true,
								"responses": []any{
									map[string]any{"user": "current@example.com"},
								},
							},
						},
					},
				},
			})
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should preserve the namespaced survey", func() {
			meal := migratedSurvey(store, "weekly_menu", "meal")
			Expect(meal).To(HaveKeyWithValue("question", "Namespaced dinner?"))
			responses := asAnySlice(meal["responses"])
			Expect(asMap(responses[0])).To(HaveKeyWithValue("user", "current@example.com"))
		})

		It("should migrate non-conflicting legacy surveys", func() {
			snack := migratedSurvey(store, "weekly_menu", "snack")
			Expect(snack).To(HaveKeyWithValue("question", "Snack?"))
			Expect(snack).To(HaveKeyWithValue("migrated_data_model", true))
		})

		It("should remove the legacy surveys subtree", func() {
			Expect(store.pages["weekly_menu"]).NotTo(HaveKey("surveys"))
		})
	})

	When("reading frontmatter fails", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{})
			store.readErr = errors.New("read failed")
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should return a wrapped read error", func() {
			Expect(migrationErr).To(MatchError(ContainSubstring("read frontmatter for weekly_menu")))
		})

		It("should not write frontmatter", func() {
			Expect(store.writeCount).To(Equal(0))
		})
	})

	When("the page frontmatter is nil", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": nil,
			})
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should not write frontmatter", func() {
			Expect(store.writeCount).To(Equal(0))
		})
	})

	When("writing migrated frontmatter fails", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": {
					"identifier": "weekly_menu",
					"surveys": map[string]any{
						"meal": map[string]any{"question": "Dinner?"},
					},
				},
			})
			store.modifyErr = errors.New("write failed")
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
		})

		It("should return a wrapped write error", func() {
			Expect(migrationErr).To(MatchError(ContainSubstring("write migrated frontmatter for weekly_menu")))
		})
	})

	When("a survey contains nested frontmatter values", func() {
		var originalNested wikipage.FrontMatter

		BeforeEach(func() {
			originalNested = wikipage.FrontMatter{"choice": "pasta"}
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"weekly_menu": {
					"identifier": "weekly_menu",
					"surveys": map[string]any{
						"meal": map[string]any{
							"question": "Dinner?",
							"values":   originalNested,
						},
					},
				},
			})
			job = eager.NewSurveyDataModelMigrationJob(store, "weekly_menu")
			migrationErr = job.Execute()
			originalNested["choice"] = "mutated"
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should clone nested frontmatter values", func() {
			meal := migratedSurvey(store, "weekly_menu", "meal")
			values := asMap(meal["values"])
			Expect(values).To(HaveKeyWithValue("choice", "pasta"))
		})
	})
})

func migratedSurvey(store *fakeReaderMutator, page, name string) map[string]any {
	return asMap(asMap(asMap(store.pages[page]["wiki"])["surveys"])[name])
}

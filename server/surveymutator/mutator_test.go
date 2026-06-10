package surveymutator

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type fakeStore struct {
	mu     sync.Mutex
	pages  map[wikipage.PageIdentifier]wikipage.FrontMatter
	writes int
}

func newFakeStore() *fakeStore {
	return &fakeStore{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (s *fakeStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, ok := s.pages[id]
	if !ok {
		return "", nil, errors.New("missing")
	}
	return id, cloneFrontMatter(fm), nil
}

func (s *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[id] = cloneFrontMatter(fm)
	s.writes++
	return nil
}

func (*fakeStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (*fakeStore) WriteMarkdown(wikipage.PageIdentifier, wikipage.Markdown) error {
	return nil
}

func (*fakeStore) DeletePage(wikipage.PageIdentifier) error {
	return nil
}

func (*fakeStore) ModifyMarkdown(id wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	_, err := modifier("")
	return err
}

func cloneFrontMatter(fm wikipage.FrontMatter) wikipage.FrontMatter {
	out := wikipage.FrontMatter{}
	for k, v := range fm {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, nested := range typed {
			out[k] = cloneAny(nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, cloneAny(nested))
		}
		return out
	default:
		return typed
	}
}

var _ = Describe("Mutator", func() {
	var (
		store   *fakeStore
		mutator *Mutator
		now     time.Time
		result  *apiv1.Survey
		err     error
	)

	BeforeEach(func() {
		now = time.Date(2026, 6, 9, 20, 20, 0, 0, time.UTC)
		store = newFakeStore()
		store.pages["weekly_menu"] = wikipage.FrontMatter{
			"surveys": map[string]any{
				"meal": map[string]any{
					"question": "Dinner?",
					"fields": []any{
						map[string]any{"name": "choice", "type": "text"},
					},
					"responses": []any{
						map[string]any{
							"user":         "a@example.com",
							"submitted_at": "2026-06-09T19:00:00Z",
							"values": map[string]any{
								"choice": "pasta",
							},
						},
					},
					"updated_at": "2026-06-09T19:00:00Z",
				},
				"other": map[string]any{
					"question": "Untouched?",
				},
			},
		}
		mutator = New(store, fixedClock{now: now})
	})

	Describe("UpdateField", func() {
		var writtenFrontmatter wikipage.FrontMatter

		BeforeEach(func() {
			result, err = mutator.UpdateField(
				context.Background(),
				"weekly_menu",
				"meal",
				"choice",
				&apiv1.SurveyField{Name: "entree", Type: "text"},
				nil,
			)
			_, writtenFrontmatter, _ = store.ReadFrontMatter("weekly_menu")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve existing responses", func() {
			Expect(result.GetResponses()).To(HaveLen(1))
			Expect(result.GetResponses()[0].GetUser()).To(Equal("a@example.com"))
			Expect(result.GetResponses()[0].GetValues().AsMap()).To(HaveKeyWithValue("choice", "pasta"))
		})

		It("should update only the named survey", func() {
			other := readMap(readMap(writtenFrontmatter, surveysKey), "other")
			Expect(other).To(HaveKeyWithValue(questionKey, "Untouched?"))
		})
	})

	Describe("SubmitResponse", func() {
		var identity tailscale.IdentityValue

		BeforeEach(func() {
			values, valuesErr := structpb.NewStruct(map[string]any{"choice": "tacos"})
			Expect(valuesErr).NotTo(HaveOccurred())
			identity = tailscale.NewIdentity("b@example.com", "B", "node")
			result, err = mutator.SubmitResponse(context.Background(), "weekly_menu", "meal", values, false, identity)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the caller response", func() {
			Expect(result.GetResponses()).To(HaveLen(2))
			Expect(result.GetResponses()[1].GetUser()).To(Equal("b@example.com"))
			Expect(result.GetResponses()[1].GetValues().AsMap()).To(HaveKeyWithValue("choice", "tacos"))
		})

		It("should server-stamp the submission time", func() {
			Expect(result.GetResponses()[1].GetSubmittedAt().AsTime()).To(Equal(now))
		})
	})

	Describe("UpsertSurvey", func() {
		var closed bool

		BeforeEach(func() {
			closed = true
			result, err = mutator.UpsertSurvey(context.Background(), "weekly_menu", "snacks", "Snack preference?", &closed, nil)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create the survey shell", func() {
			Expect(result.GetName()).To(Equal("snacks"))
			Expect(result.GetQuestion()).To(Equal("Snack preference?"))
			Expect(result.GetClosed()).To(BeTrue())
		})

		It("should stamp updated_at", func() {
			Expect(result.GetUpdatedAt().AsTime()).To(Equal(now))
		})
	})

	Describe("AddField", func() {
		BeforeEach(func() {
			result, err = mutator.AddField(
				context.Background(),
				"weekly_menu",
				"meal",
				&apiv1.SurveyField{Name: "side", Type: "choice", Options: []string{"salad", "bread"}},
				nil,
			)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the field", func() {
			Expect(result.GetFields()).To(HaveLen(2))
			Expect(result.GetFields()[1].GetName()).To(Equal("side"))
		})
	})

	Describe("RemoveField", func() {
		BeforeEach(func() {
			result, err = mutator.RemoveField(context.Background(), "weekly_menu", "meal", "choice", nil)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the requested field", func() {
			Expect(result.GetFields()).To(BeEmpty())
		})
	})

	Describe("ReorderField", func() {
		BeforeEach(func() {
			_, addErr := mutator.AddField(context.Background(), "weekly_menu", "meal", &apiv1.SurveyField{Name: "side", Type: "text"}, nil)
			Expect(addErr).NotTo(HaveOccurred())
			result, err = mutator.ReorderField(context.Background(), "weekly_menu", "meal", "side", 0, nil)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should move the field to the requested index", func() {
			Expect(result.GetFields()[0].GetName()).To(Equal("side"))
		})
	})

	Describe("DeleteResponse", func() {
		BeforeEach(func() {
			result, err = mutator.DeleteResponse(context.Background(), "weekly_menu", "meal", "a@example.com", nil, nil)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the matching response", func() {
			Expect(result.GetResponses()).To(BeEmpty())
		})
	})

	Describe("ListResponses", func() {
		var responses []*apiv1.SurveyResponse

		BeforeEach(func() {
			responses, err = mutator.ListResponses(context.Background(), "weekly_menu", "meal")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the existing responses", func() {
			Expect(responses).To(HaveLen(1))
		})
	})

	Describe("when the expected updated_at is stale", func() {
		BeforeEach(func() {
			stale := time.Date(2026, 6, 9, 18, 0, 0, 0, time.UTC)
			result, err = mutator.UpsertSurvey(context.Background(), "weekly_menu", "meal", "New?", nil, &stale)
		})

		It("should return a failed precondition error", func() {
			Expect(err).To(MatchError(ContainSubstring("expected_updated_at mismatch")))
		})

		It("should not write frontmatter", func() {
			Expect(store.writes).To(Equal(0))
		})
	})

	Describe("when a response is submitted without an authenticated user", func() {
		BeforeEach(func() {
			result, err = mutator.SubmitResponse(context.Background(), "weekly_menu", "meal", nil, false, tailscale.Anonymous)
		})

		It("should return PermissionDenied", func() {
			Expect(err).To(MatchError(ContainSubstring("authenticated user is required")))
		})

		It("should not return a survey", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when adding a duplicate field", func() {
		BeforeEach(func() {
			result, err = mutator.AddField(context.Background(), "weekly_menu", "meal", &apiv1.SurveyField{Name: "choice", Type: "text"}, nil)
		})

		It("should return ErrFieldExists", func() {
			Expect(err).To(MatchError(ErrFieldExists))
		})

		It("should not return a survey", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when reordering outside the field range", func() {
		BeforeEach(func() {
			result, err = mutator.ReorderField(context.Background(), "weekly_menu", "meal", "choice", 99, nil)
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(MatchError(ContainSubstring("new_index is out of range")))
		})

		It("should not return a survey", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when deleting a missing response", func() {
		BeforeEach(func() {
			result, err = mutator.DeleteResponse(context.Background(), "weekly_menu", "meal", "missing@example.com", nil, nil)
		})

		It("should return ErrResponseNotFound", func() {
			Expect(err).To(MatchError(ErrResponseNotFound))
		})

		It("should not return a survey", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when replacing the same user's response", func() {
		BeforeEach(func() {
			values, valuesErr := structpb.NewStruct(map[string]any{"choice": "salad"})
			Expect(valuesErr).NotTo(HaveOccurred())
			identity := tailscale.NewIdentity("a@example.com", "A", "node")
			result, err = mutator.SubmitResponse(context.Background(), "weekly_menu", "meal", values, false, identity)
		})

		It("should keep a single response for that user", func() {
			Expect(result.GetResponses()).To(HaveLen(1))
		})

		It("should replace the response values", func() {
			Expect(result.GetResponses()[0].GetValues().AsMap()).To(HaveKeyWithValue("choice", "salad"))
		})
	})
})

var _ = Describe("codec", func() {
	Describe("decodeSurvey", func() {
		var survey *apiv1.Survey

		BeforeEach(func() {
			submittedAt := timestamppb.New(time.Date(2026, 6, 9, 19, 0, 0, 0, time.UTC))
			fm := wikipage.FrontMatter{}
			encodeSurvey(fm, "meal", &apiv1.Survey{
				Name:     "meal",
				Question: "Dinner?",
				Fields: []*apiv1.SurveyField{
					{Name: "choice", Type: "choice", Options: []string{"pasta", "tacos"}},
				},
				Responses: []*apiv1.SurveyResponse{
					{
						User:        "a@example.com",
						SubmittedAt: submittedAt,
						Values:      structFromMap(map[string]any{"choice": "pasta"}),
					},
				},
				UpdatedAt: submittedAt,
			})
			survey = decodeSurvey(fm, "meal")
		})

		It("should round trip fields", func() {
			Expect(survey.GetFields()).To(HaveLen(1))
			Expect(survey.GetFields()[0].GetOptions()).To(Equal([]string{"pasta", "tacos"}))
		})

		It("should round trip response values", func() {
			Expect(survey.GetResponses()[0].GetValues().AsMap()).To(HaveKeyWithValue("choice", "pasta"))
		})
	})

	Describe("decodeField", func() {
		var field *apiv1.SurveyField

		BeforeEach(func() {
			field = decodeField(map[string]any{
				nameKey:     "rating",
				typeKey:     "number",
				labelKey:    "Rating",
				requiredKey: true,
				optionsKey:  []any{"one", "two", 3},
				minKey:      int64(1),
				maxKey:      float32(5),
			})
		})

		It("should decode scalar fields", func() {
			Expect(field.GetName()).To(Equal("rating"))
			Expect(field.GetType()).To(Equal("number"))
			Expect(field.GetLabel()).To(Equal("Rating"))
			Expect(field.GetRequired()).To(BeTrue())
		})

		It("should decode string options", func() {
			Expect(field.GetOptions()).To(Equal([]string{"one", "two"}))
		})

		It("should decode numeric bounds", func() {
			Expect(field.GetMin()).To(BeNumerically("==", 1))
			Expect(field.GetMax()).To(BeNumerically("==", 5))
		})
	})

	Describe("encodeFields", func() {
		var encoded []any

		BeforeEach(func() {
			label := "Rating"
			required := true
			minimum := 1.0
			maximum := 5.0
			encoded = encodeFields([]*apiv1.SurveyField{
				nil,
				{
					Name:     "rating",
					Type:     "number",
					Label:    &label,
					Required: &required,
					Options:  []string{"one", "two"},
					Min:      &minimum,
					Max:      &maximum,
				},
			})
		})

		It("should skip nil fields", func() {
			Expect(encoded).To(HaveLen(1))
		})

		It("should encode optional field attributes", func() {
			raw, ok := encoded[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(raw).To(HaveKeyWithValue(labelKey, "Rating"))
			Expect(raw).To(HaveKeyWithValue(requiredKey, true))
			Expect(raw).To(HaveKeyWithValue(optionsKey, []string{"one", "two"}))
			Expect(raw).To(HaveKeyWithValue(minKey, 1.0))
			Expect(raw).To(HaveKeyWithValue(maxKey, 5.0))
		})
	})

	Describe("readSlice", func() {
		var values []any

		BeforeEach(func() {
			values = readSlice(map[string]any{
				fieldsKey: []map[string]any{
					{nameKey: "choice"},
					{nameKey: "notes"},
				},
			}, fieldsKey)
		})

		It("should coerce slices of maps", func() {
			Expect(values).To(HaveLen(2))
		})
	})

	Describe("sortResponses", func() {
		var responses []*apiv1.SurveyResponse

		BeforeEach(func() {
			responses = []*apiv1.SurveyResponse{
				{User: "z@example.com"},
				{User: "a@example.com"},
				{
					User:        "time@example.com",
					SubmittedAt: timestamppb.New(time.Date(2026, 6, 9, 18, 0, 0, 0, time.UTC)),
				},
			}
			sortResponses(responses)
		})

		It("should sort nil timestamps by user", func() {
			Expect(responses[0].GetUser()).To(Equal("a@example.com"))
		})

		It("should place timestamped responses after earlier nil-timestamp user entries when compared by user", func() {
			Expect(responses[2].GetUser()).To(Equal("z@example.com"))
		})
	})
})

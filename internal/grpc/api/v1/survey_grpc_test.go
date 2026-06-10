//revive:disable:dot-imports
package v1_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/server/surveymutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

type surveySteadyClock struct{}

const (
	surveyTestYear   = 2026
	surveyTestMonth  = 6
	surveyTestDay    = 9
	surveyTestHour   = 20
	surveyTestMinute = 30
)

func (surveySteadyClock) Now() time.Time {
	return time.Date(surveyTestYear, time.Month(surveyTestMonth), surveyTestDay, surveyTestHour, surveyTestMinute, 0, 0, time.UTC)
}

func newSurveyTestServer(store *MockPageReaderMutator) *v1.Server {
	return mustNewServer(store, nil, nil).
		WithSurveyMutator(surveymutator.New(store, surveySteadyClock{}))
}

func surveyFrontmatter() wikipage.FrontMatter {
	return wikipage.FrontMatter{
		"surveys": map[string]any{
			"meal": map[string]any{
				"question": "Dinner?",
				"fields": []any{
					map[string]any{"name": "choice", "type": "text"},
					map[string]any{"name": "notes", "type": "text"},
				},
				"responses": []any{
					map[string]any{
						"user":         "alice@example.com",
						"submitted_at": "2026-06-09T19:00:00Z",
						"values":       map[string]any{"choice": "pasta"},
					},
				},
				"updated_at": "2026-06-09T19:00:00Z",
			},
		},
	}
}

var _ = Describe("SurveyService handlers", func() {
	var (
		ctx    context.Context
		store  *MockPageReaderMutator
		server *v1.Server
		err    error
	)

	BeforeEach(func() {
		ctx = tailscale.ContextWithIdentity(context.Background(), tailscale.NewIdentity("bob@example.com", "Bob", "bob-node"))
		store = &MockPageReaderMutator{Frontmatter: surveyFrontmatter()}
		server = newSurveyTestServer(store)
	})

	Describe("GetSurvey", func() {
		var resp *apiv1.GetSurveyResponse

		BeforeEach(func() {
			resp, err = server.GetSurvey(ctx, &apiv1.GetSurveyRequest{Page: "weekly_menu", Name: "meal"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the survey", func() {
			Expect(resp.GetSurvey().GetQuestion()).To(Equal("Dinner?"))
		})
	})

	Describe("UpsertSurvey", func() {
		var resp *apiv1.UpsertSurveyResponse

		BeforeEach(func() {
			resp, err = server.UpsertSurvey(ctx, &apiv1.UpsertSurveyRequest{Page: "weekly_menu", Name: "snacks", Question: "Snack?"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the upserted survey", func() {
			Expect(resp.GetSurvey().GetName()).To(Equal("snacks"))
		})
	})

	Describe("AddField", func() {
		var resp *apiv1.AddSurveyFieldResponse

		BeforeEach(func() {
			resp, err = server.AddField(ctx, &apiv1.AddSurveyFieldRequest{
				Page:       "weekly_menu",
				SurveyName: "meal",
				Field:      &apiv1.SurveyField{Name: "side", Type: "text"},
			})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the appended field", func() {
			Expect(resp.GetSurvey().GetFields()).To(HaveLen(3))
		})
	})

	Describe("UpdateField", func() {
		var resp *apiv1.UpdateSurveyFieldResponse

		BeforeEach(func() {
			resp, err = server.UpdateField(ctx, &apiv1.UpdateSurveyFieldRequest{
				Page:       "weekly_menu",
				SurveyName: "meal",
				FieldName:  "choice",
				Field:      &apiv1.SurveyField{Name: "entree", Type: "text"},
			})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the updated field", func() {
			Expect(resp.GetSurvey().GetFields()[0].GetName()).To(Equal("entree"))
		})
	})

	Describe("RemoveField", func() {
		var resp *apiv1.RemoveSurveyFieldResponse

		BeforeEach(func() {
			resp, err = server.RemoveField(ctx, &apiv1.RemoveSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", FieldName: "notes"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the field", func() {
			Expect(resp.GetSurvey().GetFields()).To(HaveLen(1))
		})
	})

	Describe("ReorderField", func() {
		var resp *apiv1.ReorderSurveyFieldResponse

		BeforeEach(func() {
			resp, err = server.ReorderField(ctx, &apiv1.ReorderSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", FieldName: "notes", NewIndex: 0})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should move the field", func() {
			Expect(resp.GetSurvey().GetFields()[0].GetName()).To(Equal("notes"))
		})
	})

	Describe("SubmitResponse", func() {
		var resp *apiv1.SubmitSurveyResponseResponse

		BeforeEach(func() {
			values, valuesErr := structpb.NewStruct(map[string]any{"choice": "tacos"})
			Expect(valuesErr).NotTo(HaveOccurred())
			resp, err = server.SubmitResponse(ctx, &apiv1.SubmitSurveyResponseRequest{Page: "weekly_menu", SurveyName: "meal", Values: values})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should add the caller response", func() {
			Expect(resp.GetSurvey().GetResponses()).To(HaveLen(2))
		})
	})

	Describe("ListResponses", func() {
		var resp *apiv1.ListSurveyResponsesResponse

		BeforeEach(func() {
			resp, err = server.ListResponses(ctx, &apiv1.ListSurveyResponsesRequest{Page: "weekly_menu", SurveyName: "meal"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the existing response", func() {
			Expect(resp.GetResponses()).To(HaveLen(1))
		})
	})

	Describe("DeleteResponse", func() {
		var resp *apiv1.DeleteSurveyResponseResponse

		BeforeEach(func() {
			resp, err = server.DeleteResponse(ctx, &apiv1.DeleteSurveyResponseRequest{Page: "weekly_menu", SurveyName: "meal", User: "alice@example.com"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the response", func() {
			Expect(resp.GetSurvey().GetResponses()).To(BeEmpty())
		})
	})
})

var _ = Describe("SurveyService handler validation", func() {
	var (
		ctx    context.Context
		store  *MockPageReaderMutator
		server *v1.Server
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = &MockPageReaderMutator{Frontmatter: surveyFrontmatter()}
		server = newSurveyTestServer(store)
	})

	Describe("when the survey mutator is not configured", func() {
		var errorsByRPC map[string]error

		BeforeEach(func() {
			server = mustNewServer(store, nil, nil)
			_, getSurveyErr := server.GetSurvey(ctx, &apiv1.GetSurveyRequest{Page: "weekly_menu", Name: "meal"})
			_, upsertSurveyErr := server.UpsertSurvey(ctx, &apiv1.UpsertSurveyRequest{Page: "weekly_menu", Name: "meal"})
			_, addFieldErr := server.AddField(ctx, &apiv1.AddSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal"})
			_, updateFieldErr := server.UpdateField(ctx, &apiv1.UpdateSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", FieldName: "choice"})
			_, removeFieldErr := server.RemoveField(ctx, &apiv1.RemoveSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", FieldName: "choice"})
			_, reorderFieldErr := server.ReorderField(ctx, &apiv1.ReorderSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", FieldName: "choice"})
			_, submitResponseErr := server.SubmitResponse(ctx, &apiv1.SubmitSurveyResponseRequest{Page: "weekly_menu", SurveyName: "meal"})
			_, listResponsesErr := server.ListResponses(ctx, &apiv1.ListSurveyResponsesRequest{Page: "weekly_menu", SurveyName: "meal"})
			_, deleteResponseErr := server.DeleteResponse(ctx, &apiv1.DeleteSurveyResponseRequest{Page: "weekly_menu", SurveyName: "meal", User: "alice@example.com"})
			errorsByRPC = map[string]error{
				"GetSurvey":      getSurveyErr,
				"UpsertSurvey":   upsertSurveyErr,
				"AddField":       addFieldErr,
				"UpdateField":    updateFieldErr,
				"RemoveField":    removeFieldErr,
				"ReorderField":   reorderFieldErr,
				"SubmitResponse": submitResponseErr,
				"ListResponses":  listResponsesErr,
				"DeleteResponse": deleteResponseErr,
			}
		})

		It("should return FailedPrecondition for every RPC", func() {
			for rpcName, rpcErr := range errorsByRPC {
				Expect(rpcErr).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "survey mutator not configured"), rpcName)
			}
		})
	})

	Describe("when page is missing", func() {
		BeforeEach(func() {
			_, err = server.GetSurvey(ctx, &apiv1.GetSurveyRequest{Name: "meal"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page is required"))
		})
	})

	Describe("when survey name is missing for GetSurvey", func() {
		BeforeEach(func() {
			_, err = server.GetSurvey(ctx, &apiv1.GetSurveyRequest{Page: "weekly_menu"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "name is required"))
		})
	})

	Describe("when survey name is missing for mutations", func() {
		BeforeEach(func() {
			_, err = server.UpsertSurvey(ctx, &apiv1.UpsertSurveyRequest{Page: "weekly_menu"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "survey_name is required"))
		})
	})

	Describe("when survey name is missing for ListResponses", func() {
		BeforeEach(func() {
			_, err = server.ListResponses(ctx, &apiv1.ListSurveyResponsesRequest{Page: "weekly_menu"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "survey_name is required"))
		})
	})

	Describe("when field_name is missing for update", func() {
		BeforeEach(func() {
			_, err = server.UpdateField(ctx, &apiv1.UpdateSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal", Field: &apiv1.SurveyField{Name: "x", Type: "text"}})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "field_name is required"))
		})
	})

	Describe("when field_name is missing for remove", func() {
		BeforeEach(func() {
			_, err = server.RemoveField(ctx, &apiv1.RemoveSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "field_name is required"))
		})
	})

	Describe("when field_name is missing for reorder", func() {
		BeforeEach(func() {
			_, err = server.ReorderField(ctx, &apiv1.ReorderSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "field_name is required"))
		})
	})

	Describe("when the survey is missing", func() {
		BeforeEach(func() {
			_, err = server.GetSurvey(ctx, &apiv1.GetSurveyRequest{Page: "weekly_menu", Name: "missing"})
		})

		It("should return NotFound", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "survey not found"))
		})
	})

	Describe("when adding a nil field", func() {
		BeforeEach(func() {
			_, err = server.AddField(ctx, &apiv1.AddSurveyFieldRequest{Page: "weekly_menu", SurveyName: "meal"})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "field.name is required"))
		})
	})

	Describe("when updating a missing field", func() {
		BeforeEach(func() {
			_, err = server.UpdateField(ctx, &apiv1.UpdateSurveyFieldRequest{
				Page:       "weekly_menu",
				SurveyName: "meal",
				FieldName:  "missing",
				Field:      &apiv1.SurveyField{Name: "missing", Type: "text"},
			})
		})

		It("should return NotFound", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "survey field not found"))
		})
	})

	Describe("when adding a duplicate field", func() {
		BeforeEach(func() {
			_, err = server.AddField(ctx, &apiv1.AddSurveyFieldRequest{
				Page:       "weekly_menu",
				SurveyName: "meal",
				Field:      &apiv1.SurveyField{Name: "choice", Type: "text"},
			})
		})

		It("should return AlreadyExists", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.AlreadyExists, "survey field already exists"))
		})
	})

	Describe("when deleting a missing response", func() {
		BeforeEach(func() {
			_, err = server.DeleteResponse(ctx, &apiv1.DeleteSurveyResponseRequest{Page: "weekly_menu", SurveyName: "meal", User: "missing@example.com"})
		})

		It("should return NotFound", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "survey response not found"))
		})
	})
})

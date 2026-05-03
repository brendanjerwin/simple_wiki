//revive:disable:dot-imports
package v1_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
)

// checklistSteadyClock implements checklistmutator.Clock for tests.
type checklistSteadyClock struct{}

const (
	checklistTestYear  = 2026
	checklistTestMonth = 4
	checklistTestDay   = 25
	checklistTestHour  = 12
)

func (*checklistSteadyClock) Now() time.Time {
	return time.Date(checklistTestYear, time.Month(checklistTestMonth), checklistTestDay, checklistTestHour, 0, 0, 0, time.UTC)
}

var _ = Describe("ChecklistService handlers — errChecklistMutatorNotConfigured", func() {
	var (
		ctx    context.Context
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		// mustNewServer creates a server with NO checklistMutator wired in.
		server = mustNewServer(&MockPageReaderMutator{}, nil, nil)
	})

	Describe("AddItem", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.AddItem(ctx, &apiv1.AddItemRequest{Page: "p", ListName: "l", Text: "T"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("UpdateItem", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.UpdateItem(ctx, &apiv1.UpdateItemRequest{Page: "p", ListName: "l", Uid: "u"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("ToggleItem", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ToggleItem(ctx, &apiv1.ToggleItemRequest{Page: "p", ListName: "l", Uid: "u"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("DeleteItem", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.DeleteItem(ctx, &apiv1.DeleteItemRequest{Page: "p", ListName: "l", Uid: "u"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("ReorderItem", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ReorderItem(ctx, &apiv1.ReorderItemRequest{Page: "p", ListName: "l", Uid: "u"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("ListItems", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ListItems(ctx, &apiv1.ListItemsRequest{Page: "p", ListName: "l"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("GetChecklists", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.GetChecklists(ctx, &apiv1.GetChecklistsRequest{Page: "p"})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})

	Describe("WatchList", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				err = server.WatchList(&apiv1.WatchListRequest{Page: "p", ListName: "l"}, &fakeWatchListStream{ctx: ctx})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "checklist mutator not configured"))
			})
		})
	})
})

// fakeWatchListStream is a no-op ChecklistService_WatchListServer used
// by the WatchList validation specs; the validation paths bail before
// any Send so the only method that matters is Context.
type fakeWatchListStream struct {
	apiv1.ChecklistService_WatchListServer
	ctx context.Context
}

func (s *fakeWatchListStream) Context() context.Context { return s.ctx }
func (*fakeWatchListStream) Send(*apiv1.WatchListResponse) error {
	return nil
}

var _ = Describe("ChecklistService handlers — page required validation", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{}
		cl := &checklistSteadyClock{}
		ug := ulid.NewSequenceGenerator("01HXAAAAAAAAAAAAAAAAAAAAAA")
		mutator := checklistmutator.New(mock, cl, ug)
		server = mustNewServer(mock, nil, nil).WithChecklistMutator(mutator)
	})

	Describe("AddItem", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.AddItem(ctx, &apiv1.AddItemRequest{Page: "", ListName: "l", Text: "T"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("UpdateItem", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.UpdateItem(ctx, &apiv1.UpdateItemRequest{Page: "", ListName: "l", Uid: "u"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("ToggleItem", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.ToggleItem(ctx, &apiv1.ToggleItemRequest{Page: "", ListName: "l", Uid: "u"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("DeleteItem", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.DeleteItem(ctx, &apiv1.DeleteItemRequest{Page: "", ListName: "l", Uid: "u"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("ReorderItem", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.ReorderItem(ctx, &apiv1.ReorderItemRequest{Page: "", ListName: "l", Uid: "u"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("ListItems", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.ListItems(ctx, &apiv1.ListItemsRequest{Page: "", ListName: "l"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("GetChecklists", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				_, err := server.GetChecklists(ctx, &apiv1.GetChecklistsRequest{Page: ""})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("WatchList", func() {
		When("page is empty", func() {
			It("should return InvalidArgument", func() {
				err := server.WatchList(&apiv1.WatchListRequest{Page: "", ListName: "l"}, &fakeWatchListStream{ctx: ctx})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})

		When("list_name is empty", func() {
			It("should return InvalidArgument", func() {
				err := server.WatchList(&apiv1.WatchListRequest{Page: "p", ListName: ""}, &fakeWatchListStream{ctx: ctx})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "list_name"))
			})
		})
	})
})

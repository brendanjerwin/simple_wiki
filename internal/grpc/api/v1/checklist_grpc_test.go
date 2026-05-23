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
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
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

	Describe("DeduplicateItems", func() {
		When("checklistMutator is not configured", func() {
			var err error

			BeforeEach(func() {
				_, err = server.DeduplicateItems(ctx, &apiv1.DeduplicateItemsRequest{Page: "p", ListName: "l"})
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

	Describe("DeduplicateItems", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.DeduplicateItems(ctx, &apiv1.DeduplicateItemsRequest{Page: "", ListName: "l"})
			})

			It("should return InvalidArgument", func() {
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

var _ = Describe("ChecklistService handlers — public checklist response shape", func() {
	var (
		ctx      context.Context
		server   *v1.Server
		mutator  *checklistmutator.Mutator
		eggsUID  string
		listResp *apiv1.ListItemsResponse
		getResp  *apiv1.GetChecklistsResponse
		listErr  error
		getErr   error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock := &MockPageReaderMutator{Frontmatter: wikipage.FrontMatter{}}
		cl := &checklistSteadyClock{}
		ug := ulid.NewSequenceGenerator("01HXBBBBBBBBBBBBBBBBBBBBBB")
		mutator = checklistmutator.New(mock, cl, ug)
		server = mustNewServer(mock, nil, nil).WithChecklistMutator(mutator)

		firstItem, _, err := mutator.AddItem(ctx, "weekly_menu", "ingredients-on-hand", checklistmutator.AddItemArgs{Text: "milk"}, tailscale.Anonymous)
		Expect(err).NotTo(HaveOccurred())

		eggsItem, _, err := mutator.AddItem(ctx, "weekly_menu", "ingredients-on-hand", checklistmutator.AddItemArgs{Text: "eggs"}, tailscale.Anonymous)
		Expect(err).NotTo(HaveOccurred())
		eggsUID = eggsItem.GetUid()

		_, err = mutator.DeleteItem(ctx, "weekly_menu", "ingredients-on-hand", firstItem.GetUid(), nil, tailscale.Anonymous)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		listResp, listErr = server.ListItems(ctx, &apiv1.ListItemsRequest{Page: "weekly_menu", ListName: "ingredients-on-hand"})
		getResp, getErr = server.GetChecklists(ctx, &apiv1.GetChecklistsRequest{Page: "weekly_menu"})
	})

	Describe("ListItems", func() {
		It("should return no error", func() {
			Expect(listErr).NotTo(HaveOccurred())
		})

		It("should preserve live items", func() {
			Expect(listResp.GetChecklist().GetItems()).To(HaveLen(1))
			Expect(listResp.GetChecklist().GetItems()[0].GetText()).To(Equal("eggs"))
		})

		It("should preserve the sync token", func() {
			Expect(listResp.GetChecklist().GetSyncToken()).To(BeNumerically(">", 0))
		})

		It("should omit the internal event log", func() {
			Expect(listResp.GetChecklist().GetEvents()).To(BeEmpty())
		})

		It("should omit tombstones", func() {
			Expect(listResp.GetChecklist().GetTombstones()).To(BeEmpty())
		})

		It("should omit max_seq", func() {
			Expect(listResp.GetChecklist().GetMaxSeq()).To(BeZero())
		})

		When("a page size is requested", func() {
			var (
				firstPageResp  *apiv1.ListItemsResponse
				secondPageResp *apiv1.ListItemsResponse
				firstPageErr   error
				secondPageErr  error
				addErr         error
			)

			BeforeEach(func() {
				_, _, addErr = mutator.AddItem(ctx, "weekly_menu", "ingredients-on-hand", checklistmutator.AddItemArgs{Text: "flour"}, tailscale.Anonymous)

				firstPageResp, firstPageErr = server.ListItems(ctx, &apiv1.ListItemsRequest{Page: "weekly_menu", ListName: "ingredients-on-hand", PageSize: 1})
				secondPageResp, secondPageErr = server.ListItems(ctx, &apiv1.ListItemsRequest{
					Page:      "weekly_menu",
					ListName:  "ingredients-on-hand",
					PageSize:  1,
					PageToken: firstPageResp.GetNextPageToken(),
				})
			})

			It("should add the second item", func() {
				Expect(addErr).NotTo(HaveOccurred())
			})

			It("should return the first page", func() {
				Expect(firstPageErr).NotTo(HaveOccurred())
				Expect(firstPageResp.GetChecklist().GetItems()).To(HaveLen(1))
			})

			It("should return a continuation token", func() {
				Expect(firstPageResp.GetNextPageToken()).To(Equal("1"))
			})

			It("should return the next page", func() {
				Expect(secondPageErr).NotTo(HaveOccurred())
				Expect(secondPageResp.GetChecklist().GetItems()).To(HaveLen(1))
				Expect(secondPageResp.GetChecklist().GetItems()[0].GetText()).To(Equal("flour"))
			})
		})

		When("the page token is invalid", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ListItems(ctx, &apiv1.ListItemsRequest{Page: "weekly_menu", ListName: "ingredients-on-hand", PageToken: "not-a-number"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page_token"))
			})
		})
	})

	Describe("GetChecklists", func() {
		It("should return no error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("should preserve the checklist", func() {
			Expect(getResp.GetChecklists()).To(HaveLen(1))
			Expect(getResp.GetChecklists()[0].GetName()).To(Equal("ingredients-on-hand"))
		})

		It("should preserve the sync token", func() {
			Expect(getResp.GetChecklists()[0].GetSyncToken()).To(BeNumerically(">", 0))
		})

		It("should omit the internal event log", func() {
			Expect(getResp.GetChecklists()[0].GetEvents()).To(BeEmpty())
		})

		It("should omit tombstones", func() {
			Expect(getResp.GetChecklists()[0].GetTombstones()).To(BeEmpty())
		})

		It("should omit max_seq", func() {
			Expect(getResp.GetChecklists()[0].GetMaxSeq()).To(BeZero())
		})
	})

	Describe("mutation responses", func() {
		var (
			addResp     *apiv1.AddItemResponse
			updateResp  *apiv1.UpdateItemResponse
			toggleResp  *apiv1.ToggleItemResponse
			reorderResp *apiv1.ReorderItemResponse
			deleteResp  *apiv1.DeleteItemResponse
			err         error
			breadUID    string
		)

		BeforeEach(func() {
			addResp, err = server.AddItem(ctx, &apiv1.AddItemRequest{
				Page:     "weekly_menu",
				ListName: "ingredients-on-hand",
				Text:     "bread",
			})
			Expect(err).NotTo(HaveOccurred())
			breadUID = addResp.GetItem().GetUid()

			updatedText := "fresh eggs"
			updateResp, err = server.UpdateItem(ctx, &apiv1.UpdateItemRequest{
				Page:     "weekly_menu",
				ListName: "ingredients-on-hand",
				Uid:      eggsUID,
				Text:     &updatedText,
			})
			Expect(err).NotTo(HaveOccurred())

			toggleResp, err = server.ToggleItem(ctx, &apiv1.ToggleItemRequest{
				Page:     "weekly_menu",
				ListName: "ingredients-on-hand",
				Uid:      eggsUID,
			})
			Expect(err).NotTo(HaveOccurred())

			reorderResp, err = server.ReorderItem(ctx, &apiv1.ReorderItemRequest{
				Page:         "weekly_menu",
				ListName:     "ingredients-on-hand",
				Uid:          breadUID,
				NewSortOrder: 1,
			})
			Expect(err).NotTo(HaveOccurred())

			deleteResp, err = server.DeleteItem(ctx, &apiv1.DeleteItemRequest{
				Page:     "weekly_menu",
				ListName: "ingredients-on-hand",
				Uid:      breadUID,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should strip sync internals from AddItem", func() {
			expectPublicChecklist(addResp.GetChecklist())
		})

		It("should strip sync internals from UpdateItem", func() {
			expectPublicChecklist(updateResp.GetChecklist())
		})

		It("should strip sync internals from ToggleItem", func() {
			expectPublicChecklist(toggleResp.GetChecklist())
		})

		It("should strip sync internals from ReorderItem", func() {
			expectPublicChecklist(reorderResp.GetChecklist())
		})

		It("should strip sync internals from DeleteItem", func() {
			expectPublicChecklist(deleteResp.GetChecklist())
		})
	})
})

func expectPublicChecklist(checklist *apiv1.Checklist) {
	Expect(checklist.GetSyncToken()).To(BeNumerically(">", 0))
	Expect(checklist.GetEvents()).To(BeEmpty())
	Expect(checklist.GetTombstones()).To(BeEmpty())
	Expect(checklist.GetMaxSeq()).To(BeZero())
}

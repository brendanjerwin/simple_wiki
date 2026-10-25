//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	grpcv1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

var errTest = errors.New("test error")

// fakeAgentInventory is a stub v1.AgentInventory for handler tests.
type fakeAgentInventory struct {
	agents       []*apiv1.AgentInfo
	defaultAgent string
}

func newFakeAgentInventory() *fakeAgentInventory {
	return &fakeAgentInventory{
		agents: []*apiv1.AgentInfo{
			{
				Id:          server.LocalACPAgentID,
				DisplayName: "Local Agent",
				Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
				Description: "Local ACP pool",
				Available:   true,
			},
		},
		defaultAgent: server.LocalACPAgentID,
	}
}

func (f *fakeAgentInventory) List() []*apiv1.AgentInfo { return f.agents }
func (f *fakeAgentInventory) DefaultAgentID() string   { return f.defaultAgent }
func (*fakeAgentInventory) RegisterPoolAgent(_ *apiv1.RegisterPoolAgentRequest) error {
	return nil
}

func (f *fakeAgentInventory) FindByID(id string) *apiv1.AgentInfo {
	for _, a := range f.agents {
		if a.Id == id {
			return a
		}
	}
	return nil
}

// fakeAgentPageBindingStore is a stub v1.AgentPageBindingStore for handler tests.
type fakeAgentPageBindingStore struct {
	bindings map[string]*server.PageAgentBinding
	readErr  error
	writeErr error
}

func newFakeAgentPageBindingStore() *fakeAgentPageBindingStore {
	return &fakeAgentPageBindingStore{bindings: map[string]*server.PageAgentBinding{}}
}

func (f *fakeAgentPageBindingStore) GetBinding(page string) (*server.PageAgentBinding, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.bindings[page], nil
}

func (f *fakeAgentPageBindingStore) SetBinding(page string, input *server.AgentBindingInput) (string, error) {
	if f.writeErr != nil {
		return "", f.writeErr
	}
	updatedAt := "2026-10-24T18:00:00Z"
	f.bindings[page] = &server.PageAgentBinding{
		AgentID:     input.AgentID,
		DisplayName: input.DisplayName,
		UpdatedBy:   input.UpdatedBy,
		UpdatedAt:   updatedAt,
	}
	return updatedAt, nil
}

// newAgentInventoryServer wires a Server with minimal dependencies for inventory tests.
func newAgentInventoryServer() (*grpcv1.Server, *fakeAgentInventory, *fakeAgentPageBindingStore) {
	inv := newFakeAgentInventory()
	binding := newFakeAgentPageBindingStore()
	srv := mustNewServer(&MockPageReaderMutator{}, nil, nil).
		WithAgentInventory(inv).
		WithAgentPageBindingStore(binding)
	return srv, inv, binding
}

var _ = Describe("AgentInventoryService handlers", func() {
	var (
		ctx     context.Context
		srv     *grpcv1.Server
		binding *fakeAgentPageBindingStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		srv, _, binding = newAgentInventoryServer()
	})

	Describe("ListAgents", func() {
		Describe("when the inventory is configured", func() {
			var resp *apiv1.ListAgentsResponse
			var err error

			BeforeEach(func() {
				resp, err = srv.ListAgents(ctx, &apiv1.ListAgentsRequest{})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the list of agents", func() {
				Expect(resp.GetAgents()).To(HaveLen(1))
				Expect(resp.GetAgents()[0].GetId()).To(Equal(server.LocalACPAgentID))
			})
		})

		Describe("when the inventory is not configured", func() {
			var err error

			BeforeEach(func() {
				bare := mustNewServer(&MockPageReaderMutator{}, nil, nil)
				_, err = bare.ListAgents(ctx, &apiv1.ListAgentsRequest{})
			})

			It("should return FailedPrecondition", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.FailedPrecondition))
			})
		})
	})

	Describe("GetPageAgent", func() {
		Describe("when no binding has been set", func() {
			var resp *apiv1.GetPageAgentResponse
			var err error

			BeforeEach(func() {
				resp, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the default agent", func() {
				Expect(resp.GetAgent().GetId()).To(Equal(server.LocalACPAgentID))
			})

			It("should report not explicitly set", func() {
				Expect(resp.GetExplicitlySet()).To(BeFalse())
			})
		})

		Describe("when a binding has been set", func() {
			var resp *apiv1.GetPageAgentResponse
			var err error

			BeforeEach(func() {
				binding.bindings["test-page"] = &server.PageAgentBinding{
					AgentID:     server.LocalACPAgentID,
					DisplayName: "Local Agent",
				}
				resp, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the stored agent", func() {
				Expect(resp.GetAgent().GetId()).To(Equal(server.LocalACPAgentID))
			})

			It("should report explicitly set", func() {
				Expect(resp.GetExplicitlySet()).To(BeTrue())
			})
		})

		Describe("when the stored agent ID no longer exists in inventory", func() {
			var resp *apiv1.GetPageAgentResponse
			var err error

			BeforeEach(func() {
				binding.bindings["test-page"] = &server.PageAgentBinding{
					AgentID:     "removed-agent",
					DisplayName: "Removed Agent",
				}
				resp, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should surface the orphaned binding as unavailable, not fall back", func() {
				Expect(resp.GetAgent().GetId()).To(Equal("removed-agent"))
				Expect(resp.GetAgent().GetAvailable()).To(BeFalse())
			})

			It("should report explicitly set", func() {
				Expect(resp.GetExplicitlySet()).To(BeTrue())
			})
		})

		Describe("when page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: ""})
			})

			It("should return InvalidArgument", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		Describe("when the inventory is not configured", func() {
			var err error

			BeforeEach(func() {
				bare := mustNewServer(&MockPageReaderMutator{}, nil, nil)
				_, err = bare.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should return FailedPrecondition", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.FailedPrecondition))
			})
		})

		Describe("when the binding store returns an error", func() {
			var err error

			BeforeEach(func() {
				binding.readErr = errTest
				_, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should return Internal", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
			})
		})

		Describe("when the binding store reports the page does not exist", func() {
			var err error

			BeforeEach(func() {
				binding.readErr = fmt.Errorf("read frontmatter for %q: %w", "test-page", os.ErrNotExist)
				_, err = srv.GetPageAgent(ctx, &apiv1.GetPageAgentRequest{Page: "test-page"})
			})

			It("should return NotFound", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
			})
		})
	})

	Describe("SetPageAgent", func() {
		Describe("when setting a valid agent", func() {
			var resp *apiv1.SetPageAgentResponse
			var err error

			BeforeEach(func() {
				resp, err = srv.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "test-page",
					AgentId: server.LocalACPAgentID,
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the bound agent", func() {
				Expect(resp.GetAgent().GetId()).To(Equal(server.LocalACPAgentID))
			})

			It("should persist the binding", func() {
				Expect(binding.bindings["test-page"].AgentID).To(Equal(server.LocalACPAgentID))
			})
		})

		Describe("when page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "",
					AgentId: server.LocalACPAgentID,
				})
			})

			It("should return InvalidArgument", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		Describe("when agent_id is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "test-page",
					AgentId: "",
				})
			})

			It("should return InvalidArgument", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		Describe("when agent_id is not in inventory", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "test-page",
					AgentId: "nonexistent-agent",
				})
			})

			It("should return NotFound", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
			})
		})

		Describe("when the inventory is not configured", func() {
			var err error

			BeforeEach(func() {
				bare := mustNewServer(&MockPageReaderMutator{}, nil, nil)
				_, err = bare.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "test-page",
					AgentId: server.LocalACPAgentID,
				})
			})

			It("should return FailedPrecondition", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.FailedPrecondition))
			})
		})

		Describe("when the binding store returns an error", func() {
			var err error

			BeforeEach(func() {
				binding.writeErr = errTest
				_, err = srv.SetPageAgent(ctx, &apiv1.SetPageAgentRequest{
					Page:    "test-page",
					AgentId: server.LocalACPAgentID,
				})
			})

			It("should return Internal", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
			})
		})
	})
})

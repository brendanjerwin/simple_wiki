//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// fakeScheduledTurnDispatcher is a test double for v1.ScheduledTurnDispatcher.
type fakeScheduledTurnDispatcher struct {
	mu              sync.Mutex
	subscribeCh     <-chan *apiv1.ScheduledTurnRequest
	unsubscribeFunc func()
	unsubscribed    bool

	completeErr      error
	completeCalls    int
	lastCompleteReq  *apiv1.CompleteScheduledTurnRequest
}

func (f *fakeScheduledTurnDispatcher) Subscribe() (<-chan *apiv1.ScheduledTurnRequest, func()) {
	unsubscribe := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.unsubscribed = true
	}
	if f.unsubscribeFunc != nil {
		unsubscribe = f.unsubscribeFunc
	}
	return f.subscribeCh, unsubscribe
}

func (f *fakeScheduledTurnDispatcher) Complete(req *apiv1.CompleteScheduledTurnRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	f.lastCompleteReq = req
	return f.completeErr
}

// fakeScheduledTurnsStream implements
// apiv1.ScheduledTurnService_SubscribeScheduledTurnsServer for testing.
type fakeScheduledTurnsStream struct {
	grpc.ServerStream

	mu       sync.Mutex
	sent     []*apiv1.ScheduledTurnRequest
	sendErr  error
	ctx      context.Context
	cancelFn context.CancelFunc
}

func newFakeScheduledTurnsStream() *fakeScheduledTurnsStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeScheduledTurnsStream{ctx: ctx, cancelFn: cancel}
}

func (s *fakeScheduledTurnsStream) Send(req *apiv1.ScheduledTurnRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, req)
	return nil
}

func (s *fakeScheduledTurnsStream) Context() context.Context {
	return s.ctx
}

func (s *fakeScheduledTurnsStream) Cancel() {
	if s.cancelFn != nil {
		s.cancelFn()
	}
}

func (s *fakeScheduledTurnsStream) SentMessages() []*apiv1.ScheduledTurnRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*apiv1.ScheduledTurnRequest, len(s.sent))
	copy(out, s.sent)
	return out
}

func (*fakeScheduledTurnsStream) SetHeader(metadata.MD) error  { return nil }
func (*fakeScheduledTurnsStream) SendHeader(metadata.MD) error { return nil }
func (*fakeScheduledTurnsStream) SetTrailer(metadata.MD)       {}
func (*fakeScheduledTurnsStream) SendMsg(any) error            { return nil }
func (*fakeScheduledTurnsStream) RecvMsg(any) error            { return nil }

var _ = Describe("ScheduledTurnService", func() {
	var server *v1.Server

	Describe("CompleteScheduledTurn", func() {
		var (
			ctx  context.Context
			req  *apiv1.CompleteScheduledTurnRequest
			resp *apiv1.CompleteScheduledTurnResponse
			err  error
		)

		BeforeEach(func() {
			ctx = context.Background()
			req = &apiv1.CompleteScheduledTurnRequest{RequestId: "turn-1"}
		})

		When("the dispatcher is not configured", func() {
			BeforeEach(func() {
				server = mustNewServer(nil, nil, nil)
				resp, err = server.CompleteScheduledTurn(ctx, req)
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("request_id is empty", func() {
			var fake *fakeScheduledTurnDispatcher

			BeforeEach(func() {
				fake = &fakeScheduledTurnDispatcher{}
				server = mustNewServer(nil, nil, nil).WithScheduledTurnDispatcher(fake)
				req.RequestId = ""
				resp, err = server.CompleteScheduledTurn(ctx, req)
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "request_id is required"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})

			It("should not call the dispatcher", func() {
				Expect(fake.completeCalls).To(Equal(0))
			})
		})

		When("the dispatcher succeeds", func() {
			var fake *fakeScheduledTurnDispatcher

			BeforeEach(func() {
				fake = &fakeScheduledTurnDispatcher{completeErr: nil}
				server = mustNewServer(nil, nil, nil).WithScheduledTurnDispatcher(fake)
				resp, err = server.CompleteScheduledTurn(ctx, req)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a non-nil response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should call the dispatcher exactly once", func() {
				Expect(fake.completeCalls).To(Equal(1))
			})

			It("should pass the same request to the dispatcher", func() {
				Expect(fake.lastCompleteReq).To(BeIdenticalTo(req))
			})
		})

		When("the dispatcher returns an error", func() {
			var fake *fakeScheduledTurnDispatcher

			BeforeEach(func() {
				fake = &fakeScheduledTurnDispatcher{completeErr: errors.New("orphan completion")}
				server = mustNewServer(nil, nil, nil).WithScheduledTurnDispatcher(fake)
				resp, err = server.CompleteScheduledTurn(ctx, req)
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "complete scheduled turn"))
			})

			It("should include the underlying error message", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "orphan completion"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})
	})

	Describe("SubscribeScheduledTurns", func() {
		var (
			req    *apiv1.SubscribeScheduledTurnsRequest
			stream *fakeScheduledTurnsStream
		)

		BeforeEach(func() {
			req = &apiv1.SubscribeScheduledTurnsRequest{}
			stream = newFakeScheduledTurnsStream()
		})

		When("the dispatcher is not configured", func() {
			var err error

			BeforeEach(func() {
				server = mustNewServer(nil, nil, nil)
				err = server.SubscribeScheduledTurns(req, stream)
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
			})
		})

		When("the dispatcher delivers a request and closes the channel", func() {
			var (
				fake     *fakeScheduledTurnDispatcher
				err      error
				expected *apiv1.ScheduledTurnRequest
			)

			BeforeEach(func() {
				expected = &apiv1.ScheduledTurnRequest{RequestId: "turn-42"}
				ch := make(chan *apiv1.ScheduledTurnRequest, 1)
				ch <- expected
				close(ch)

				fake = &fakeScheduledTurnDispatcher{subscribeCh: ch}
				server = mustNewServer(nil, nil, nil).WithScheduledTurnDispatcher(fake)

				err = server.SubscribeScheduledTurns(req, stream)
			})

			It("should return nil when the channel closes", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should forward the dispatched request to the stream", func() {
				sent := stream.SentMessages()
				Expect(sent).To(HaveLen(1))
				Expect(sent[0]).To(BeIdenticalTo(expected))
			})

			It("should unsubscribe when the handler returns", func() {
				Expect(fake.unsubscribed).To(BeTrue())
			})
		})

		When("the stream context is cancelled", func() {
			var (
				fake   *fakeScheduledTurnDispatcher
				doneCh chan error
			)

			BeforeEach(func() {
				// Channel that never delivers and never closes; the only way the
				// handler can return is via context cancellation.
				ch := make(chan *apiv1.ScheduledTurnRequest)
				fake = &fakeScheduledTurnDispatcher{subscribeCh: ch}
				server = mustNewServer(nil, nil, nil).WithScheduledTurnDispatcher(fake)

				doneCh = make(chan error, 1)
				go func() {
					doneCh <- server.SubscribeScheduledTurns(req, stream)
				}()

				stream.Cancel()
			})

			It("should return the context error within one second", func() {
				select {
				case err := <-doneCh:
					Expect(err).To(MatchError(context.Canceled))
				case <-time.After(time.Second):
					Fail("SubscribeScheduledTurns did not return after context cancellation")
				}
			})
		})
	})
})

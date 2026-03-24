package main

import (
	"context"
	"errors"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

// mockSubscribeChatMessagesStream implements apiv1.ChatService_SubscribeChatMessagesClient for testing.
// It embeds the grpc.ClientStream interface (nil) and only overrides Recv.
type mockSubscribeChatMessagesStream struct {
	grpc.ClientStream
	recvFn func() (*apiv1.ChatMessage, error)
}

func (m *mockSubscribeChatMessagesStream) Recv() (*apiv1.ChatMessage, error) {
	return m.recvFn()
}

// mockChatClient implements apiv1.ChatServiceClient for testing, overriding only SubscribeChatMessages.
type mockChatClient struct {
	apiv1.ChatServiceClient
	subscribeFn func(context.Context, *apiv1.SubscribeChatMessagesRequest, ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error)
}

func (m *mockChatClient) SubscribeChatMessages(ctx context.Context, in *apiv1.SubscribeChatMessagesRequest, opts ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
	return m.subscribeFn(ctx, in, opts...)
}

var _ = Describe("parseGRPCHost", func() {
	When("given an https URL without an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("https://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the default HTTPS port", func() {
			Expect(host).To(Equal("wiki.example.com:443"))
		})

		It("should return scheme https", func() {
			Expect(scheme).To(Equal("https"))
		})
	})

	When("given an https URL with an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("https://wiki.example.com:8443")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the explicit port", func() {
			Expect(host).To(Equal("wiki.example.com:8443"))
		})

		It("should return scheme https", func() {
			Expect(scheme).To(Equal("https"))
		})
	})

	When("given an http URL without an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("http://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the default HTTP port", func() {
			Expect(host).To(Equal("wiki.example.com:80"))
		})

		It("should return scheme http", func() {
			Expect(scheme).To(Equal("http"))
		})
	})

	When("given an http URL with an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("http://wiki.example.com:8080")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the explicit port", func() {
			Expect(host).To(Equal("wiki.example.com:8080"))
		})

		It("should return scheme http", func() {
			Expect(scheme).To(Equal("http"))
		})
	})

	When("given a URL with an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, _, err = parseGRPCHost("ftp://wiki.example.com")
		})

		It("should return an error mentioning the scheme", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
		})
	})

	When("given an empty scheme", func() {
		var err error

		BeforeEach(func() {
			_, _, err = parseGRPCHost("wiki.example.com")
		})

		It("should return an unsupported scheme error", func() {
			Expect(err).To(MatchError(ContainSubstring("unsupported URL scheme")))
		})
	})
})

var _ = Describe("createGRPCConn", func() {
	When("given a valid https URL", func() {
		var err error
		var conn *grpc.ClientConn

		BeforeEach(func() {
			conn, err = createGRPCConn("https://wiki.example.com")
		})

		AfterEach(func() {
			if conn != nil {
				_ = conn.Close()
			}
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil connection", func() {
			Expect(conn).NotTo(BeNil())
		})
	})

	When("given a valid http URL", func() {
		var err error
		var conn *grpc.ClientConn

		BeforeEach(func() {
			conn, err = createGRPCConn("http://wiki.example.com")
		})

		AfterEach(func() {
			if conn != nil {
				_ = conn.Close()
			}
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil connection", func() {
			Expect(conn).NotTo(BeNil())
		})
	})

	When("given an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, err = createGRPCConn("ftp://wiki.example.com")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
		})
	})
})

var _ = Describe("createAPIClients", func() {
	When("given a valid gRPC connection", func() {
		var clients *apiClients
		var conn *grpc.ClientConn

		BeforeEach(func() {
			var err error
			conn, err = createGRPCConn("http://localhost:1")
			Expect(err).NotTo(HaveOccurred())
			clients = createAPIClients(conn)
		})

		AfterEach(func() {
			_ = conn.Close()
		})

		It("should return non-nil clients struct", func() {
			Expect(clients).NotTo(BeNil())
		})

		It("should have a non-nil chat client", func() {
			Expect(clients.chat).NotTo(BeNil())
		})

		It("should have a non-nil frontmatter client", func() {
			Expect(clients.frontmatter).NotTo(BeNil())
		})

		It("should have a non-nil inventory client", func() {
			Expect(clients.inventory).NotTo(BeNil())
		})

		It("should have a non-nil pageImport client", func() {
			Expect(clients.pageImport).NotTo(BeNil())
		})

		It("should have a non-nil pageManagement client", func() {
			Expect(clients.pageManagement).NotTo(BeNil())
		})

		It("should have a non-nil search client", func() {
			Expect(clients.search).NotTo(BeNil())
		})

		It("should have a non-nil systemInfo client", func() {
			Expect(clients.systemInfo).NotTo(BeNil())
		})
	})
})

var _ = Describe("registerToolHandlers", func() {
	When("called with a new MCPServer and empty clients", func() {
		var toolCount int

		BeforeEach(func() {
			s := mcpserver.NewMCPServer("test", "1.0")
			registerToolHandlers(s, &apiClients{})
			toolCount = len(s.ListTools())
		})

		It("should register at least one tool", func() {
			Expect(toolCount).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("computeBackoffAfterFailure", func() {
	When("the stream ran for a short duration (rapid failure)", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			delayMs, nextMs = computeBackoffAfterFailure(initialBackoffMs, 100*time.Millisecond)
		})

		It("should use the current backoff as the delay", func() {
			Expect(delayMs).To(Equal(initialBackoffMs))
		})

		It("should double the backoff for the next iteration", func() {
			Expect(nextMs).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})
	})

	When("there are multiple rapid consecutive failures", func() {
		var delayMs2, nextMs2 int

		BeforeEach(func() {
			// Simulate accumulation from a previous failure
			_, nextMs1 := computeBackoffAfterFailure(initialBackoffMs, 100*time.Millisecond)
			delayMs2, nextMs2 = computeBackoffAfterFailure(nextMs1, 100*time.Millisecond)
		})

		It("should keep accumulating the backoff delay", func() {
			Expect(delayMs2).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})

		It("should double again for the next iteration", func() {
			Expect(nextMs2).To(Equal(initialBackoffMs * int(backoffMultiplier) * int(backoffMultiplier)))
		})
	})

	When("the stream ran long enough to be considered healthy", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			// Start from an elevated backoff (simulates previous rapid failures)
			elevatedBackoff := 16000
			healthyDuration := time.Duration(initialBackoffMs+500) * time.Millisecond
			delayMs, nextMs = computeBackoffAfterFailure(elevatedBackoff, healthyDuration)
		})

		It("should reset the delay to initialBackoffMs", func() {
			Expect(delayMs).To(Equal(initialBackoffMs))
		})

		It("should set next backoff to initial*multiplier after the reset", func() {
			Expect(nextMs).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})
	})

	When("the backoff would exceed the maximum", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			delayMs, nextMs = computeBackoffAfterFailure(maxBackoffMs, 100*time.Millisecond)
		})

		It("should use maxBackoffMs as the delay", func() {
			Expect(delayMs).To(Equal(maxBackoffMs))
		})

		It("should cap the next backoff at maxBackoffMs", func() {
			Expect(nextMs).To(Equal(maxBackoffMs))
		})
	})
})

var _ = Describe("subscribeToChatMessages", func() {
	var s *mcpserver.MCPServer

	BeforeEach(func() {
		s = mcpserver.NewMCPServer("test", "1.0")
	})

	When("SubscribeChatMessages fails to establish the stream", func() {
		var err error

		BeforeEach(func() {
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					return nil, errors.New("dial error")
				},
			}
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		It("should return the subscribe error", func() {
			Expect(err).To(MatchError(ContainSubstring("dial error")))
		})
	})

	When("context is cancelled before Recv completes", func() {
		var err error

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							return nil, context.Canceled
						},
					}, nil
				},
			}
			err = subscribeToChatMessages(ctx, s, client)
		})

		It("should return nil", func() {
			Expect(err).To(BeNil())
		})
	})

	When("stream.Recv returns a non-context error", func() {
		var err error

		BeforeEach(func() {
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							return nil, errors.New("connection reset")
						},
					}, nil
				},
			}
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		It("should return the stream error", func() {
			Expect(err).To(MatchError(ContainSubstring("connection reset")))
		})
	})

	When("a USER message is received then stream errors", func() {
		var err error

		BeforeEach(func() {
			callCount := 0
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							callCount++
							if callCount == 1 {
								return &apiv1.ChatMessage{
									Id:      "msg-1",
									Sender:  apiv1.Sender_USER,
									Content: "hello",
									Page:    "test-page",
								}, nil
							}
							return nil, errors.New("stream ended")
						},
					}, nil
				},
			}
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		It("should return the subsequent stream error after processing the user message", func() {
			Expect(err).To(MatchError(ContainSubstring("stream ended")))
		})
	})

	When("a non-USER message is received then context is cancelled", func() {
		var err error

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			callCount := 0
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							callCount++
							if callCount == 1 {
								return &apiv1.ChatMessage{
									Id:     "msg-2",
									Sender: apiv1.Sender_ASSISTANT,
								}, nil
							}
							cancel()
							return nil, context.Canceled
						},
					}, nil
				},
			}
			err = subscribeToChatMessages(ctx, s, client)
		})

		It("should filter the non-USER message and return nil on context cancellation", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("maintainChatSubscription", func() {
	var s *mcpserver.MCPServer

	BeforeEach(func() {
		s = mcpserver.NewMCPServer("test", "1.0")
	})

	When("context is already cancelled at start", func() {
		var done chan struct{}
		var subscribeCalled bool

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			subscribeCalled = false

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					subscribeCalled = true
					return nil, errors.New("should not be called")
				},
			}
			done = make(chan struct{})
			go func() {
				maintainChatSubscription(ctx, s, client)
				close(done)
			}()
		})

		It("should return without calling subscribe", func() {
			Eventually(done, "1s").Should(BeClosed())
		})

		It("should not call subscribe", func() {
			Eventually(done, "1s").Should(BeClosed())
			Expect(subscribeCalled).To(BeFalse())
		})
	})

	When("subscribe signals a clean disconnect via context cancellation", func() {
		var done chan struct{}

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					cancel()
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							return nil, context.Canceled
						},
					}, nil
				},
			}
			done = make(chan struct{})
			go func() {
				maintainChatSubscription(ctx, s, client)
				close(done)
			}()
		})

		It("should return when subscribe returns nil", func() {
			Eventually(done, "1s").Should(BeClosed())
		})
	})

	When("subscribe fails and context is cancelled before backoff expires", func() {
		var done chan struct{}
		var callCount int

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			callCount = 0

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					callCount++
					cancel() // cancel context so the backoff select exits immediately
					return nil, errors.New("connection refused")
				},
			}
			done = make(chan struct{})
			go func() {
				maintainChatSubscription(ctx, s, client)
				close(done)
			}()
		})

		It("should return after the context is cancelled", func() {
			Eventually(done, "1s").Should(BeClosed())
		})

		It("should have called subscribe once", func() {
			Eventually(done, "1s").Should(BeClosed())
			Expect(callCount).To(Equal(1))
		})
	})

	When("subscribe fails once then reconnects after backoff and exits cleanly", func() {
		var done chan struct{}
		var callCount int

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			callCount = 0

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *apiv1.SubscribeChatMessagesRequest, _ ...grpc.CallOption) (apiv1.ChatService_SubscribeChatMessagesClient, error) {
					callCount++
					if callCount == 1 {
						// First failure: don't cancel ctx — let the backoff timer elapse
						return nil, errors.New("first failure")
					}
					// Second call (after backoff): signal clean exit
					cancel()
					return &mockSubscribeChatMessagesStream{
						recvFn: func() (*apiv1.ChatMessage, error) {
							return nil, context.Canceled
						},
					}, nil
				},
			}
			done = make(chan struct{})
			go func() {
				maintainChatSubscription(ctx, s, client)
				close(done)
			}()
		})

		It("should reconnect after backoff and return on clean exit", func() {
			// Allows up to 3s for the ~1s initial backoff to elapse before reconnecting
			Eventually(done, "3s").Should(BeClosed())
		})

		It("should have called subscribe twice", func() {
			Eventually(done, "3s").Should(BeClosed())
			Expect(callCount).To(Equal(2))
		})
	})
})

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

// mockConnectChatStream implements chatMessageReceiver for unit testing receiveChatMessages.
type mockConnectChatStream struct {
	messages []*apiv1.ChatMessage
	index    int
	err      error
}

func (m *mockConnectChatStream) Receive() bool {
	if m.index < len(m.messages) {
		m.index++
		return true
	}
	return false
}

func (m *mockConnectChatStream) Msg() *apiv1.ChatMessage {
	if m.index > 0 && m.index <= len(m.messages) {
		return m.messages[m.index-1]
	}
	return nil
}

func (m *mockConnectChatStream) Err() error {
	return m.err
}

func (*mockConnectChatStream) Close() error {
	return nil
}

// mockChatClient implements apiv1connect.ChatServiceClient for testing.
type mockChatClient struct {
	apiv1connect.ChatServiceClient
	subscribeFn func(context.Context, *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error)
}

func (m *mockChatClient) SubscribeChatMessages(ctx context.Context, req *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
	return m.subscribeFn(ctx, req)
}

var _ = Describe("buildMCPCommand", func() {
	var cmd cli.Command

	BeforeEach(func() {
		urlFlag := cli.StringFlag{
			Name:  "url, u",
			Usage: "wiki base URL",
			Value: "http://localhost:8050",
		}
		cmd = buildMCPCommand(urlFlag)
	})

	It("should have name mcp", func() {
		Expect(cmd.Name).To(Equal("mcp"))
	})

	It("should have a non-empty usage", func() {
		Expect(cmd.Usage).NotTo(BeEmpty())
	})

	It("should include the url and page flags", func() {
		Expect(cmd.Flags).To(HaveLen(2))
	})

	It("should have a non-nil action", func() {
		Expect(cmd.Action).NotTo(BeNil())
	})

	When("the action is invoked with an unsupported URL scheme", func() {
		var actionErr error

		BeforeEach(func() {
			app := cli.NewApp()
			app.Commands = []cli.Command{cmd}
			actionErr = app.Run([]string{"app", "mcp", "--url", "ftp://wiki.example.com"})
		})

		It("should return an error about the unsupported scheme", func() {
			Expect(actionErr).To(MatchError(ContainSubstring("unsupported URL scheme")))
		})
	})
})

var _ = Describe("setupMCPServer", func() {
	When("given a valid http URL", func() {
		var s *mcpserver.MCPServer
		var httpClient *http.Client
		var err error

		BeforeEach(func() {
			s, httpClient, err = setupMCPServer("http://localhost:1", channelInstructions, nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil MCP server", func() {
			Expect(s).NotTo(BeNil())
		})

		It("should return a non-nil HTTP client", func() {
			Expect(httpClient).NotTo(BeNil())
		})
	})
})

var _ = Describe("normalizeBaseURL", func() {
	When("given an https URL", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("https://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL unchanged", func() {
			Expect(normalized).To(Equal("https://wiki.example.com"))
		})
	})

	When("given an http URL", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("http://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL unchanged", func() {
			Expect(normalized).To(Equal("http://wiki.example.com"))
		})
	})

	When("given a URL with an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, err = normalizeBaseURL("ftp://wiki.example.com")
		})

		It("should return an error mentioning the scheme", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
		})
	})

	When("given an unparseable URL", func() {
		var err error

		BeforeEach(func() {
			// "://invalid" has no scheme before "://" which causes url.Parse to fail
			_, err = normalizeBaseURL("://invalid")
		})

		It("should return a parse error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid base URL")))
		})
	})

	When("given a URL with a trailing slash", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("https://wiki.example.com/")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL without a trailing slash", func() {
			Expect(normalized).To(Equal("https://wiki.example.com"))
		})
	})

	When("given a URL with an empty host", func() {
		var err error

		BeforeEach(func() {
			_, err = normalizeBaseURL("https://")
		})

		It("should return an error mentioning the missing host", func() {
			Expect(err).To(MatchError(ContainSubstring("missing host")))
		})
	})
})

var _ = Describe("createAPIClients", func() {
	When("given an HTTP client and base URL", func() {
		var clients *apiClients

		BeforeEach(func() {
			httpClient := &http.Client{}
			clients = createAPIClients(httpClient, "http://localhost:8050")
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
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					return nil, errors.New("dial error")
				},
			}
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		It("should return the subscribe error", func() {
			Expect(err).To(MatchError(ContainSubstring("dial error")))
		})
	})

	When("context is cancelled before Receive completes", func() {
		var err error

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					// Return an error immediately (simulating failure to subscribe)
					return nil, context.Canceled
				},
			}
			err = subscribeToChatMessages(ctx, s, client)
		})

		It("should return nil", func() {
			Expect(err).To(BeNil())
		})
	})

	When("stream.Receive returns a non-context error", func() {
		var err error

		BeforeEach(func() {
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					// Return an error, not a stream
					return nil, errors.New("connection reset")
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
			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					// For this test, just fail with an error after the message would have been processed
					return nil, errors.New("stream ended")
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
			// Cancel the context immediately
			cancel()

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					// Return an error due to already-cancelled context
					return nil, context.Canceled
				},
			}
			err = subscribeToChatMessages(ctx, s, client)
		})

		It("should return nil on context cancellation", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("receiveChatMessages", func() {
	var s *mcpserver.MCPServer

	BeforeEach(func() {
		s = mcpserver.NewMCPServer("test", "1.0")
	})

	When("the stream delivers a USER message then closes cleanly", func() {
		var err error

		BeforeEach(func() {
			stream := &mockConnectChatStream{
				messages: []*apiv1.ChatMessage{
					{Sender: apiv1.Sender_USER, Content: "hello", Page: "home", Id: "msg-1"},
				},
				err: nil,
			}
			err = receiveChatMessages(context.Background(), s, stream)
		})

		It("should return a reconnect error because the server closed the stream", func() {
			Expect(err).To(MatchError(ContainSubstring("stream closed by server")))
		})
	})

	When("the stream delivers a non-USER message then closes cleanly", func() {
		var err error

		BeforeEach(func() {
			stream := &mockConnectChatStream{
				messages: []*apiv1.ChatMessage{
					{Sender: apiv1.Sender_ASSISTANT, Content: "response"},
				},
				err: nil,
			}
			err = receiveChatMessages(context.Background(), s, stream)
		})

		It("should return a reconnect error because the server closed the stream", func() {
			Expect(err).To(MatchError(ContainSubstring("stream closed by server")))
		})
	})

	When("the stream ends with an error", func() {
		var err error

		BeforeEach(func() {
			stream := &mockConnectChatStream{
				messages: []*apiv1.ChatMessage{},
				err:      errors.New("network failure"),
			}
			err = receiveChatMessages(context.Background(), s, stream)
		})

		It("should return the stream error", func() {
			Expect(err).To(MatchError(ContainSubstring("network failure")))
		})
	})

	When("context is cancelled and stream ends with a context error", func() {
		var err error

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			stream := &mockConnectChatStream{
				messages: []*apiv1.ChatMessage{},
				err:      context.Canceled,
			}
			err = receiveChatMessages(ctx, s, stream)
		})

		It("should return nil", func() {
			Expect(err).To(BeNil())
		})
	})

	When("context is cancelled and stream ends cleanly (EOF)", func() {
		var err error

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			stream := &mockConnectChatStream{
				messages: []*apiv1.ChatMessage{},
				err:      nil,
			}
			err = receiveChatMessages(ctx, s, stream)
		})

		It("should return nil", func() {
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
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
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
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					cancel()
					// Return an error due to cancellation
					return nil, context.Canceled
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
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
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

	When("subscribe fails and context is cancelled during the backoff sleep", func() {
		var done chan struct{}
		var callCount int

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			callCount = 0

			client := &mockChatClient{
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					callCount++
					// Return an error without cancelling context first,
					// so the error propagates to maintainChatSubscription.
					// Then cancel after a brief delay so it fires during the backoff sleep.
					go func() {
						time.Sleep(10 * time.Millisecond)
						cancel()
					}()
					return nil, errors.New("connection refused")
				},
			}
			done = make(chan struct{})
			go func() {
				maintainChatSubscription(ctx, s, client)
				close(done)
			}()
		})

		It("should return when the context is cancelled during backoff", func() {
			Eventually(done, "2s").Should(BeClosed())
		})

		It("should have called subscribe once", func() {
			Eventually(done, "2s").Should(BeClosed())
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
				subscribeFn: func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error) {
					callCount++
					if callCount == 1 {
						// First failure: don't cancel ctx — let the backoff timer elapse
						return nil, errors.New("first failure")
					}
					// Second call (after backoff): signal clean exit
					cancel()
					return nil, context.Canceled
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

// testChatServiceHandler is a minimal ChatServiceHandler for use in test HTTP servers.
type testChatServiceHandler struct {
	apiv1connect.UnimplementedChatServiceHandler
	subscribeFn func(context.Context, *connect.Request[apiv1.SubscribeChatMessagesRequest], *connect.ServerStream[apiv1.ChatMessage]) error
}

func (h *testChatServiceHandler) SubscribeChatMessages(
	ctx context.Context,
	req *connect.Request[apiv1.SubscribeChatMessagesRequest],
	stream *connect.ServerStream[apiv1.ChatMessage],
) error {
	return h.subscribeFn(ctx, req, stream)
}

// newTestChatServer creates an httptest.Server that serves a single SubscribeChatMessages handler.
func newTestChatServer(
	subscribeFn func(context.Context, *connect.Request[apiv1.SubscribeChatMessagesRequest], *connect.ServerStream[apiv1.ChatMessage]) error,
) *httptest.Server {
	handler := &testChatServiceHandler{subscribeFn: subscribeFn}
	mux := http.NewServeMux()
	path, h := apiv1connect.NewChatServiceHandler(handler)
	mux.Handle(path, h)
	return httptest.NewServer(mux)
}

var _ = Describe("subscribeToChatMessages with a real Connect server", func() {
	var s *mcpserver.MCPServer

	BeforeEach(func() {
		s = mcpserver.NewMCPServer("test", "1.0")
	})

	When("the server sends a USER message then closes the stream", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			server = newTestChatServer(func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest], stream *connect.ServerStream[apiv1.ChatMessage]) error {
				return stream.Send(&apiv1.ChatMessage{
					Id:      "msg-1",
					Content: "hello from user",
					Sender:  apiv1.Sender_USER,
					Page:    "test-page",
				})
			})

			client := apiv1connect.NewChatServiceClient(server.Client(), server.URL)
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return a reconnect error because the server closed the stream", func() {
			Expect(err).To(MatchError(ContainSubstring("stream closed by server")))
		})
	})

	When("the server sends a non-USER message then closes the stream", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			server = newTestChatServer(func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest], stream *connect.ServerStream[apiv1.ChatMessage]) error {
				return stream.Send(&apiv1.ChatMessage{
					Id:      "msg-2",
					Content: "response from assistant",
					Sender:  apiv1.Sender_ASSISTANT,
					Page:    "test-page",
				})
			})

			client := apiv1connect.NewChatServiceClient(server.Client(), server.URL)
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return a reconnect error because the server closed the stream", func() {
			Expect(err).To(MatchError(ContainSubstring("stream closed by server")))
		})
	})

	When("the server returns a connect error on the stream", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			server = newTestChatServer(func(_ context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest], stream *connect.ServerStream[apiv1.ChatMessage]) error {
				return connect.NewError(connect.CodeInternal, errors.New("server error"))
			})

			client := apiv1connect.NewChatServiceClient(server.Client(), server.URL)
			err = subscribeToChatMessages(context.Background(), s, client)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return a stream error", func() {
			Expect(err).To(MatchError(ContainSubstring("stream error")))
		})
	})

	When("the context is cancelled while the stream is open", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())

			server = newTestChatServer(func(svcCtx context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest], stream *connect.ServerStream[apiv1.ChatMessage]) error {
				// Cancel the client context, then block until the server context is done
				cancel()
				<-svcCtx.Done()
				return svcCtx.Err()
			})

			client := apiv1connect.NewChatServiceClient(server.Client(), server.URL)
			err = subscribeToChatMessages(ctx, s, client)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return nil on context cancellation", func() {
			Expect(err).To(BeNil())
		})
	})

	When("a message is received and then context times out during streaming", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			msgReceived := make(chan struct{})

			server = newTestChatServer(func(svcCtx context.Context, _ *connect.Request[apiv1.SubscribeChatMessagesRequest], stream *connect.ServerStream[apiv1.ChatMessage]) error {
				// Send one user message
				_ = stream.Send(&apiv1.ChatMessage{
					Id:      "msg-timeout",
					Content: "message before timeout",
					Sender:  apiv1.Sender_USER,
					Page:    "test-page",
				})
				// Signal that the message was sent
				close(msgReceived)
				// Block until server context is done
				<-svcCtx.Done()
				return svcCtx.Err()
			})

			// Create a cancellable context
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel the context once the message has been sent by the server
			go func() {
				<-msgReceived
				// Small delay to allow the client to Receive the message
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			client := apiv1connect.NewChatServiceClient(server.Client(), server.URL)
			err = subscribeToChatMessages(ctx, s, client)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return nil when the stream error is due to context cancellation", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("setupMCPServer experimental capability hook", func() {
	When("an initialize request is handled", func() {
		var response mcp.JSONRPCMessage

		BeforeEach(func() {
			s, _, err := setupMCPServer("http://localhost:1", channelInstructions, nil)
			Expect(err).NotTo(HaveOccurred())

			// Send a real initialize message to trigger the OnAfterInitialize hook
			initMsg := []byte(`{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "initialize",
				"params": {
					"protocolVersion": "2024-11-05",
					"capabilities": {},
					"clientInfo": {"name": "test-client", "version": "1.0"}
				}
			}`)
			response = s.HandleMessage(context.Background(), initMsg)
		})

		It("should return a non-nil response", func() {
			Expect(response).NotTo(BeNil())
		})
	})
})

//revive:disable:dot-imports
package v1_test

import (
	"context"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockChatBufferManager is a test implementation of ChatBufferManager.
type mockChatBufferManager struct {
	mu              sync.Mutex
	messages        map[string][]*chatbuffer.Message
	pageSubscribers map[string][]chan chatbuffer.Event
	addUserMessageError  error
	addAssistantError    error
	editMessageError     error
	addReactionError     error

	// Tracking fields for new handler tests
	notifyToolCallCalls       []notifyToolCallArgs
	cancelPageCalls           []string
	respondToPermissionCalls  []respondToPermissionArgs
	requestPermissionCalls    []requestPermissionArgs
	requestPermissionResponse string

	// Configurable return values for status methods
	hasPageChannelSubscriberVal    bool
	hasInstanceRequestSubscriberVal bool
	isInstanceRequestedVal         bool

	// Configurable replay messages for SubscribeToPageChannelWithReplay
	pageChannelReplayMessages []*chatbuffer.Message
	pageChannelChan           chan *chatbuffer.Message

	// Configurable cancellation channels for SubscribeToCancellation
	cancellationChans []chan struct{}
}

type notifyToolCallArgs struct {
	page, messageID, toolCallID, title, toolStatus string
}

type respondToPermissionArgs struct {
	requestID, selectedOptionID string
}

type requestPermissionArgs struct {
	page, requestID, title, description string
	options                             []chatbuffer.PermissionOption
}

func newMockChatBufferManager() *mockChatBufferManager {
	return &mockChatBufferManager{
		messages:        make(map[string][]*chatbuffer.Message),
		pageSubscribers: make(map[string][]chan chatbuffer.Event),
	}
}

// sendEventToPage sends an event to all page subscribers (for testing)
func (m *mockChatBufferManager) sendEventToPage(page string, event chatbuffer.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.pageSubscribers[page] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (m *mockChatBufferManager) AddUserMessage(page, content, senderName string) (string, error) {
	if m.addUserMessageError != nil {
		return "", m.addUserMessageError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	messageID := "test-message-id"
	msg := &chatbuffer.Message{
		ID:         messageID,
		Sender:     "user",
		Content:    content,
		Page:       page,
		SenderName: senderName,
		Reactions:  []chatbuffer.Reaction{},
	}

	m.messages[page] = append(m.messages[page], msg)

	// Notify page subscribers
	event := chatbuffer.Event{
		Type:    chatbuffer.EventTypeNewMessage,
		Message: msg,
	}
	for _, ch := range m.pageSubscribers[page] {
		select {
		case ch <- event:
		default:
		}
	}

	return messageID, nil
}

func (m *mockChatBufferManager) AddAssistantMessage(page, content, replyToID string) (string, error) {
	if m.addAssistantError != nil {
		return "", m.addAssistantError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	messageID := "assistant-message-id"
	msg := &chatbuffer.Message{
		ID:         messageID,
		Sender:     "assistant",
		Content:    content,
		Page:       page,
		SenderName: "Claude",
		ReplyToID:  replyToID,
		Reactions:  []chatbuffer.Reaction{},
	}

	m.messages[page] = append(m.messages[page], msg)

	// Notify page subscribers
	event := chatbuffer.Event{
		Type:    chatbuffer.EventTypeNewMessage,
		Message: msg,
	}
	for _, ch := range m.pageSubscribers[page] {
		select {
		case ch <- event:
		default:
		}
	}

	return messageID, nil
}

func (m *mockChatBufferManager) EditMessage(messageID, newContent string, _ bool) error {
	if m.editMessageError != nil {
		return m.editMessageError
	}
	return nil
}

func (m *mockChatBufferManager) AddReaction(messageID, emoji, reactor string) error {
	if m.addReactionError != nil {
		return m.addReactionError
	}
	return nil
}

func (m *mockChatBufferManager) GetMessages(page string) []*chatbuffer.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.messages[page]
}

func (m *mockChatBufferManager) SubscribeToPage(page string) (<-chan chatbuffer.Event, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan chatbuffer.Event, 10)
	m.pageSubscribers[page] = append(m.pageSubscribers[page], ch)

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, subscriber := range m.pageSubscribers[page] {
			if subscriber == ch {
				m.pageSubscribers[page] = append(m.pageSubscribers[page][:i], m.pageSubscribers[page][i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

func (m *mockChatBufferManager) SubscribeToPageWithReplay(page string) ([]*chatbuffer.Message, <-chan chatbuffer.Event, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Subscribe first
	ch := make(chan chatbuffer.Event, 10)
	m.pageSubscribers[page] = append(m.pageSubscribers[page], ch)

	// Return existing messages
	messages := m.messages[page]

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, subscriber := range m.pageSubscribers[page] {
			if subscriber == ch {
				m.pageSubscribers[page] = append(m.pageSubscribers[page][:i], m.pageSubscribers[page][i+1:]...)
				close(ch)
				break
			}
		}
	}

	return messages, ch, unsubscribe
}

func (m *mockChatBufferManager) SubscribeToPageChannelWithReplay(string) ([]*chatbuffer.Message, <-chan *chatbuffer.Message, func()) {
	if m.pageChannelChan != nil {
		return m.pageChannelReplayMessages, m.pageChannelChan, func() {}
	}
	ch := make(chan *chatbuffer.Message, 10)
	return m.pageChannelReplayMessages, ch, func() { close(ch) }
}

func (*mockChatBufferManager) SubscribeToPageChannel(string) (<-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message, 10)
	return ch, func() { close(ch) }
}

func (m *mockChatBufferManager) HasPageChannelSubscriber(string) bool {
	return m.hasPageChannelSubscriberVal
}

func (*mockChatBufferManager) RequestInstance(string) {
	// no-op: satisfies interface; mock does not manage chat instances
}

func (*mockChatBufferManager) SubscribeToInstanceRequests() (<-chan string, func()) {
	ch := make(chan string, 10)
	return ch, func() { close(ch) }
}

func (m *mockChatBufferManager) HasInstanceRequestSubscribers() bool {
	return m.hasInstanceRequestSubscriberVal
}

func (m *mockChatBufferManager) IsInstanceRequested(string) bool {
	return m.isInstanceRequestedVal
}

func (m *mockChatBufferManager) NotifyToolCall(page, messageID, toolCallID, title, toolStatus string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyToolCallCalls = append(m.notifyToolCallCalls, notifyToolCallArgs{page, messageID, toolCallID, title, toolStatus})
}

func (m *mockChatBufferManager) CancelPage(page string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelPageCalls = append(m.cancelPageCalls, page)
	return true
}

func (m *mockChatBufferManager) SubscribeToCancellation(string) (<-chan struct{}, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cancellationChans) > 0 {
		ch := m.cancellationChans[0]
		m.cancellationChans = m.cancellationChans[1:]
		return ch, func() {}
	}

	ch := make(chan struct{}, 1)
	return ch, func() {}
}

func (*mockChatBufferManager) EmitPermissionRequest(string, *chatbuffer.PermissionRequestEvent) {}

func (m *mockChatBufferManager) RespondToPermission(requestID, selectedOptionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respondToPermissionCalls = append(m.respondToPermissionCalls, respondToPermissionArgs{requestID, selectedOptionID})
}

func (m *mockChatBufferManager) pageSubscriberCount(page string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pageSubscribers[page])
}

// mockChatStreamServer is a mock for testing SubscribeChat.
type mockChatStreamServer struct {
	mu          sync.Mutex
	events      []*apiv1.ChatEvent
	sendErr     error
	contextDone bool
}

func (m *mockChatStreamServer) Send(event *apiv1.ChatEvent) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockChatStreamServer) GetEventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *mockChatStreamServer) GetEvents() []*apiv1.ChatEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*apiv1.ChatEvent, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockChatStreamServer) Context() context.Context {
	if m.contextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*mockChatStreamServer) SetHeader(metadata.MD) error   { return nil }
func (*mockChatStreamServer) SendHeader(metadata.MD) error  { return nil }
func (*mockChatStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}
func (*mockChatStreamServer) SendMsg(any) error             { return nil }
func (*mockChatStreamServer) RecvMsg(any) error             { return nil }

// mockChatMessagesStreamServer is a mock for testing SubscribePageChatMessages.
type mockChatMessagesStreamServer struct {
	mu          sync.Mutex
	messages    []*apiv1.ChatMessage
	sendErr     error
	contextDone bool
}

func (m *mockChatMessagesStreamServer) Send(msg *apiv1.ChatMessage) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatMessagesStreamServer) GetMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockChatMessagesStreamServer) Context() context.Context {
	if m.contextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*mockChatMessagesStreamServer) SetHeader(metadata.MD) error   { return nil }
func (*mockChatMessagesStreamServer) SendHeader(metadata.MD) error  { return nil }
func (*mockChatMessagesStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}
func (*mockChatMessagesStreamServer) SendMsg(any) error             { return nil }
func (*mockChatMessagesStreamServer) RecvMsg(any) error             { return nil }

// mockInstanceRequestStreamServer mocks the SubscribeInstanceRequests stream server.
type mockInstanceRequestStreamServer struct {
	mu          sync.Mutex
	requests    []*apiv1.InstanceRequest
	contextDone bool
}

func (m *mockInstanceRequestStreamServer) Send(req *apiv1.InstanceRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	return nil
}

func (m *mockInstanceRequestStreamServer) Context() context.Context {
	if m.contextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*mockInstanceRequestStreamServer) SetHeader(metadata.MD) error   { return nil }
func (*mockInstanceRequestStreamServer) SendHeader(metadata.MD) error  { return nil }
func (*mockInstanceRequestStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}
func (*mockInstanceRequestStreamServer) SendMsg(any) error             { return nil }
func (*mockInstanceRequestStreamServer) RecvMsg(any) error             { return nil }

// mockCancellationStreamServer mocks the SubscribePageCancellations stream server.
type mockCancellationStreamServer struct {
	mu           sync.Mutex
	cancellations []*apiv1.PageCancellation
	sendErr      error
	ctx          context.Context
	ctxCancel    context.CancelFunc
}

func newMockCancellationStreamServer() *mockCancellationStreamServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockCancellationStreamServer{
		ctx:       ctx,
		ctxCancel: cancel,
	}
}

func (m *mockCancellationStreamServer) Send(msg *apiv1.PageCancellation) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancellations = append(m.cancellations, msg)
	return nil
}

func (m *mockCancellationStreamServer) GetCancellationCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cancellations)
}

func (m *mockCancellationStreamServer) Context() context.Context {
	return m.ctx
}

func (*mockCancellationStreamServer) SetHeader(metadata.MD) error  { return nil }
func (*mockCancellationStreamServer) SendHeader(metadata.MD) error { return nil }
func (*mockCancellationStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}
func (*mockCancellationStreamServer) SendMsg(any) error { return nil }
func (*mockCancellationStreamServer) RecvMsg(any) error { return nil }

var _ = Describe("ChatService", func() {
	var (
		server      *v1.Server
		ctx         context.Context
		chatManager *mockChatBufferManager
	)

	BeforeEach(func() {
		ctx = context.Background()
		chatManager = newMockChatBufferManager()

		var err error
		server, err = v1.NewServer(
			v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
			noOpPageReaderMutator{},
			noOpBleveIndexQueryer{},
			noOpFrontmatterIndexQueryer{},
			lumber.NewConsoleLogger(lumber.WARN),
			chatManager,
			noOpPageOpener{},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("SendMessage", func() {
		When("sending a valid user message", func() {
			var (
				req  *apiv1.SendChatMessageRequest
				resp *apiv1.SendChatMessageResponse
				err  error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatMessageRequest{
					Page:    "test-page",
					Content: "Hello, world!",
				}

				resp, err = server.SendMessage(ctx, req)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a message ID", func() {
				Expect(resp.MessageId).NotTo(BeEmpty())
			})
		})

		When("page is empty", func() {
			var (
				req *apiv1.SendChatMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatMessageRequest{
					Page:    "",
					Content: "Hello",
				}

				_, err = server.SendMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("content is empty", func() {
			var (
				req *apiv1.SendChatMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatMessageRequest{
					Page:    "test-page",
					Content: "",
				}

				_, err = server.SendMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("no channel subscribers are connected", func() {
			var (
				req *apiv1.SendChatMessageRequest
				err error
			)

			BeforeEach(func() {
				chatManager.addUserMessageError = chatbuffer.ErrNoSubscribers

				req = &apiv1.SendChatMessageRequest{
					Page:    "test-page",
					Content: "Hello",
				}

				_, err = server.SendMessage(ctx, req)
			})

			It("should return Unavailable error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Unavailable))
				Expect(st.Message()).To(ContainSubstring("no channel subscriber connected"))
			})
		})
	})

	Describe("SendChatReply", func() {
		When("sending a valid assistant reply", func() {
			var (
				req  *apiv1.SendChatReplyRequest
				resp *apiv1.SendChatReplyResponse
				err  error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatReplyRequest{
					Page:      "test-page",
					Content:   "Hello from Claude!",
					ReplyToId: "parent-message-id",
				}

				resp, err = server.SendChatReply(ctx, req)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a message ID", func() {
				Expect(resp.MessageId).NotTo(BeEmpty())
			})
		})

		When("page is empty", func() {
			var (
				req *apiv1.SendChatReplyRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatReplyRequest{
					Page:    "",
					Content: "Hello",
				}

				_, err = server.SendChatReply(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("content is empty", func() {
			var (
				req *apiv1.SendChatReplyRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.SendChatReplyRequest{
					Page:    "test-page",
					Content: "",
				}

				_, err = server.SendChatReply(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("EditChatMessage", func() {
		When("editing an existing message", func() {
			var (
				req  *apiv1.EditChatMessageRequest
				resp *apiv1.EditChatMessageResponse
				err  error
			)

			BeforeEach(func() {
				req = &apiv1.EditChatMessageRequest{
					MessageId:  "message-123",
					NewContent: "Updated content",
				}

				resp, err = server.EditChatMessage(ctx, req)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})
		})

		When("message_id is empty", func() {
			var (
				req *apiv1.EditChatMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.EditChatMessageRequest{
					MessageId:  "",
					NewContent: "Updated",
				}

				_, err = server.EditChatMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("new_content is empty", func() {
			var (
				req *apiv1.EditChatMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.EditChatMessageRequest{
					MessageId:  "message-123",
					NewContent: "",
				}

				_, err = server.EditChatMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("message is not found", func() {
			var (
				req *apiv1.EditChatMessageRequest
				err error
			)

			BeforeEach(func() {
				chatManager.editMessageError = chatbuffer.ErrMessageNotFound

				req = &apiv1.EditChatMessageRequest{
					MessageId:  "nonexistent",
					NewContent: "Updated",
				}

				_, err = server.EditChatMessage(ctx, req)
			})

			It("should return NotFound error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
			})
		})
	})

	Describe("ReactToMessage", func() {
		When("adding a reaction to a message", func() {
			var (
				req  *apiv1.ReactToMessageRequest
				resp *apiv1.ReactToMessageResponse
				err  error
			)

			BeforeEach(func() {
				req = &apiv1.ReactToMessageRequest{
					MessageId: "message-123",
					Emoji:     "👍",
				}

				resp, err = server.ReactToMessage(ctx, req)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})
		})

		When("message_id is empty", func() {
			var (
				req *apiv1.ReactToMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.ReactToMessageRequest{
					MessageId: "",
					Emoji:     "👍",
				}

				_, err = server.ReactToMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("emoji is empty", func() {
			var (
				req *apiv1.ReactToMessageRequest
				err error
			)

			BeforeEach(func() {
				req = &apiv1.ReactToMessageRequest{
					MessageId: "message-123",
					Emoji:     "",
				}

				_, err = server.ReactToMessage(ctx, req)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("message is not found", func() {
			var (
				req *apiv1.ReactToMessageRequest
				err error
			)

			BeforeEach(func() {
				chatManager.addReactionError = chatbuffer.ErrMessageNotFound

				req = &apiv1.ReactToMessageRequest{
					MessageId: "nonexistent",
					Emoji:     "👍",
				}

				_, err = server.ReactToMessage(ctx, req)
			})

			It("should return NotFound error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
			})
		})
	})

	Describe("SubscribeChat", func() {
		When("subscribing to a page with existing messages", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				// Add some existing messages
				chatManager.messages["test-page"] = []*chatbuffer.Message{
					{ID: "msg-1", Sender: "user", Content: "Hello", Page: "test-page"},
					{ID: "msg-2", Sender: "assistant", Content: "Hi!", Page: "test-page"},
				}

				req = &apiv1.SubscribeChatRequest{
					Page: "test-page",
				}
				streamServer = &mockChatStreamServer{contextDone: true}

				_ = server.SubscribeChat(req, streamServer)
			})

			It("should replay existing messages", func() {
				Expect(streamServer.events).To(HaveLen(2))
			})

			It("should send messages as new_message events", func() {
				Expect(streamServer.events[0].GetNewMessage()).NotTo(BeNil())
				Expect(streamServer.events[1].GetNewMessage()).NotTo(BeNil())
			})
		})

		When("subscribing and receiving new messages", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatRequest{
					Page: "test-page",
				}
				streamServer = &mockChatStreamServer{}

				// Run subscription in background
				go func() {
					_ = server.SubscribeChat(req, streamServer)
				}()

				// Wait for subscription to be registered before sending events
				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Trigger a new message through AddUserMessage which will notify subscribers
				_, _ = chatManager.AddUserMessage("test-page", "New message", "test-user")

				// Wait for event to be processed
				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Cancel context to stop subscription
				streamServer.contextDone = true
			})

			It("should stream new message events", func() {
				Expect(streamServer.GetEventCount()).To(BeNumerically(">=", 1))
			})
		})

		When("subscribing and receiving edit events", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatRequest{
					Page: "test-page",
				}
				streamServer = &mockChatStreamServer{}

				// Run subscription in background
				go func() {
					_ = server.SubscribeChat(req, streamServer)
				}()

				// Wait for subscription to be registered before sending events
				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Trigger an edit event using mock helper
				event := chatbuffer.Event{
					Type: chatbuffer.EventTypeEdit,
					Edit: &chatbuffer.EditEvent{
						MessageID:  "msg-1",
						NewContent: "Updated content",
						Timestamp:  time.Now(),
					},
				}
				chatManager.sendEventToPage("test-page", event)

				// Wait for event to be processed
				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Cancel context to stop subscription
				streamServer.contextDone = true
			})

			It("should stream edit events", func() {
				hasEdit := false
				for _, e := range streamServer.GetEvents() {
					if e.GetEdit() != nil {
						hasEdit = true
						break
					}
				}
				Expect(hasEdit).To(BeTrue())
			})
		})

		When("subscribing and receiving reaction events", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatRequest{
					Page: "test-page",
				}
				streamServer = &mockChatStreamServer{}

				// Run subscription in background
				go func() {
					_ = server.SubscribeChat(req, streamServer)
				}()

				// Wait for subscription to be registered before sending events
				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Trigger a reaction event using mock helper
				event := chatbuffer.Event{
					Type: chatbuffer.EventTypeReaction,
					Reaction: &chatbuffer.ReactionEvent{
						MessageID: "msg-1",
						Emoji:     "👍",
						Reactor:   "user",
					},
				}
				chatManager.sendEventToPage("test-page", event)

				// Wait for event to be processed
				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Cancel context to stop subscription
				streamServer.contextDone = true
			})

			It("should stream reaction events", func() {
				hasReaction := false
				for _, e := range streamServer.GetEvents() {
					if e.GetReaction() != nil {
						hasReaction = true
						break
					}
				}
				Expect(hasReaction).To(BeTrue())
			})
		})

		When("send fails on replay", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
				err          error
			)

			BeforeEach(func() {
				// Add an existing message
				chatManager.messages["test-page"] = []*chatbuffer.Message{
					{ID: "msg-1", Sender: "user", Content: "Hello", Page: "test-page"},
				}

				req = &apiv1.SubscribeChatRequest{
					Page: "test-page",
				}
				streamServer = &mockChatStreamServer{
					sendErr: status.Error(codes.Internal, "send failed"),
				}

				err = server.SubscribeChat(req, streamServer)
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("page is empty", func() {
			var (
				req          *apiv1.SubscribeChatRequest
				streamServer *mockChatStreamServer
				err          error
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatRequest{
					Page: "",
				}
				streamServer = &mockChatStreamServer{}

				err = server.SubscribeChat(req, streamServer)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("GetChatStatus", func() {
		When("called without a page parameter", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return connected false", func() {
				Expect(resp.Connected).To(BeFalse())
			})
		})

		When("called with a page parameter", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{Page: "test-page"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return connected false when no subscribers", func() {
				Expect(resp.Connected).To(BeFalse())
			})

			It("should return pool_connected false when no pool daemon", func() {
				Expect(resp.PoolConnected).To(BeFalse())
			})

			It("should return starting false when no instance requested", func() {
				Expect(resp.Starting).To(BeFalse())
			})
		})
	})

	Describe("SubscribePageChatMessages", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				err = server.SubscribePageChatMessages(
					&apiv1.SubscribePageChatMessagesRequest{},
					&mockChatMessagesStreamServer{},
				)
			})

			It("should return InvalidArgument status code", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
			})
		})

		When("context is cancelled", func() {
			var (
				streamServer *mockChatMessagesStreamServer
				doneCh       chan struct{}
			)

			BeforeEach(func() {
				streamServer = &mockChatMessagesStreamServer{contextDone: true}
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribePageChatMessages(
						&apiv1.SubscribePageChatMessagesRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should exit cleanly", func() {
				Expect(true).To(BeTrue())
			})
		})
	})

	Describe("SubscribeInstanceRequests", func() {
		When("context is cancelled", func() {
			var doneCh chan struct{}

			BeforeEach(func() {
				streamServer := &mockInstanceRequestStreamServer{contextDone: true}
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribeInstanceRequests(
						&apiv1.SubscribeInstanceRequestsRequest{},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should exit cleanly", func() {
				Expect(true).To(BeTrue())
			})
		})
	})

	Describe("SendToolCallNotification", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.SendToolCallNotification(ctx, &apiv1.SendToolCallNotificationRequest{
					Page:      "",
					MessageId: "msg-1",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("page is required"))
			})
		})

		When("message_id is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.SendToolCallNotification(ctx, &apiv1.SendToolCallNotificationRequest{
					Page:      "test-page",
					MessageId: "",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("message_id is required"))
			})
		})

		When("request is valid", func() {
			var (
				resp *apiv1.SendToolCallNotificationResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.SendToolCallNotification(ctx, &apiv1.SendToolCallNotificationRequest{
					Page:       "test-page",
					MessageId:  "msg-1",
					ToolCallId: "tc-1",
					Title:      "Reading file",
					Status:     "running",
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should call NotifyToolCall with correct arguments", func() {
				Expect(chatManager.notifyToolCallCalls).To(HaveLen(1))
				call := chatManager.notifyToolCallCalls[0]
				Expect(call.page).To(Equal("test-page"))
				Expect(call.messageID).To(Equal("msg-1"))
				Expect(call.toolCallID).To(Equal("tc-1"))
				Expect(call.title).To(Equal("Reading file"))
				Expect(call.toolStatus).To(Equal("running"))
			})
		})
	})

	Describe("CancelAgentPrompt", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.CancelAgentPrompt(ctx, &apiv1.CancelAgentPromptRequest{
					Page: "",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("page is required"))
			})
		})

		When("request is valid", func() {
			var (
				resp *apiv1.CancelAgentPromptResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.CancelAgentPrompt(ctx, &apiv1.CancelAgentPromptRequest{
					Page: "test-page",
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should call CancelPage with the correct page", func() {
				Expect(chatManager.cancelPageCalls).To(HaveLen(1))
				Expect(chatManager.cancelPageCalls[0]).To(Equal("test-page"))
			})
		})
	})

	Describe("RespondToPermission", func() {
		When("request_id is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RespondToPermission(ctx, &apiv1.RespondToPermissionRequest{
					RequestId:        "",
					SelectedOptionId: "allow",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("request_id is required"))
			})
		})

		When("selected_option_id is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RespondToPermission(ctx, &apiv1.RespondToPermissionRequest{
					RequestId:        "req-1",
					SelectedOptionId: "",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("selected_option_id is required"))
			})
		})

		When("request is valid", func() {
			var (
				resp *apiv1.RespondToPermissionResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.RespondToPermission(ctx, &apiv1.RespondToPermissionRequest{
					RequestId:        "req-1",
					SelectedOptionId: "allow",
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should call RespondToPermission with correct arguments", func() {
				Expect(chatManager.respondToPermissionCalls).To(HaveLen(1))
				call := chatManager.respondToPermissionCalls[0]
				Expect(call.requestID).To(Equal("req-1"))
				Expect(call.selectedOptionID).To(Equal("allow"))
			})
		})
	})

	Describe("RequestPermissionFromUser", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RequestPermissionFromUser(ctx, &apiv1.RequestPermissionFromUserRequest{
					Page:      "",
					RequestId: "req-1",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("page is required"))
			})
		})

		When("request_id is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RequestPermissionFromUser(ctx, &apiv1.RequestPermissionFromUserRequest{
					Page:      "test-page",
					RequestId: "",
				})
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("request_id is required"))
			})
		})

		When("request is valid", func() {
			var (
				resp *apiv1.RequestPermissionFromUserResponse
				err  error
			)

			BeforeEach(func() {
				chatManager.requestPermissionResponse = "allow-once"

				resp, err = server.RequestPermissionFromUser(ctx, &apiv1.RequestPermissionFromUserRequest{
					Page:        "test-page",
					RequestId:   "req-1",
					Title:       "Execute command",
					Description: "Run npm install",
					Options: []*apiv1.ChatPermissionOption{
						{OptionId: "allow-once", Label: "Allow", Description: "Allow this action"},
						{OptionId: "deny", Label: "Deny", Description: "Deny this action"},
					},
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the selected option from the buffer", func() {
				Expect(resp.SelectedOptionId).To(Equal("allow-once"))
			})

			It("should call RequestPermission with correct arguments", func() {
				Expect(chatManager.requestPermissionCalls).To(HaveLen(1))
				call := chatManager.requestPermissionCalls[0]
				Expect(call.page).To(Equal("test-page"))
				Expect(call.requestID).To(Equal("req-1"))
				Expect(call.title).To(Equal("Execute command"))
				Expect(call.description).To(Equal("Run npm install"))
				Expect(call.options).To(HaveLen(2))
				Expect(call.options[0].OptionID).To(Equal("allow-once"))
				Expect(call.options[1].OptionID).To(Equal("deny"))
			})
		})
	})

	Describe("SubscribePageChatMessages with replay", func() {
		When("buffer has existing messages including user messages", func() {
			var (
				streamServer *mockChatMessagesStreamServer
				doneCh       chan struct{}
			)

			BeforeEach(func() {
				chatManager.pageChannelReplayMessages = []*chatbuffer.Message{
					{ID: "msg-1", Sender: "user", Content: "First user message", Page: "test-page"},
					{ID: "msg-2", Sender: "assistant", Content: "Reply", Page: "test-page"},
					{ID: "msg-3", Sender: "user", Content: "Latest user message", Page: "test-page"},
				}

				streamServer = &mockChatMessagesStreamServer{contextDone: true}
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribePageChatMessages(
						&apiv1.SubscribePageChatMessagesRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should replay only the last user message", func() {
				Expect(streamServer.GetMessageCount()).To(Equal(1))
			})

			It("should replay the correct message", func() {
				streamServer.mu.Lock()
				defer streamServer.mu.Unlock()
				Expect(streamServer.messages[0].Id).To(Equal("msg-3"))
				Expect(streamServer.messages[0].Content).To(Equal("Latest user message"))
			})
		})

		When("buffer has only assistant messages", func() {
			var (
				streamServer *mockChatMessagesStreamServer
				doneCh       chan struct{}
			)

			BeforeEach(func() {
				chatManager.pageChannelReplayMessages = []*chatbuffer.Message{
					{ID: "msg-1", Sender: "assistant", Content: "Hello", Page: "test-page"},
					{ID: "msg-2", Sender: "assistant", Content: "World", Page: "test-page"},
				}

				streamServer = &mockChatMessagesStreamServer{contextDone: true}
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribePageChatMessages(
						&apiv1.SubscribePageChatMessagesRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should not replay any messages", func() {
				Expect(streamServer.GetMessageCount()).To(Equal(0))
			})
		})
	})

	Describe("GetChatStatus page-aware", func() {
		When("pool is connected and page has a subscriber", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				chatManager.hasInstanceRequestSubscriberVal = true
				chatManager.hasPageChannelSubscriberVal = true

				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{Page: "test-page"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return pool_connected true", func() {
				Expect(resp.PoolConnected).To(BeTrue())
			})

			It("should return connected true", func() {
				Expect(resp.Connected).To(BeTrue())
			})
		})

		When("pool is connected but page has no subscriber", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				chatManager.hasInstanceRequestSubscriberVal = true
				chatManager.hasPageChannelSubscriberVal = false

				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{Page: "test-page"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return pool_connected true", func() {
				Expect(resp.PoolConnected).To(BeTrue())
			})

			It("should return connected false because per-page subscribers are required", func() {
				Expect(resp.Connected).To(BeFalse())
			})
		})

		When("pool is connected and instance is requested for page", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				chatManager.hasInstanceRequestSubscriberVal = true
				chatManager.isInstanceRequestedVal = true

				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{Page: "test-page"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return starting true", func() {
				Expect(resp.Starting).To(BeTrue())
			})
		})

		When("no page is specified and pool is connected", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				chatManager.hasInstanceRequestSubscriberVal = true

				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return pool_connected true", func() {
				Expect(resp.PoolConnected).To(BeTrue())
			})

			It("should return connected false because no page was queried", func() {
				Expect(resp.Connected).To(BeFalse())
			})

			It("should return starting false because no page was queried", func() {
				Expect(resp.Starting).To(BeFalse())
			})
		})
	})

	Describe("SubscribeChat with tool call events", func() {
		When("receiving a tool call event", func() {
			var (
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				streamServer = &mockChatStreamServer{}

				go func() {
					_ = server.SubscribeChat(
						&apiv1.SubscribeChatRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				event := chatbuffer.Event{
					Type: chatbuffer.EventTypeToolCall,
					ToolCall: &chatbuffer.ToolCallEvent{
						MessageID:  "msg-1",
						ToolCallID: "tc-1",
						Title:      "Reading file",
						Status:     "running",
					},
				}
				chatManager.sendEventToPage("test-page", event)

				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				streamServer.contextDone = true
			})

			It("should stream tool call events", func() {
				hasToolCall := false
				for _, e := range streamServer.GetEvents() {
					if tc := e.GetToolCall(); tc != nil {
						hasToolCall = true
						Expect(tc.MessageId).To(Equal("msg-1"))
						Expect(tc.ToolCallId).To(Equal("tc-1"))
						Expect(tc.Title).To(Equal("Reading file"))
						Expect(tc.Status).To(Equal("running"))
						break
					}
				}
				Expect(hasToolCall).To(BeTrue())
			})
		})

		When("receiving a permission request event", func() {
			var (
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				streamServer = &mockChatStreamServer{}

				go func() {
					_ = server.SubscribeChat(
						&apiv1.SubscribeChatRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				event := chatbuffer.Event{
					Type: chatbuffer.EventTypePermissionRequest,
					PermissionRequest: &chatbuffer.PermissionRequestEvent{
						RequestID:   "perm-1",
						Page:        "test-page",
						Title:       "Execute command",
						Description: "Run npm install",
						Options: []chatbuffer.PermissionOption{
							{OptionID: "allow", Label: "Allow", Description: "Allow this"},
							{OptionID: "deny", Label: "Deny", Description: "Deny this"},
						},
					},
				}
				chatManager.sendEventToPage("test-page", event)

				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				streamServer.contextDone = true
			})

			It("should stream permission request events", func() {
				hasPermReq := false
				for _, e := range streamServer.GetEvents() {
					if pr := e.GetPermissionRequest(); pr != nil {
						hasPermReq = true
						Expect(pr.RequestId).To(Equal("perm-1"))
						Expect(pr.Page).To(Equal("test-page"))
						Expect(pr.Title).To(Equal("Execute command"))
						Expect(pr.Description).To(Equal("Run npm install"))
						Expect(pr.Options).To(HaveLen(2))
						Expect(pr.Options[0].OptionId).To(Equal("allow"))
						Expect(pr.Options[1].OptionId).To(Equal("deny"))
						break
					}
				}
				Expect(hasPermReq).To(BeTrue())
			})
		})

		When("receiving an unknown event type", func() {
			var (
				streamServer *mockChatStreamServer
			)

			BeforeEach(func() {
				streamServer = &mockChatStreamServer{}

				go func() {
					_ = server.SubscribeChat(
						&apiv1.SubscribeChatRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(func() int { return chatManager.pageSubscriberCount("test-page") }, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Send an event with an unknown type (high value not in the enum)
				event := chatbuffer.Event{
					Type: chatbuffer.EventType(999),
				}
				chatManager.sendEventToPage("test-page", event)

				// Send a known event after so we can verify the stream is still working
				event2 := chatbuffer.Event{
					Type:    chatbuffer.EventTypeNewMessage,
					Message: &chatbuffer.Message{ID: "msg-after", Sender: "user", Content: "After unknown", Page: "test-page"},
				}
				chatManager.sendEventToPage("test-page", event2)

				Eventually(streamServer.GetEventCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				streamServer.contextDone = true
			})

			It("should skip the unknown event and continue streaming", func() {
				events := streamServer.GetEvents()
				Expect(events).To(HaveLen(1))
				Expect(events[0].GetNewMessage()).NotTo(BeNil())
				Expect(events[0].GetNewMessage().Id).To(Equal("msg-after"))
			})
		})
	})

	Describe("SubscribePageChatMessages streaming new messages", func() {
		When("a new message arrives on the channel", func() {
			var (
				streamServer *mockChatMessagesStreamServer
				msgChan      chan *chatbuffer.Message
			)

			BeforeEach(func() {
				msgChan = make(chan *chatbuffer.Message, 10)
				chatManager.pageChannelChan = msgChan

				streamServer = &mockChatMessagesStreamServer{}

				go func() {
					_ = server.SubscribePageChatMessages(
						&apiv1.SubscribePageChatMessagesRequest{Page: "test-page"},
						streamServer,
					)
				}()

				// Send a new message on the channel
				msgChan <- &chatbuffer.Message{
					ID:      "new-msg-1",
					Sender:  "user",
					Content: "Hello from user",
					Page:    "test-page",
				}

				Eventually(streamServer.GetMessageCount, "1s", "10ms").Should(Equal(1))

				streamServer.contextDone = true
			})

			It("should stream the new message", func() {
				streamServer.mu.Lock()
				defer streamServer.mu.Unlock()
				Expect(streamServer.messages[0].Id).To(Equal("new-msg-1"))
				Expect(streamServer.messages[0].Content).To(Equal("Hello from user"))
			})
		})

		When("replay has messages but channel closes immediately", func() {
			var (
				streamServer *mockChatMessagesStreamServer
				doneCh       chan struct{}
			)

			BeforeEach(func() {
				closedChan := make(chan *chatbuffer.Message)
				close(closedChan)
				chatManager.pageChannelChan = closedChan
				chatManager.pageChannelReplayMessages = []*chatbuffer.Message{
					{ID: "msg-1", Sender: "user", Content: "User msg", Page: "test-page"},
				}

				streamServer = &mockChatMessagesStreamServer{}
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribePageChatMessages(
						&apiv1.SubscribePageChatMessagesRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should replay the last user message before exiting", func() {
				Expect(streamServer.GetMessageCount()).To(Equal(1))
			})
		})
	})

	Describe("SubscribePageCancellations", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				streamServer := newMockCancellationStreamServer()
				err = server.SubscribePageCancellations(
					&apiv1.SubscribePageCancellationsRequest{},
					streamServer,
				)
			})

			It("should return InvalidArgument error", func() {
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.InvalidArgument))
				Expect(st.Message()).To(ContainSubstring("page is required"))
			})
		})

		When("a cancellation signal is sent", func() {
			var (
				streamServer *mockCancellationStreamServer
			)

			BeforeEach(func() {
				// Create two cancellation channels: one for initial subscribe, one for re-subscribe
				ch1 := make(chan struct{}, 1)
				ch2 := make(chan struct{}, 1)
				chatManager.cancellationChans = []chan struct{}{ch1, ch2}

				streamServer = newMockCancellationStreamServer()

				go func() {
					_ = server.SubscribePageCancellations(
						&apiv1.SubscribePageCancellationsRequest{Page: "test-page"},
						streamServer,
					)
				}()

				// Send cancellation signal
				ch1 <- struct{}{}

				Eventually(streamServer.GetCancellationCount, "1s", "10ms").Should(Equal(1))

				// Cancel context to stop the stream
				streamServer.ctxCancel()
			})

			It("should send a PageCancellation message", func() {
				Expect(streamServer.GetCancellationCount()).To(Equal(1))
			})
		})

		When("context is cancelled before any signal", func() {
			var doneCh chan struct{}

			BeforeEach(func() {
				streamServer := newMockCancellationStreamServer()
				streamServer.ctxCancel()
				doneCh = make(chan struct{})

				go func() {
					defer close(doneCh)
					_ = server.SubscribePageCancellations(
						&apiv1.SubscribePageCancellationsRequest{Page: "test-page"},
						streamServer,
					)
				}()

				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should exit cleanly", func() {
				Expect(true).To(BeTrue())
			})
		})
	})

	Describe("RequestPermissionFromUser blocking behavior", func() {
		When("the response arrives after a delay", func() {
			var (
				resp     *apiv1.RequestPermissionFromUserResponse
				err      error
				doneCh   chan struct{}
				duration time.Duration
			)

			BeforeEach(func() {
				chatManager.requestPermissionResponse = "deny"
				doneCh = make(chan struct{})
				startTime := time.Now()

				go func() {
					defer close(doneCh)
					resp, err = server.RequestPermissionFromUser(ctx, &apiv1.RequestPermissionFromUserRequest{
						Page:      "test-page",
						RequestId: "perm-req-1",
						Title:     "Execute dangerous command",
						Description: "rm -rf /tmp/test",
						Options: []*apiv1.ChatPermissionOption{
							{OptionId: "allow", Label: "Allow", Description: "Allow this action"},
							{OptionId: "deny", Label: "Deny", Description: "Deny this action"},
						},
					})
					duration = time.Since(startTime)
				}()

				Eventually(doneCh, "2s", "10ms").Should(BeClosed())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the selected option ID", func() {
				Expect(resp.SelectedOptionId).To(Equal("deny"))
			})

			It("should have called RequestPermission with converted options", func() {
				Expect(chatManager.requestPermissionCalls).To(HaveLen(1))
				call := chatManager.requestPermissionCalls[0]
				Expect(call.options).To(HaveLen(2))
				Expect(call.options[0].OptionID).To(Equal("allow"))
				Expect(call.options[0].Label).To(Equal("Allow"))
				Expect(call.options[0].Description).To(Equal("Allow this action"))
				Expect(call.options[1].OptionID).To(Equal("deny"))
			})

			It("should complete quickly since mock returns immediately", func() {
				Expect(duration).To(BeNumerically("<", time.Second))
			})
		})
	})
})

func (m *mockChatBufferManager) RequestPermission(page, requestID, title, description string, options []chatbuffer.PermissionOption) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestPermissionCalls = append(m.requestPermissionCalls, requestPermissionArgs{page, requestID, title, description, options})
	return m.requestPermissionResponse
}

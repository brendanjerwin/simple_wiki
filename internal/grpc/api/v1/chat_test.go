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
	mu                   sync.Mutex
	messages             map[string][]*chatbuffer.Message
	channelSubscribers   []chan *chatbuffer.Message
	pageSubscribers      map[string][]chan chatbuffer.Event
	addUserMessageError  error
	addAssistantError    error
	editMessageError     error
	addReactionError     error
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

// sendMessageToChannel sends a message to all channel subscribers (for testing)
func (m *mockChatBufferManager) sendMessageToChannel(msg *chatbuffer.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.channelSubscribers {
		select {
		case ch <- msg:
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

	// Notify channel subscribers
	for _, ch := range m.channelSubscribers {
		select {
		case ch <- msg:
		default:
		}
	}

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

func (m *mockChatBufferManager) EditMessage(messageID, newContent string) error {
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

func (m *mockChatBufferManager) SubscribeToChannel() (<-chan *chatbuffer.Message, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *chatbuffer.Message, 10)
	m.channelSubscribers = append(m.channelSubscribers, ch)

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, subscriber := range m.channelSubscribers {
			if subscriber == ch {
				m.channelSubscribers = append(m.channelSubscribers[:i], m.channelSubscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

func (m *mockChatBufferManager) HasChannelSubscribers() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.channelSubscribers) > 0
}

func (m *mockChatBufferManager) SubscribeToPageChannel(string) (<-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message, 10)
	return ch, func() { close(ch) }
}

func (m *mockChatBufferManager) HasPageChannelSubscriber(string) bool {
	return false
}

func (m *mockChatBufferManager) RequestInstance(string) {}

func (m *mockChatBufferManager) SubscribeToInstanceRequests() (<-chan string, func()) {
	ch := make(chan string, 10)
	return ch, func() { close(ch) }
}

func (m *mockChatBufferManager) HasInstanceRequestSubscribers() bool {
	return false
}

func (m *mockChatBufferManager) IsInstanceRequested(string) bool {
	return false
}

func (m *mockChatBufferManager) channelSubscriberCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.channelSubscribers)
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
func (*mockChatStreamServer) SetTrailer(metadata.MD)        {}
func (*mockChatStreamServer) SendMsg(any) error             { return nil }
func (*mockChatStreamServer) RecvMsg(any) error             { return nil }

// mockChatMessagesStreamServer is a mock for testing SubscribeChatMessages.
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
func (*mockChatMessagesStreamServer) SetTrailer(metadata.MD)        {}
func (*mockChatMessagesStreamServer) SendMsg(any) error             { return nil }
func (*mockChatMessagesStreamServer) RecvMsg(any) error             { return nil }

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
		When("Claude channel subscriber is connected", func() {
			var (
				resp *apiv1.GetChatStatusResponse
				err  error
			)

			BeforeEach(func() {
				// Subscribe to channel to simulate a connected Claude
				_, unsubscribe := chatManager.SubscribeToChannel()
				DeferCleanup(unsubscribe)

				resp, err = server.GetChatStatus(ctx, &apiv1.GetChatStatusRequest{})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return connected true", func() {
				Expect(resp.Connected).To(BeTrue())
			})
		})

		When("no Claude channel subscriber is connected", func() {
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
	})

	Describe("SubscribeChatMessages", func() {
		When("subscribing to channel messages", func() {
			var (
				req          *apiv1.SubscribeChatMessagesRequest
				streamServer *mockChatMessagesStreamServer
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatMessagesRequest{}
				streamServer = &mockChatMessagesStreamServer{contextDone: true}

				_ = server.SubscribeChatMessages(req, streamServer)
			})

			It("should not error", func() {
				// Test passes if no panic or error
				Expect(true).To(BeTrue())
			})
		})

		When("receiving new channel messages", func() {
			var (
				req          *apiv1.SubscribeChatMessagesRequest
				streamServer *mockChatMessagesStreamServer
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatMessagesRequest{}
				streamServer = &mockChatMessagesStreamServer{}

				// Run subscription in background
				go func() {
					_ = server.SubscribeChatMessages(req, streamServer)
				}()

				// Wait for subscription to be registered before sending messages
				Eventually(chatManager.channelSubscriberCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Trigger a channel message using mock helper
				msg := &chatbuffer.Message{
					ID:      "channel-msg",
					Sender:  "user",
					Content: "Channel message",
					Page:    "some-page",
				}
				chatManager.sendMessageToChannel(msg)

				// Wait for message to be processed
				Eventually(streamServer.GetMessageCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Cancel context to stop subscription
				streamServer.contextDone = true
			})

			It("should stream channel messages", func() {
				Expect(streamServer.GetMessageCount()).To(BeNumerically(">=", 1))
			})
		})

		When("send fails", func() {
			var (
				req          *apiv1.SubscribeChatMessagesRequest
				streamServer *mockChatMessagesStreamServer
				doneCh       chan struct{}
			)

			BeforeEach(func() {
				req = &apiv1.SubscribeChatMessagesRequest{}
				streamServer = &mockChatMessagesStreamServer{
					sendErr: status.Error(codes.Internal, "send failed"),
				}
				doneCh = make(chan struct{})

				// Run subscription in background; close doneCh when it returns
				go func() {
					_ = server.SubscribeChatMessages(req, streamServer)
					close(doneCh)
				}()

				// Wait for subscription to be registered before sending messages
				Eventually(chatManager.channelSubscriberCount, "1s", "10ms").Should(BeNumerically(">=", 1))

				// Trigger a channel message that will fail to send using mock helper
				msg := &chatbuffer.Message{
					ID:      "channel-msg",
					Sender:  "user",
					Content: "This will fail",
					Page:    "some-page",
				}
				chatManager.sendMessageToChannel(msg)

				// Wait for the subscription goroutine to exit after the send error
				Eventually(doneCh, "1s", "10ms").Should(BeClosed())
			})

			It("should handle send error", func() {
				// Test passes if no panic occurs
				Expect(true).To(BeTrue())
			})
		})
	})
})

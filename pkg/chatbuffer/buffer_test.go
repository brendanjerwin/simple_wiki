//revive:disable:dot-imports
package chatbuffer_test

import (
	"context"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestChatBuffer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ChatBuffer Suite")
}

var _ = Describe("Manager", func() {
	var manager *chatbuffer.Manager

	BeforeEach(func() {
		manager = chatbuffer.NewManager()
		DeferCleanup(manager.Close)
	})

	Describe("NewManager", func() {
		It("should create a new manager", func() {
			Expect(manager).NotTo(BeNil())
		})
	})

	Describe("AddUserMessage", func() {
		When("no page subscribers or pool daemon are connected", func() {
			var (
				messageID string
				err       error
			)

			BeforeEach(func() {
				messageID, err = manager.AddUserMessage("test-page", "Hello", "user1")
			})

			It("should return ErrNoSubscribers", func() {
				Expect(err).To(MatchError(chatbuffer.ErrNoSubscribers))
			})

			It("should return empty message ID", func() {
				Expect(messageID).To(BeEmpty())
			})
		})

		When("page channel subscriber is connected", func() {
			var (
				msgChan   <-chan *chatbuffer.Message
				messageID string
				err       error
			)

			BeforeEach(func() {
				var unsubscribe func()
				msgChan, unsubscribe = manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsubscribe)

				messageID, err = manager.AddUserMessage("test-page", "Hello, world!", "alice")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a non-empty message ID", func() {
				Expect(messageID).NotTo(BeEmpty())
			})

			It("should notify page channel subscribers", func() {
				Eventually(msgChan).Should(Receive(And(
					HaveField("Content", "Hello, world!"),
					HaveField("Sender", "user"),
					HaveField("SenderName", "alice"),
					HaveField("Page", "test-page"),
				)))
			})
		})

		When("page subscriber is listening", func() {
			var (
				eventChan <-chan chatbuffer.Event
				messageID string
			)

			BeforeEach(func() {
				// Subscribe to page channel (required for user messages)
				_, unsubChan := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsubChan)

				var unsubPage func()
				eventChan, unsubPage = manager.SubscribeToPage("test-page")
				DeferCleanup(unsubPage)

				messageID, _ = manager.AddUserMessage("test-page", "Test message", "bob")
			})

			It("should notify page subscribers with new message event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeNewMessage),
					HaveField("Message.Content", "Test message"),
					HaveField("Message.ID", messageID),
				)))
			})
		})

		When("page channel subscriber buffer is full and cannot receive", func() {
			var addedCount int

			BeforeEach(func() {
				// Subscribe but never read, so the channel buffer fills up
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				// Add 101 messages — the channel buffer holds 100, so the 101st hits the default drop branch
				for i := 0; i < 101; i++ {
					_, _ = manager.AddUserMessage("test-page", "Message", "user")
				}

				addedCount = len(manager.GetMessages("test-page"))
			})

			It("should not deadlock or panic", func() {
				Expect(addedCount).To(Equal(101))
			})
		})

		When("adding more than MaxMessagesPerPage messages", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				// Add MaxMessagesPerPage + 10 messages
				for i := 0; i < chatbuffer.MaxMessagesPerPage+10; i++ {
					_, _ = manager.AddUserMessage("test-page", "Message", "user")
				}

				messages = manager.GetMessages("test-page")
			})

			It("should keep only MaxMessagesPerPage messages", func() {
				Expect(messages).To(HaveLen(chatbuffer.MaxMessagesPerPage))
			})

			It("should evict oldest messages first", func() {
				// First message should have sequence 11 (since we added 210 messages total)
				Expect(messages[0].Sequence).To(Equal(int64(11)))
			})
		})
	})

	Describe("AddAssistantMessage", func() {
		When("adding a simple assistant message", func() {
			var (
				messageID string
				err       error
			)

			BeforeEach(func() {
				messageID, err = manager.AddAssistantMessage("test-page", "Hi there!", "")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a message ID", func() {
				Expect(messageID).NotTo(BeEmpty())
			})

			It("should store the message", func() {
				messages := manager.GetMessages("test-page")
				Expect(messages).To(HaveLen(1))
				Expect(messages[0].Content).To(Equal("Hi there!"))
				Expect(messages[0].Sender).To(Equal("assistant"))
				Expect(messages[0].SenderName).To(BeEmpty())
			})
		})

		When("adding a message with reply_to", func() {
			var (
				replyToID string
				replyID   string
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				// Add initial user message
				replyToID, _ = manager.AddUserMessage("test-page", "Question?", "user")

				// Add assistant reply
				replyID, _ = manager.AddAssistantMessage("test-page", "Answer!", replyToID)
			})

			It("should link to parent message", func() {
				messages := manager.GetMessages("test-page")
				Expect(messages).To(HaveLen(2))
				Expect(messages[1].ReplyToID).To(Equal(replyToID))
			})

			It("should be retrievable by ID", func() {
				msg, err := manager.GetMessageByID(replyID)
				Expect(err).NotTo(HaveOccurred())
				Expect(msg.ReplyToID).To(Equal(replyToID))
			})
		})

		When("page subscriber is listening", func() {
			var eventChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				eventChan, unsub = manager.SubscribeToPage("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddAssistantMessage("test-page", "Response", "")
			})

			It("should notify page subscribers", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeNewMessage),
					HaveField("Message.Content", "Response"),
					HaveField("Message.Sender", "assistant"),
				)))
			})
		})
	})

	Describe("EditMessage", func() {
		When("editing an existing message", func() {
			var (
				messageID string
				eventChan <-chan chatbuffer.Event
				err       error
			)

			BeforeEach(func() {
				_, unsubChan := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsubChan)

				messageID, _ = manager.AddUserMessage("test-page", "Original", "user")

				var unsubPage func()
				eventChan, unsubPage = manager.SubscribeToPage("test-page")
				DeferCleanup(unsubPage)

				err = manager.EditMessage(messageID, "Edited content", false)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the message content", func() {
				messages := manager.GetMessages("test-page")
				Expect(messages).To(HaveLen(1))
				Expect(messages[0].Content).To(Equal("Edited content"))
			})

			It("should notify page subscribers with edit event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeEdit),
					HaveField("Edit.MessageID", messageID),
					HaveField("Edit.NewContent", "Edited content"),
				)))
			})
		})

		When("editing with streaming=true", func() {
			var (
				messageID string
				eventChan <-chan chatbuffer.Event
			)

			BeforeEach(func() {
				_, unsubChan := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsubChan)

				messageID, _ = manager.AddUserMessage("test-page", "Original", "user")

				var unsubPage func()
				eventChan, unsubPage = manager.SubscribeToPage("test-page")
				DeferCleanup(unsubPage)

				_ = manager.EditMessage(messageID, "Streaming update", true)
			})

			It("should include streaming=true in the edit event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeEdit),
					HaveField("Edit.Streaming", true),
					HaveField("Edit.NewContent", "Streaming update"),
				)))
			})
		})

		When("editing a non-existent message", func() {
			var err error

			BeforeEach(func() {
				err = manager.EditMessage("nonexistent-id", "New content", false)
			})

			It("should return ErrMessageNotFound", func() {
				Expect(err).To(MatchError(chatbuffer.ErrMessageNotFound))
			})
		})

		When("editing a message in a different page", func() {
			var (
				messageID string
				err       error
			)

			BeforeEach(func() {
				_, unsub1 := manager.SubscribeToPageChannel("page1")
				DeferCleanup(unsub1)

				messageID, _ = manager.AddUserMessage("page1", "Message", "user")
				_, _ = manager.AddAssistantMessage("page2", "Other", "")

				err = manager.EditMessage(messageID, "Updated", false)
			})

			It("should find and update the message", func() {
				Expect(err).NotTo(HaveOccurred())

				messages := manager.GetMessages("page1")
				Expect(messages[0].Content).To(Equal("Updated"))
			})
		})
	})

	Describe("AddReaction", func() {
		When("adding a reaction to an existing message", func() {
			var (
				messageID string
				eventChan <-chan chatbuffer.Event
				err       error
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("test-page", "Message", "user")

				var unsubPage func()
				eventChan, unsubPage = manager.SubscribeToPage("test-page")
				DeferCleanup(unsubPage)

				err = manager.AddReaction(messageID, "👍", "alice")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add the reaction to the message", func() {
				messages := manager.GetMessages("test-page")
				Expect(messages[0].Reactions).To(HaveLen(1))
				Expect(messages[0].Reactions[0].Emoji).To(Equal("👍"))
				Expect(messages[0].Reactions[0].Reactor).To(Equal("alice"))
			})

			It("should notify page subscribers with reaction event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeReaction),
					HaveField("Reaction.MessageID", messageID),
					HaveField("Reaction.Emoji", "👍"),
					HaveField("Reaction.Reactor", "alice"),
				)))
			})
		})

		When("adding a duplicate reaction", func() {
			var (
				messageID string
				messages  []*chatbuffer.Message
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("test-page", "Message", "user")

				// Add same reaction twice
				_ = manager.AddReaction(messageID, "👍", "alice")
				_ = manager.AddReaction(messageID, "👍", "alice")

				messages = manager.GetMessages("test-page")
			})

			It("should not add duplicate reaction", func() {
				Expect(messages[0].Reactions).To(HaveLen(1))
			})
		})

		When("adding different reactions from different reactors", func() {
			var (
				messageID string
				messages  []*chatbuffer.Message
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("test-page", "Message", "user")

				_ = manager.AddReaction(messageID, "👍", "alice")
				_ = manager.AddReaction(messageID, "👍", "bob")
				_ = manager.AddReaction(messageID, "❤️", "alice")

				messages = manager.GetMessages("test-page")
			})

			It("should add all distinct reactions", func() {
				Expect(messages[0].Reactions).To(HaveLen(3))
			})
		})

		When("reacting to a non-existent message", func() {
			var err error

			BeforeEach(func() {
				err = manager.AddReaction("nonexistent-id", "👍", "user")
			})

			It("should return ErrMessageNotFound", func() {
				Expect(err).To(MatchError(chatbuffer.ErrMessageNotFound))
			})
		})
	})

	Describe("GetMessages", func() {
		When("getting messages from an empty page", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				messages = manager.GetMessages("empty-page")
			})

			It("should return an empty slice", func() {
				Expect(messages).To(BeEmpty())
			})
		})

		When("getting messages with reactions", func() {
			var (
				messageID string
				messages  []*chatbuffer.Message
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("test-page", "Message", "user")
				_ = manager.AddReaction(messageID, "👍", "alice")

				messages = manager.GetMessages("test-page")
			})

			It("should return copies with reactions", func() {
				Expect(messages).To(HaveLen(1))
				Expect(messages[0].Reactions).To(HaveLen(1))
			})

			It("should not affect original when modifying copy", func() {
				// Modify the copy
				messages[0].Content = "Modified"
				messages[0].Reactions = append(messages[0].Reactions, chatbuffer.Reaction{
					Emoji:   "❤️",
					Reactor: "bob",
				})

				// Original should be unchanged
				original := manager.GetMessages("test-page")
				Expect(original[0].Content).To(Equal("Message"))
				Expect(original[0].Reactions).To(HaveLen(1))
			})
		})
	})

	Describe("GetMessageByID", func() {
		When("getting an existing message", func() {
			var (
				messageID string
				msg       *chatbuffer.Message
				err       error
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("test-page", "Find me", "user")
				msg, err = manager.GetMessageByID(messageID)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the message", func() {
				Expect(msg).NotTo(BeNil())
				Expect(msg.Content).To(Equal("Find me"))
				Expect(msg.ID).To(Equal(messageID))
			})
		})

		When("getting a non-existent message", func() {
			var (
				msg *chatbuffer.Message
				err error
			)

			BeforeEach(func() {
				msg, err = manager.GetMessageByID("nonexistent")
			})

			It("should return ErrMessageNotFound", func() {
				Expect(err).To(MatchError(ContainSubstring("message not found")))
			})

			It("should return nil message", func() {
				Expect(msg).To(BeNil())
			})
		})

		When("searching across multiple pages", func() {
			var (
				messageID string
				msg       *chatbuffer.Message
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("page2")
				DeferCleanup(unsub)

				_, _ = manager.AddAssistantMessage("page1", "Message 1", "")
				messageID, _ = manager.AddUserMessage("page2", "Message 2", "user")
				_, _ = manager.AddAssistantMessage("page3", "Message 3", "")

				msg, _ = manager.GetMessageByID(messageID)
			})

			It("should find the message in page2", func() {
				Expect(msg).NotTo(BeNil())
				Expect(msg.Page).To(Equal("page2"))
				Expect(msg.Content).To(Equal("Message 2"))
			})
		})
	})

	Describe("SubscribeToPage", func() {
		When("subscribing to a page", func() {
			var (
				eventChan  <-chan chatbuffer.Event
				unsubscribe func()
			)

			BeforeEach(func() {
				eventChan, unsubscribe = manager.SubscribeToPage("test-page")
			})

			It("should return a channel", func() {
				Expect(eventChan).NotTo(BeNil())
			})

			It("should return an unsubscribe function", func() {
				Expect(unsubscribe).NotTo(BeNil())
			})

			When("unsubscribing", func() {
				BeforeEach(func() {
					unsubscribe()
				})

				It("should close the channel", func() {
					Eventually(eventChan).Should(BeClosed())
				})
			})
		})

		When("multiple subscribers on same page", func() {
			var (
				chan1 <-chan chatbuffer.Event
				chan2 <-chan chatbuffer.Event
			)

			BeforeEach(func() {
				_, unsubPage := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsubPage)

				var unsub1, unsub2 func()
				chan1, unsub1 = manager.SubscribeToPage("test-page")
				chan2, unsub2 = manager.SubscribeToPage("test-page")
				DeferCleanup(unsub1)
				DeferCleanup(unsub2)

				_, _ = manager.AddUserMessage("test-page", "Broadcast", "user")
			})

			It("should notify all subscribers", func() {
				Eventually(chan1).Should(Receive(HaveField("Type", chatbuffer.EventTypeNewMessage)))
				Eventually(chan2).Should(Receive(HaveField("Type", chatbuffer.EventTypeNewMessage)))
			})
		})
	})

	Describe("Message sequencing", func() {
		When("adding multiple messages to same page", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("test-page", "First", "user")
				_, _ = manager.AddAssistantMessage("test-page", "Second", "")
				_, _ = manager.AddUserMessage("test-page", "Third", "user")

				messages = manager.GetMessages("test-page")
			})

			It("should assign monotonically increasing sequences", func() {
				Expect(messages).To(HaveLen(3))
				Expect(messages[0].Sequence).To(Equal(int64(1)))
				Expect(messages[1].Sequence).To(Equal(int64(2)))
				Expect(messages[2].Sequence).To(Equal(int64(3)))
			})
		})

		When("adding messages to different pages", func() {
			var page1Msgs, page2Msgs []*chatbuffer.Message

			BeforeEach(func() {
				// Use instance request subscriber to allow sending to any page
				_, unsub := manager.SubscribeToInstanceRequests()
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("page1", "A", "user")
				_, _ = manager.AddUserMessage("page2", "B", "user")
				_, _ = manager.AddUserMessage("page1", "C", "user")

				page1Msgs = manager.GetMessages("page1")
				page2Msgs = manager.GetMessages("page2")
			})

			It("should maintain independent sequences per page", func() {
				Expect(page1Msgs[0].Sequence).To(Equal(int64(1)))
				Expect(page1Msgs[1].Sequence).To(Equal(int64(2)))
				Expect(page2Msgs[0].Sequence).To(Equal(int64(1)))
			})
		})
	})

	Describe("Timestamp handling", func() {
		When("adding messages", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("test-page", "Message", "user")

				messages = manager.GetMessages("test-page")
			})

			It("should set timestamp to current time", func() {
				Expect(messages[0].Timestamp).To(BeTemporally("~", time.Now(), time.Second))
			})
		})
	})

	Describe("SubscribeToPageWithReplay", func() {
		When("existing messages are present", func() {
			var (
				replayMessages []*chatbuffer.Message
				eventChan      <-chan chatbuffer.Event
			)

			BeforeEach(func() {
				_, _ = manager.AddAssistantMessage("test-page", "First message", "")
				_, _ = manager.AddAssistantMessage("test-page", "Second message", "")

				var unsub func()
				replayMessages, eventChan, unsub = manager.SubscribeToPageWithReplay("test-page")
				DeferCleanup(unsub)
			})

			It("should return existing messages as replay", func() {
				Expect(replayMessages).To(HaveLen(2))
				Expect(replayMessages[0].Content).To(Equal("First message"))
				Expect(replayMessages[1].Content).To(Equal("Second message"))
			})

			It("should return a non-nil event channel", func() {
				Expect(eventChan).NotTo(BeNil())
			})
		})

		When("no existing messages are present", func() {
			var replayMessages []*chatbuffer.Message

			BeforeEach(func() {
				var unsub func()
				replayMessages, _, unsub = manager.SubscribeToPageWithReplay("test-page")
				DeferCleanup(unsub)
			})

			It("should return an empty replay slice", func() {
				Expect(replayMessages).To(BeEmpty())
			})
		})

		When("new events arrive after subscribing", func() {
			var eventChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				_, eventChan, unsub = manager.SubscribeToPageWithReplay("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddAssistantMessage("test-page", "New message", "")
			})

			It("should deliver new message events on the channel", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeNewMessage),
					HaveField("Message.Content", "New message"),
				)))
			})
		})

		When("unsubscribing", func() {
			var (
				eventChan <-chan chatbuffer.Event
				unsub     func()
			)

			BeforeEach(func() {
				_, eventChan, unsub = manager.SubscribeToPageWithReplay("test-page")
				unsub()
			})

			It("should close the event channel", func() {
				Eventually(eventChan).Should(BeClosed())
			})
		})
	})

	Describe("SubscribeToPageChannel", func() {
		When("subscribing to a page", func() {
			var (
				msgChan <-chan *chatbuffer.Message
				unsub   func()
			)

			BeforeEach(func() {
				msgChan, unsub = manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)
			})

			It("should return a non-nil channel", func() {
				Expect(msgChan).NotTo(BeNil())
			})

			It("should mark the page as having a subscriber", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeTrue())
			})

			It("should not affect other pages", func() {
				Expect(manager.HasPageChannelSubscriber("other-page")).To(BeFalse())
			})
		})

		When("a user message is sent to a subscribed page", func() {
			var msgChan <-chan *chatbuffer.Message

			BeforeEach(func() {
				var unsub func()
				msgChan, unsub = manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("test-page", "Hello", "user1")
			})

			It("should deliver the message to the page channel", func() {
				Eventually(msgChan).Should(Receive(HaveField("Content", "Hello")))
			})
		})

		When("a user message is sent to a different page", func() {
			var msgChan <-chan *chatbuffer.Message

			BeforeEach(func() {
				var unsub func()
				msgChan, unsub = manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				// Use instance request subscriber to allow sending to other-page
				_, poolUnsub := manager.SubscribeToInstanceRequests()
				DeferCleanup(poolUnsub)

				_, _ = manager.AddUserMessage("other-page", "Hello", "user1")
			})

			It("should not deliver the message", func() {
				Consistently(msgChan).ShouldNot(Receive())
			})
		})

		When("unsubscribing", func() {
			var (
				msgChan <-chan *chatbuffer.Message
				unsub   func()
			)

			BeforeEach(func() {
				msgChan, unsub = manager.SubscribeToPageChannel("test-page")
				unsub()
			})

			It("should close the channel", func() {
				Eventually(msgChan).Should(BeClosed())
			})

			It("should mark as not having a page channel subscriber", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeFalse())
			})
		})

		When("page channel is the only subscriber", func() {
			When("sending a user message", func() {
				var (
					messageID string
					err       error
				)

				BeforeEach(func() {
					_, unsub := manager.SubscribeToPageChannel("test-page")
					DeferCleanup(unsub)

					messageID, err = manager.AddUserMessage("test-page", "Hello", "user1")
				})

				It("should succeed without global subscribers", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return a message ID", func() {
					Expect(messageID).NotTo(BeEmpty())
				})
			})
		})
	})

	Describe("HasPageChannelSubscriber", func() {
		When("no subscribers", func() {
			It("should return false", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeFalse())
			})
		})

		When("subscriber exists for the page", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)
			})

			It("should return true", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeTrue())
			})
		})

		When("subscriber exists for a different page", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("other-page")
				DeferCleanup(unsub)
			})

			It("should return false for the unsubscribed page", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeFalse())
			})
		})
	})

	Describe("RequestInstance", func() {
		When("no subscribers or requests exist", func() {
			BeforeEach(func() {
				manager.RequestInstance("test-page")
			})

			It("should mark the page as requested", func() {
				Expect(manager.IsInstanceRequested("test-page")).To(BeTrue())
			})
		})

		When("a page channel subscriber already exists", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				manager.RequestInstance("test-page")
			})

			It("should not mark as requested", func() {
				Expect(manager.IsInstanceRequested("test-page")).To(BeFalse())
			})
		})

		When("the page was already requested recently", func() {
			var requestChan <-chan string

			BeforeEach(func() {
				var unsub func()
				requestChan, unsub = manager.SubscribeToInstanceRequests()
				DeferCleanup(unsub)

				manager.RequestInstance("test-page")
				Eventually(requestChan).Should(Receive())

				manager.RequestInstance("test-page")
			})

			It("should not emit a duplicate request", func() {
				Consistently(requestChan).ShouldNot(Receive())
			})
		})

		When("a pool daemon is subscribed to instance requests", func() {
			var requestChan <-chan string

			BeforeEach(func() {
				var unsub func()
				requestChan, unsub = manager.SubscribeToInstanceRequests()
				DeferCleanup(unsub)

				manager.RequestInstance("test-page")
			})

			It("should notify the pool daemon", func() {
				Eventually(requestChan).Should(Receive(Equal("test-page")))
			})
		})
	})

	Describe("SubscribeToInstanceRequests", func() {
		When("subscribing", func() {
			var (
				requestChan <-chan string
				unsub       func()
			)

			BeforeEach(func() {
				requestChan, unsub = manager.SubscribeToInstanceRequests()
				DeferCleanup(unsub)
			})

			It("should return a non-nil channel", func() {
				Expect(requestChan).NotTo(BeNil())
			})

			It("should mark as having instance request subscribers", func() {
				Expect(manager.HasInstanceRequestSubscribers()).To(BeTrue())
			})
		})

		When("unsubscribing", func() {
			var requestChan <-chan string

			BeforeEach(func() {
				var unsub func()
				requestChan, unsub = manager.SubscribeToInstanceRequests()
				unsub()
			})

			It("should close the channel", func() {
				Eventually(requestChan).Should(BeClosed())
			})

			It("should mark as not having instance request subscribers", func() {
				Expect(manager.HasInstanceRequestSubscribers()).To(BeFalse())
			})
		})
	})

	Describe("HasInstanceRequestSubscribers", func() {
		When("no subscribers", func() {
			It("should return false", func() {
				Expect(manager.HasInstanceRequestSubscribers()).To(BeFalse())
			})
		})
	})

	Describe("IsInstanceRequested", func() {
		When("no requests exist", func() {
			It("should return false", func() {
				Expect(manager.IsInstanceRequested("test-page")).To(BeFalse())
			})
		})

		When("a page channel subscriber exists for the page", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("test-page")
				DeferCleanup(unsub)

				manager.RequestInstance("test-page")
			})

			It("should return false even if recorded", func() {
				Expect(manager.IsInstanceRequested("test-page")).To(BeFalse())
			})
		})
	})

	Describe("NotifyToolCall", func() {
		When("a page subscriber is listening", func() {
			var eventChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				eventChan, unsub = manager.SubscribeToPage("test-page")
				DeferCleanup(unsub)

				manager.NotifyToolCall("test-page", "msg-1", "tc-1", "Read File", "running")
			})

			It("should emit a tool call event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypeToolCall),
					HaveField("ToolCall.MessageID", "msg-1"),
					HaveField("ToolCall.ToolCallID", "tc-1"),
					HaveField("ToolCall.Title", "Read File"),
					HaveField("ToolCall.Status", "running"),
				)))
			})
		})

		When("subscribers exist on different pages", func() {
			var (
				otherChan <-chan chatbuffer.Event
			)

			BeforeEach(func() {
				var unsub func()
				otherChan, unsub = manager.SubscribeToPage("other-page")
				DeferCleanup(unsub)

				manager.NotifyToolCall("test-page", "msg-1", "tc-1", "Read File", "running")
			})

			It("should not deliver to subscribers on other pages", func() {
				Consistently(otherChan).ShouldNot(Receive())
			})
		})

		When("no page subscribers exist", func() {
			It("should not panic", func() {
				Expect(func() {
					manager.NotifyToolCall("test-page", "msg-1", "tc-1", "Read File", "running")
				}).NotTo(Panic())
			})
		})

		When("checking stored messages after notification", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				manager.NotifyToolCall("test-page", "msg-1", "tc-1", "Read File", "running")
				messages = manager.GetMessages("test-page")
			})

			It("should not store tool call events in the buffer", func() {
				Expect(messages).To(BeEmpty())
			})
		})
	})

	Describe("RequestPermission", func() {
		When("a response is provided via RespondToPermission", func() {
			var selectedOption string

			BeforeEach(func() {
				done := make(chan struct{})

				go func() {
					defer close(done)
					selectedOption = manager.RequestPermission(
						context.Background(),
						"test-page",
						"req-1",
						"Allow Edit",
						"Do you want to allow editing?",
						[]chatbuffer.PermissionOption{
							{OptionID: "yes", Label: "Yes", Description: "Allow"},
							{OptionID: "no", Label: "No", Description: "Deny"},
						},
					)
				}()

				// Give the goroutine time to register the pending permission
				time.Sleep(50 * time.Millisecond)

				manager.RespondToPermission("req-1", "yes")

				Eventually(done, 2*time.Second).Should(BeClosed())
			})

			It("should return the selected option ID", func() {
				Expect(selectedOption).To(Equal("yes"))
			})
		})

		When("emitting the permission request to page subscribers", func() {
			var eventChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				eventChan, unsub = manager.SubscribeToPage("test-page")
				DeferCleanup(unsub)

				go func() {
					manager.RequestPermission(
						context.Background(),
						"test-page",
						"req-2",
						"Allow Edit",
						"Do you want to allow editing?",
						[]chatbuffer.PermissionOption{
							{OptionID: "yes", Label: "Yes", Description: "Allow"},
						},
					)
				}()
			})

			It("should deliver the permission request event", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypePermissionRequest),
					HaveField("PermissionRequest.RequestID", "req-2"),
					HaveField("PermissionRequest.Title", "Allow Edit"),
					HaveField("PermissionRequest.Description", "Do you want to allow editing?"),
				)))
			})
		})

		When("context is cancelled before a response arrives", func() {
			var selectedOption string

			BeforeEach(func() {
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})

				go func() {
					defer close(done)
					selectedOption = manager.RequestPermission(
						ctx,
						"test-page",
						"req-cancel-1",
						"Allow Edit",
						"Do you want to allow editing?",
						[]chatbuffer.PermissionOption{
							{OptionID: "yes", Label: "Yes", Description: "Allow"},
						},
					)
				}()

				// Give the goroutine time to register the pending permission
				time.Sleep(50 * time.Millisecond)
				cancel()

				Eventually(done, 2*time.Second).Should(BeClosed())
			})

			It("should return an empty string when context is cancelled", func() {
				Expect(selectedOption).To(BeEmpty())
			})
		})
	})

	Describe("EmitPermissionRequest", func() {
		When("page subscribers are listening", func() {
			var eventChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				eventChan, unsub = manager.SubscribeToPage("test-page")
				DeferCleanup(unsub)

				manager.EmitPermissionRequest("test-page", &chatbuffer.PermissionRequestEvent{
					Page:        "test-page",
					RequestID:   "req-3",
					Title:       "Confirm Action",
					Description: "Are you sure?",
					Options: []chatbuffer.PermissionOption{
						{OptionID: "confirm", Label: "Confirm", Description: "Proceed"},
					},
				})
			})

			It("should deliver the event to page subscribers", func() {
				Eventually(eventChan).Should(Receive(And(
					HaveField("Type", chatbuffer.EventTypePermissionRequest),
					HaveField("PermissionRequest.RequestID", "req-3"),
					HaveField("PermissionRequest.Title", "Confirm Action"),
				)))
			})
		})

		When("subscribers exist on a different page", func() {
			var otherChan <-chan chatbuffer.Event

			BeforeEach(func() {
				var unsub func()
				otherChan, unsub = manager.SubscribeToPage("other-page")
				DeferCleanup(unsub)

				manager.EmitPermissionRequest("test-page", &chatbuffer.PermissionRequestEvent{
					Page:      "test-page",
					RequestID: "req-4",
					Title:     "Confirm",
				})
			})

			It("should not deliver to other page subscribers", func() {
				Consistently(otherChan).ShouldNot(Receive())
			})
		})
	})

	Describe("RespondToPermission", func() {
		When("responding to a pending request", func() {
			var selectedOption string

			BeforeEach(func() {
				done := make(chan struct{})

				go func() {
					defer close(done)
					selectedOption = manager.RequestPermission(
						context.Background(),
						"test-page",
						"req-respond-1",
						"Title",
						"Description",
						[]chatbuffer.PermissionOption{
							{OptionID: "opt-a", Label: "A"},
							{OptionID: "opt-b", Label: "B"},
						},
					)
				}()

				time.Sleep(50 * time.Millisecond)

				manager.RespondToPermission("req-respond-1", "opt-b")

				Eventually(done, 2*time.Second).Should(BeClosed())
			})

			It("should unblock the request with the correct option", func() {
				Expect(selectedOption).To(Equal("opt-b"))
			})
		})

		When("responding to a non-existent request", func() {
			It("should not panic", func() {
				Expect(func() {
					manager.RespondToPermission("nonexistent-req", "some-option")
				}).NotTo(Panic())
			})
		})
	})

	Describe("GetPendingPermissionsForPage", func() {
		When("no permissions are pending", func() {
			var result []*chatbuffer.PermissionRequestEvent

			BeforeEach(func() {
				result = manager.GetPendingPermissionsForPage("test-page")
			})

			It("should return nil", func() {
				Expect(result).To(BeNil())
			})
		})

		When("a permission is pending for the page", func() {
			var result []*chatbuffer.PermissionRequestEvent

			BeforeEach(func() {
				done := make(chan struct{})

				go func() {
					defer close(done)
					manager.RequestPermission(
						context.Background(),
						"test-page",
						"req-pending-1",
						"Allow write?",
						"Agent wants to write files.",
						[]chatbuffer.PermissionOption{
							{OptionID: "allow", Label: "Allow"},
							{OptionID: "deny", Label: "Deny"},
						},
					)
				}()

				// Wait for the goroutine to register the pending permission
				Eventually(func() []*chatbuffer.PermissionRequestEvent {
					return manager.GetPendingPermissionsForPage("test-page")
				}, "1s", "10ms").Should(HaveLen(1))

				result = manager.GetPendingPermissionsForPage("test-page")

				manager.RespondToPermission("req-pending-1", "allow")
				Eventually(done, "2s").Should(BeClosed())
			})

			It("should return the pending permission event", func() {
				Expect(result).To(HaveLen(1))
			})

			It("should include the correct request ID", func() {
				Expect(result[0].RequestID).To(Equal("req-pending-1"))
			})

			It("should include the correct title", func() {
				Expect(result[0].Title).To(Equal("Allow write?"))
			})

			It("should include the correct options", func() {
				Expect(result[0].Options).To(HaveLen(2))
				Expect(result[0].Options[0].OptionID).To(Equal("allow"))
				Expect(result[0].Options[1].OptionID).To(Equal("deny"))
			})
		})

		When("a permission is pending for a different page", func() {
			var result []*chatbuffer.PermissionRequestEvent

			BeforeEach(func() {
				done := make(chan struct{})

				go func() {
					defer close(done)
					manager.RequestPermission(
						context.Background(),
						"other-page",
						"req-other-1",
						"Other permission",
						"Some description",
						nil,
					)
				}()

				// Wait for the goroutine to register
				Eventually(func() []*chatbuffer.PermissionRequestEvent {
					return manager.GetPendingPermissionsForPage("other-page")
				}, "1s", "10ms").Should(HaveLen(1))

				result = manager.GetPendingPermissionsForPage("test-page")

				manager.RespondToPermission("req-other-1", "")
				Eventually(done, "2s").Should(BeClosed())
			})

			It("should not return permissions for other pages", func() {
				Expect(result).To(BeNil())
			})
		})

		When("permission is no longer pending after response", func() {
			var result []*chatbuffer.PermissionRequestEvent

			BeforeEach(func() {
				done := make(chan struct{})

				go func() {
					defer close(done)
					manager.RequestPermission(
						context.Background(),
						"test-page",
						"req-completed-1",
						"Completed permission",
						"Some description",
						nil,
					)
				}()

				// Wait for registration then respond
				Eventually(func() []*chatbuffer.PermissionRequestEvent {
					return manager.GetPendingPermissionsForPage("test-page")
				}, "1s", "10ms").Should(HaveLen(1))

				manager.RespondToPermission("req-completed-1", "allow")
				Eventually(done, "2s").Should(BeClosed())

				result = manager.GetPendingPermissionsForPage("test-page")
			})

			It("should not return the completed permission", func() {
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("SubscribeToPageChannelWithReplay", func() {
		When("existing messages are present", func() {
			var (
				replayMessages []*chatbuffer.Message
				msgChan        <-chan *chatbuffer.Message
			)

			BeforeEach(func() {
				// Add messages before subscribing
				_, _ = manager.AddAssistantMessage("test-page", "First message", "")
				_, _ = manager.AddAssistantMessage("test-page", "Second message", "")

				var unsub func()
				replayMessages, msgChan, unsub = manager.SubscribeToPageChannelWithReplay("test-page")
				DeferCleanup(unsub)
			})

			It("should return existing messages as replay", func() {
				Expect(replayMessages).To(HaveLen(2))
				Expect(replayMessages[0].Content).To(Equal("First message"))
				Expect(replayMessages[1].Content).To(Equal("Second message"))
			})

			It("should return a non-nil message channel", func() {
				Expect(msgChan).NotTo(BeNil())
			})

			It("should mark the page as having a subscriber", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeTrue())
			})
		})

		When("no existing messages are present", func() {
			var replayMessages []*chatbuffer.Message

			BeforeEach(func() {
				var unsub func()
				replayMessages, _, unsub = manager.SubscribeToPageChannelWithReplay("test-page")
				DeferCleanup(unsub)
			})

			It("should return an empty replay slice", func() {
				Expect(replayMessages).To(BeEmpty())
			})
		})

		When("new messages arrive after subscribing", func() {
			var msgChan <-chan *chatbuffer.Message

			BeforeEach(func() {
				var unsub func()
				_, msgChan, unsub = manager.SubscribeToPageChannelWithReplay("test-page")
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("test-page", "New message", "user1")
			})

			It("should deliver new messages on the channel", func() {
				Eventually(msgChan).Should(Receive(HaveField("Content", "New message")))
			})
		})

		When("unsubscribing", func() {
			var (
				msgChan <-chan *chatbuffer.Message
				unsub   func()
			)

			BeforeEach(func() {
				_, msgChan, unsub = manager.SubscribeToPageChannelWithReplay("test-page")
				unsub()
			})

			It("should close the channel", func() {
				Eventually(msgChan).Should(BeClosed())
			})

			It("should mark as not having a page channel subscriber", func() {
				Expect(manager.HasPageChannelSubscriber("test-page")).To(BeFalse())
			})
		})
	})

	Describe("CancelPage", func() {
		When("a cancellation subscriber exists", func() {
			var (
				cancelChan <-chan struct{}
				result     bool
			)

			BeforeEach(func() {
				var unsub func()
				cancelChan, unsub = manager.SubscribeToCancellation("test-page")
				DeferCleanup(unsub)

				result = manager.CancelPage("test-page")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})

			It("should signal the cancellation channel", func() {
				Eventually(cancelChan).Should(Receive())
			})
		})

		When("no cancellation subscribers exist", func() {
			var result bool

			BeforeEach(func() {
				result = manager.CancelPage("test-page")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("cancellation subscribers exist on a different page", func() {
			var (
				otherChan <-chan struct{}
				result    bool
			)

			BeforeEach(func() {
				var unsub func()
				otherChan, unsub = manager.SubscribeToCancellation("other-page")
				DeferCleanup(unsub)

				result = manager.CancelPage("test-page")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})

			It("should not signal the other page's channel", func() {
				Consistently(otherChan).ShouldNot(Receive())
			})
		})

		When("multiple cancellation subscribers exist on same page", func() {
			var (
				cancelChan1 <-chan struct{}
				cancelChan2 <-chan struct{}
				result      bool
			)

			BeforeEach(func() {
				var unsub1, unsub2 func()
				cancelChan1, unsub1 = manager.SubscribeToCancellation("test-page")
				cancelChan2, unsub2 = manager.SubscribeToCancellation("test-page")
				DeferCleanup(unsub1)
				DeferCleanup(unsub2)

				result = manager.CancelPage("test-page")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})

			It("should signal all subscribers", func() {
				Eventually(cancelChan1).Should(Receive())
				Eventually(cancelChan2).Should(Receive())
			})
		})

		When("cancelling the same page twice", func() {
			var (
				firstResult  bool
				secondResult bool
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToCancellation("test-page")
				DeferCleanup(unsub)

				firstResult = manager.CancelPage("test-page")
				secondResult = manager.CancelPage("test-page")
			})

			It("should return true on first cancel", func() {
				Expect(firstResult).To(BeTrue())
			})

			It("should return false on second cancel", func() {
				Expect(secondResult).To(BeFalse())
			})
		})
	})

	Describe("SubscribeToCancellation", func() {
		When("subscribing to a page", func() {
			var (
				cancelChan <-chan struct{}
				unsub      func()
			)

			BeforeEach(func() {
				cancelChan, unsub = manager.SubscribeToCancellation("test-page")
				DeferCleanup(unsub)
			})

			It("should return a non-nil channel", func() {
				Expect(cancelChan).NotTo(BeNil())
			})

			It("should return a non-nil unsubscribe function", func() {
				Expect(unsub).NotTo(BeNil())
			})
		})

		When("CancelPage is called after subscribing", func() {
			var cancelChan <-chan struct{}

			BeforeEach(func() {
				var unsub func()
				cancelChan, unsub = manager.SubscribeToCancellation("test-page")
				DeferCleanup(unsub)

				manager.CancelPage("test-page")
			})

			It("should receive a cancellation signal", func() {
				Eventually(cancelChan).Should(Receive())
			})
		})

		When("unsubscribing before CancelPage", func() {
			var result bool

			BeforeEach(func() {
				_, unsub := manager.SubscribeToCancellation("test-page")
				unsub()

				result = manager.CancelPage("test-page")
			})

			It("should not find any subscribers to notify", func() {
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("Concurrent access", func() {
		When("multiple goroutines add messages concurrently", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToPageChannel("concurrent-page")
				DeferCleanup(unsub)

				done := make(chan bool)
				for i := 0; i < 10; i++ {
					go func(i int) {
						_, _ = manager.AddUserMessage("concurrent-page", "Message", "user")
						done <- true
					}(i)
				}

				// Wait for all goroutines
				for i := 0; i < 10; i++ {
					<-done
				}

				messages = manager.GetMessages("concurrent-page")
			})

			It("should safely store all messages", func() {
				Expect(messages).To(HaveLen(10))
			})

			It("should assign unique sequences", func() {
				sequences := make(map[int64]bool)
				for _, msg := range messages {
					sequences[msg.Sequence] = true
				}
				Expect(sequences).To(HaveLen(10))
			})
		})
	})
})

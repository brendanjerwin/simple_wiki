//revive:disable:dot-imports
package chatbuffer_test

import (
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

		It("should have no channel subscribers initially", func() {
			Expect(manager.HasChannelSubscribers()).To(BeFalse())
		})
	})

	Describe("AddUserMessage", func() {
		When("no channel subscribers are connected", func() {
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

		When("channel subscriber is connected", func() {
			var (
				msgChan   <-chan *chatbuffer.Message
				messageID string
				err       error
			)

			BeforeEach(func() {
				var unsubscribe func()
				msgChan, unsubscribe = manager.SubscribeToChannel()
				DeferCleanup(unsubscribe)

				messageID, err = manager.AddUserMessage("test-page", "Hello, world!", "alice")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a non-empty message ID", func() {
				Expect(messageID).NotTo(BeEmpty())
			})

			It("should notify channel subscribers", func() {
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
				// Subscribe to channel first (required for user messages)
				_, unsubChan := manager.SubscribeToChannel()
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

		When("adding more than MaxMessagesPerPage messages", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
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
				Expect(messages[0].SenderName).To(Equal("Claude"))
			})
		})

		When("adding a message with reply_to", func() {
			var (
				replyToID string
				replyID   string
			)

			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
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
				_, unsubChan := manager.SubscribeToChannel()
				DeferCleanup(unsubChan)

				messageID, _ = manager.AddUserMessage("test-page", "Original", "user")

				var unsubPage func()
				eventChan, unsubPage = manager.SubscribeToPage("test-page")
				DeferCleanup(unsubPage)

				err = manager.EditMessage(messageID, "Edited content")
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

		When("editing a non-existent message", func() {
			var err error

			BeforeEach(func() {
				err = manager.EditMessage("nonexistent-id", "New content")
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
				_, unsub := manager.SubscribeToChannel()
				DeferCleanup(unsub)

				messageID, _ = manager.AddUserMessage("page1", "Message", "user")
				_, _ = manager.AddAssistantMessage("page2", "Other", "")

				err = manager.EditMessage(messageID, "Updated")
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
				DeferCleanup(unsub)

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

	Describe("SubscribeToChannel", func() {
		When("subscribing to channel", func() {
			var (
				msgChan     <-chan *chatbuffer.Message
				unsubscribe func()
			)

			BeforeEach(func() {
				msgChan, unsubscribe = manager.SubscribeToChannel()
			})

			It("should return a channel", func() {
				Expect(msgChan).NotTo(BeNil())
			})

			It("should mark as having channel subscribers", func() {
				Expect(manager.HasChannelSubscribers()).To(BeTrue())
			})

			When("unsubscribing", func() {
				BeforeEach(func() {
					unsubscribe()
				})

				It("should close the channel", func() {
					Eventually(msgChan).Should(BeClosed())
				})

				It("should mark as not having channel subscribers", func() {
					Expect(manager.HasChannelSubscribers()).To(BeFalse())
				})
			})
		})

		When("multiple channel subscribers", func() {
			var (
				chan1 <-chan *chatbuffer.Message
				chan2 <-chan *chatbuffer.Message
			)

			BeforeEach(func() {
				var unsub1, unsub2 func()
				chan1, unsub1 = manager.SubscribeToChannel()
				chan2, unsub2 = manager.SubscribeToChannel()
				DeferCleanup(unsub1)
				DeferCleanup(unsub2)

				_, _ = manager.AddUserMessage("test-page", "Message", "user")
			})

			It("should notify all channel subscribers", func() {
				Eventually(chan1).Should(Receive(HaveField("Content", "Message")))
				Eventually(chan2).Should(Receive(HaveField("Content", "Message")))
			})
		})
	})

	Describe("HasChannelSubscribers", func() {
		When("no subscribers", func() {
			It("should return false", func() {
				Expect(manager.HasChannelSubscribers()).To(BeFalse())
			})
		})

		When("subscriber exists", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
				DeferCleanup(unsub)
			})

			It("should return true", func() {
				Expect(manager.HasChannelSubscribers()).To(BeTrue())
			})
		})

		When("subscriber unsubscribes", func() {
			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
				unsub()
			})

			It("should return false", func() {
				Expect(manager.HasChannelSubscribers()).To(BeFalse())
			})
		})
	})

	Describe("Message sequencing", func() {
		When("adding multiple messages to same page", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
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
				_, unsub := manager.SubscribeToChannel()
				DeferCleanup(unsub)

				_, _ = manager.AddUserMessage("test-page", "Message", "user")

				messages = manager.GetMessages("test-page")
			})

			It("should set timestamp to current time", func() {
				Expect(messages[0].Timestamp).To(BeTemporally("~", time.Now(), time.Second))
			})
		})
	})

	Describe("Concurrent access", func() {
		When("multiple goroutines add messages concurrently", func() {
			var messages []*chatbuffer.Message

			BeforeEach(func() {
				_, unsub := manager.SubscribeToChannel()
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

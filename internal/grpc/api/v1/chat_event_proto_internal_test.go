// Internal tests for bufferEventToProto and the event converters it dispatches
// to. These complement the external ChatService tests in chat_test.go.
package v1

import (
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("bufferEventToProto", func() {
	When("the event is a turn status update", func() {
		It("maps the page and active flag onto ChatTurnStatus", func() {
			ev := bufferEventToProto(chatbuffer.Event{
				Type:       chatbuffer.EventTypeTurnStatus,
				TurnStatus: &chatbuffer.TurnStatusEvent{Page: "recipes", Active: true},
			})
			Expect(ev).NotTo(BeNil())
			Expect(ev.GetTurnStatus()).NotTo(BeNil())
			Expect(ev.GetTurnStatus().GetPage()).To(Equal("recipes"))
			Expect(ev.GetTurnStatus().GetActive()).To(BeTrue())
		})

		It("preserves the inactive state", func() {
			ev := bufferEventToProto(chatbuffer.Event{
				Type:       chatbuffer.EventTypeTurnStatus,
				TurnStatus: &chatbuffer.TurnStatusEvent{Page: "", Active: false},
			})
			Expect(ev).NotTo(BeNil())
			Expect(ev.GetTurnStatus().GetPage()).To(BeEmpty())
			Expect(ev.GetTurnStatus().GetActive()).To(BeFalse())
		})
	})

	When("the event is a background task update", func() {
		It("maps every background task field", func() {
			ev := bufferEventToProto(chatbuffer.Event{
				Type: chatbuffer.EventTypeBackgroundTask,
				BackgroundTask: &chatbuffer.BackgroundTaskEvent{
					MessageID:   "msg-1",
					ToolCallID:  "call-1",
					Title:       "Scanning pages",
					Status:      "working",
					Detail:      "12 of 40",
					StartedAtMs: 1728000000000,
				},
			})
			Expect(ev).NotTo(BeNil())
			task := ev.GetBackgroundTask()
			Expect(task).NotTo(BeNil())
			Expect(task.GetMessageId()).To(Equal("msg-1"))
			Expect(task.GetToolCallId()).To(Equal("call-1"))
			Expect(task.GetTitle()).To(Equal("Scanning pages"))
			Expect(task.GetStatus()).To(Equal("working"))
			Expect(task.GetDetail()).To(Equal("12 of 40"))
			Expect(task.GetStartedAtMs()).To(Equal(int64(1728000000000)))
		})

		It("maps terminal statuses without alteration", func() {
			ev := bufferEventToProto(chatbuffer.Event{
				Type: chatbuffer.EventTypeBackgroundTask,
				BackgroundTask: &chatbuffer.BackgroundTaskEvent{
					MessageID: "msg-2",
					Status:    "completed",
				},
			})
			Expect(ev).NotTo(BeNil())
			Expect(ev.GetBackgroundTask().GetStatus()).To(Equal("completed"))
			Expect(ev.GetBackgroundTask().GetToolCallId()).To(BeEmpty())
			Expect(ev.GetBackgroundTask().GetStartedAtMs()).To(BeZero())
		})
	})
})

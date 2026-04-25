package main

import (
	"context"
	"sync/atomic"

	acp "github.com/coder/acp-go-sdk"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// NOTE: Phase 4B end-to-end coverage (full ephemeral-instance spawn + teardown
// verification) is NOT testable as a unit because it shells out to the real
// agent binary via exec.Cmd. End-to-end coverage of that path is provided by
// Phase 7 manual verification.

var _ = Describe("scheduledTurnClient SessionUpdate max_turns enforcement", func() {
	When("AgentMessageChunk notifications reach maxTurns", func() {
		var (
			client       *scheduledTurnClient
			cancelCalls  atomic.Int32
			recordCancel context.CancelFunc
		)

		BeforeEach(func() {
			cancelCalls.Store(0)
			recordCancel = func() {
				cancelCalls.Add(1)
			}
			client = newScheduledTurnClient("test-page", 3, recordCancel)

			chunkNotification := acp.SessionNotification{
				SessionId: "session-1",
				Update:    acp.UpdateAgentMessageText("hi"),
			}

			Expect(client.SessionUpdate(context.Background(), chunkNotification)).To(Succeed())
			Expect(client.SessionUpdate(context.Background(), chunkNotification)).To(Succeed())
			Expect(client.SessionUpdate(context.Background(), chunkNotification)).To(Succeed())
		})

		It("should call cancel exactly once after the third chunk", func() {
			Expect(cancelCalls.Load()).To(Equal(int32(1)))
		})

		It("should report HitLimit as true", func() {
			Expect(client.HitLimit()).To(BeTrue())
		})

		When("a fourth AgentMessageChunk notification arrives after the limit was hit", func() {
			BeforeEach(func() {
				chunkNotification := acp.SessionNotification{
					SessionId: "session-1",
					Update:    acp.UpdateAgentMessageText("again"),
				}
				Expect(client.SessionUpdate(context.Background(), chunkNotification)).To(Succeed())
			})

			It("should not call cancel again (first-trigger latching)", func() {
				Expect(cancelCalls.Load()).To(Equal(int32(1)))
			})

			It("should still report HitLimit as true", func() {
				Expect(client.HitLimit()).To(BeTrue())
			})
		})
	})
})

var _ = Describe("scheduledTurnClient SessionUpdate with non-message updates", func() {
	When("a ToolCall update is received before maxTurns is reached", func() {
		var (
			client       *scheduledTurnClient
			cancelCalls  atomic.Int32
			recordCancel context.CancelFunc
			updateErr    error
		)

		BeforeEach(func() {
			cancelCalls.Store(0)
			recordCancel = func() {
				cancelCalls.Add(1)
			}
			client = newScheduledTurnClient("test-page", 3, recordCancel)

			toolCallNotification := acp.SessionNotification{
				SessionId: "session-1",
				Update:    acp.StartToolCall("tc-1", "Read file"),
			}
			updateErr = client.SessionUpdate(context.Background(), toolCallNotification)
		})

		It("should not return an error", func() {
			Expect(updateErr).NotTo(HaveOccurred())
		})

		It("should not count toward the turn limit", func() {
			Expect(client.HitLimit()).To(BeFalse())
		})

		It("should not invoke cancel", func() {
			Expect(cancelCalls.Load()).To(Equal(int32(0)))
		})
	})
})

var _ = Describe("scheduledTurnClient RequestPermission", func() {
	When("called", func() {
		var (
			client *scheduledTurnClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			client = newScheduledTurnClient("test-page", 3, func() {})
			resp, err = client.RequestPermission(context.Background(), acp.RequestPermissionRequest{
				SessionId: "session-1",
			})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the cancelled-permission outcome", func() {
			Expect(resp).To(Equal(permissionCancelledResponse()))
		})

		It("should not return a Selected outcome", func() {
			Expect(resp.Outcome.Selected).To(BeNil())
		})

		It("should return a Cancelled outcome", func() {
			Expect(resp.Outcome.Cancelled).NotTo(BeNil())
		})
	})
})

var _ = Describe("scheduledTurnClient unsupported acp.Client methods", func() {
	var client *scheduledTurnClient

	BeforeEach(func() {
		client = newScheduledTurnClient("test-page", 3, func() {})
	})

	Describe("ReadTextFile", func() {
		var (
			resp acp.ReadTextFileResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.ReadTextFileResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning file system access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("file system access not available")))
		})
	})

	Describe("WriteTextFile", func() {
		var (
			resp acp.WriteTextFileResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.WriteTextFile(context.Background(), acp.WriteTextFileRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.WriteTextFileResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning file system access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("file system access not available")))
		})
	})

	Describe("CreateTerminal", func() {
		var (
			resp acp.CreateTerminalResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.CreateTerminalResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning terminal access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
		})
	})

	Describe("TerminalOutput", func() {
		var (
			resp acp.TerminalOutputResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.TerminalOutputResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning terminal access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
		})
	})

	Describe("ReleaseTerminal", func() {
		var (
			resp acp.ReleaseTerminalResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.ReleaseTerminalResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning terminal access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
		})
	})

	Describe("WaitForTerminalExit", func() {
		var (
			resp acp.WaitForTerminalExitResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.WaitForTerminalExit(context.Background(), acp.WaitForTerminalExitRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.WaitForTerminalExitResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning terminal access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
		})
	})

	Describe("KillTerminal", func() {
		var (
			resp acp.KillTerminalResponse
			err  error
		)

		BeforeEach(func() {
			resp, err = client.KillTerminal(context.Background(), acp.KillTerminalRequest{})
		})

		It("should return a zero-value response", func() {
			Expect(resp).To(Equal(acp.KillTerminalResponse{}))
		})

		It("should return a non-nil error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error mentioning terminal access is unavailable", func() {
			Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
		})
	})
})

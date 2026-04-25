package main

import (
	"context"
	"strings"
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

var _ = Describe("buildScheduledTurnAgentCmd", func() {
	When("useSystemd is true and request_id is longer than scheduledTurnRequestIDInUnit", func() {
		var (
			daemon    *poolDaemon
			cmd       interface{ String() string }
			args      []string
			page      string
			requestID string
		)

		BeforeEach(func() {
			page = "my-page"
			requestID = "abcdef0123456789-uuid"
			daemon = &poolDaemon{
				useSystemd: true,
				agentPath:  "/fake/path",
			}
			c := daemon.buildScheduledTurnAgentCmd(context.Background(), page, requestID)
			cmd = c
			args = c.Args
		})

		It("should not be nil", func() {
			Expect(cmd).NotTo(BeNil())
		})

		It("should invoke systemd-run as the program", func() {
			Expect(args[0]).To(Equal("systemd-run"))
		})

		It("should include --user flag", func() {
			Expect(args).To(ContainElement("--user"))
		})

		It("should include --scope flag", func() {
			Expect(args).To(ContainElement("--scope"))
		})

		It("should include a --unit= flag whose value starts with wiki-scheduled-", func() {
			found := false
			for _, a := range args {
				if strings.HasPrefix(a, "--unit=") {
					found = true
					Expect(strings.TrimPrefix(a, "--unit=")).To(HavePrefix(scheduledTurnUnitPrefix))
				}
			}
			Expect(found).To(BeTrue(), "expected a --unit= argument")
		})

		It("should embed the sanitized page in the unit name", func() {
			for _, a := range args {
				if strings.HasPrefix(a, "--unit=") {
					Expect(a).To(ContainSubstring(sanitizeUnitName(page)))
					return
				}
			}
			Fail("no --unit= argument present")
		})

		It("should embed only the first scheduledTurnRequestIDInUnit characters of the request_id", func() {
			truncated := requestID[:scheduledTurnRequestIDInUnit]
			for _, a := range args {
				if strings.HasPrefix(a, "--unit=") {
					unit := strings.TrimPrefix(a, "--unit=")
					Expect(unit).To(HaveSuffix("-" + truncated))
					// Make sure the full request_id is NOT in the unit name.
					Expect(unit).NotTo(ContainSubstring(requestID))
					return
				}
			}
			Fail("no --unit= argument present")
		})

		It("should include a --working-directory= argument (cwd pinning)", func() {
			found := false
			for _, a := range args {
				if strings.HasPrefix(a, "--working-directory=") {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected a --working-directory= argument")
		})

		It("should end with the agent path", func() {
			Expect(args[len(args)-1]).To(Equal("/fake/path"))
		})
	})

	When("useSystemd is false", func() {
		var (
			daemon *poolDaemon
			args   []string
		)

		BeforeEach(func() {
			daemon = &poolDaemon{
				useSystemd: false,
				agentPath:  "/fake/path",
			}
			c := daemon.buildScheduledTurnAgentCmd(context.Background(), "page", "requestid")
			args = c.Args
		})

		It("should not include systemd-run", func() {
			for _, a := range args {
				Expect(a).NotTo(Equal("systemd-run"))
			}
		})

		It("should invoke the agent path directly", func() {
			Expect(args[0]).To(Equal("/fake/path"))
		})

		It("should have exactly one argument (the agent path)", func() {
			Expect(args).To(HaveLen(1))
		})
	})

	When("useSystemd is true and request_id is shorter than scheduledTurnRequestIDInUnit", func() {
		var (
			daemon   *poolDaemon
			args     []string
			shortReq string
		)

		BeforeEach(func() {
			shortReq = "abc"
			daemon = &poolDaemon{
				useSystemd: true,
				agentPath:  "/fake/path",
			}
			c := daemon.buildScheduledTurnAgentCmd(context.Background(), "page", shortReq)
			args = c.Args
		})

		It("should not panic when slicing the short request_id", func() {
			// If we got here, the constructor did not panic.
			Expect(args).NotTo(BeEmpty())
		})

		It("should embed the full short request_id in the unit name", func() {
			for _, a := range args {
				if strings.HasPrefix(a, "--unit=") {
					Expect(a).To(HaveSuffix("-" + shortReq))
					return
				}
			}
			Fail("no --unit= argument present")
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

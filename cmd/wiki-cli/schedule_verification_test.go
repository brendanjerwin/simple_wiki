package main

import (
	"context"
	"errors"
	"io"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"
	cli "gopkg.in/urfave/cli.v1"
)

var _ = Describe("verifyScheduleFired", func() {
	var (
		schedule *apiv1.AgentSchedule
		since    time.Time
		err      error
	)

	BeforeEach(func() {
		since = time.Date(2026, 6, 11, 14, 0, 0, 0, time.UTC)
		schedule = &apiv1.AgentSchedule{
			Id:         "daily_light_check",
			LastRun:    timestamppb.New(since.Add(time.Minute)),
			LastStatus: apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
		}
	})

	When("the schedule ran successfully after the required timestamp", func() {
		BeforeEach(func() {
			err = verifyScheduleFired(schedule, since)
		})

		It("should pass", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the latest status is warn", func() {
		BeforeEach(func() {
			schedule.LastStatus = apiv1.ScheduleStatus_SCHEDULE_STATUS_WARN
			err = verifyScheduleFired(schedule, since)
		})

		It("should pass", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the schedule has never run", func() {
		BeforeEach(func() {
			schedule.LastRun = nil
			err = verifyScheduleFired(schedule, since)
		})

		It("should explain the missing fire", func() {
			Expect(err).To(MatchError(`schedule "daily_light_check" has never run`))
		})
	})

	When("the last run predates the required timestamp", func() {
		BeforeEach(func() {
			schedule.LastRun = timestamppb.New(since.Add(-time.Minute))
			err = verifyScheduleFired(schedule, since)
		})

		It("should explain the stale fire", func() {
			Expect(err).To(MatchError(`schedule "daily_light_check" last ran at 2026-06-11T13:59:00Z before required 2026-06-11T14:00:00Z`))
		})
	})

	When("the last status is still running", func() {
		BeforeEach(func() {
			schedule.LastStatus = apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING
			err = verifyScheduleFired(schedule, since)
		})

		It("should explain the in-flight state", func() {
			Expect(err).To(MatchError(`schedule "daily_light_check" is still running from 2026-06-11T14:01:00Z`))
		})
	})

	When("the last status is an error", func() {
		BeforeEach(func() {
			schedule.LastStatus = apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR
			schedule.LastErrorMessage = "pool unavailable"
			err = verifyScheduleFired(schedule, since)
		})

		It("should include the recorded error", func() {
			Expect(err).To(MatchError(`schedule "daily_light_check" last status is SCHEDULE_STATUS_ERROR: pool unavailable`))
		})
	})
})

var _ = Describe("runVerifyScheduleFired", func() {
	var (
		client *stubScheduleLister
		since  time.Time
		err    error
	)

	BeforeEach(func() {
		since = time.Date(2026, 6, 11, 14, 0, 0, 0, time.UTC)
		client = &stubScheduleLister{
			response: &apiv1.ListSchedulesResponse{
				Schedules: []*apiv1.AgentSchedule{
					{
						Id:         "daily_light_check",
						LastRun:    timestamppb.New(since),
						LastStatus: apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
					},
				},
			},
		}
	})

	When("the schedule exists", func() {
		BeforeEach(func() {
			err = runVerifyScheduleFired(context.Background(), client, "ai_assistant", "daily_light_check", since)
		})

		It("should query the requested page", func() {
			Expect(client.page).To(Equal("ai_assistant"))
		})

		It("should pass", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the schedule is missing", func() {
		BeforeEach(func() {
			err = runVerifyScheduleFired(context.Background(), client, "ai_assistant", "unknown", since)
		})

		It("should explain the missing schedule", func() {
			Expect(err).To(MatchError(`schedule "unknown" not found`))
		})
	})

	When("listing schedules fails", func() {
		BeforeEach(func() {
			client.err = errors.New("connect unavailable")
			err = runVerifyScheduleFired(context.Background(), client, "ai_assistant", "daily_light_check", since)
		})

		It("should wrap the list error", func() {
			Expect(err).To(MatchError(ContainSubstring("list schedules: connect unavailable")))
		})
	})
})

var _ = Describe("buildVerifyScheduleFiredCommand", func() {
	var err error

	When("page is missing", func() {
		BeforeEach(func() {
			err = runVerifyScheduleCommand("verify-schedule-fired", "--schedule", "daily", "--since", "2026-06-11T14:00:00Z")
		})

		It("should return a validation error", func() {
			Expect(err).To(MatchError("page is required"))
		})
	})

	When("schedule is missing", func() {
		BeforeEach(func() {
			err = runVerifyScheduleCommand("verify-schedule-fired", "--page", "ai_assistant", "--since", "2026-06-11T14:00:00Z")
		})

		It("should return a validation error", func() {
			Expect(err).To(MatchError("schedule is required"))
		})
	})

	When("since is missing", func() {
		BeforeEach(func() {
			err = runVerifyScheduleCommand("verify-schedule-fired", "--page", "ai_assistant", "--schedule", "daily")
		})

		It("should return a validation error", func() {
			Expect(err).To(MatchError("since is required"))
		})
	})

	When("since is not RFC3339", func() {
		BeforeEach(func() {
			err = runVerifyScheduleCommand("verify-schedule-fired", "--page", "ai_assistant", "--schedule", "daily", "--since", "yesterday")
		})

		It("should return a parse error", func() {
			Expect(err).To(MatchError(ContainSubstring("since must be an RFC3339 timestamp")))
		})
	})
})

type stubScheduleLister struct {
	page     string
	response *apiv1.ListSchedulesResponse
	err      error
}

func (s *stubScheduleLister) ListSchedules(_ context.Context, req *connect.Request[apiv1.ListSchedulesRequest]) (*connect.Response[apiv1.ListSchedulesResponse], error) {
	s.page = req.Msg.GetPage()
	if s.err != nil {
		return nil, s.err
	}
	return connect.NewResponse(s.response), nil
}

func runVerifyScheduleCommand(args ...string) error {
	app := cli.NewApp()
	app.Writer = io.Discard
	app.ErrWriter = io.Discard
	app.Commands = []cli.Command{
		buildVerifyScheduleFiredCommand(cli.StringFlag{
			Name:  "url, u",
			Value: "http://example.invalid",
		}),
	}
	return app.Run(append([]string{"wiki-cli"}, args...))
}

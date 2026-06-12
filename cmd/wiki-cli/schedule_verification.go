package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	cli "gopkg.in/urfave/cli.v1"
)

const scheduleVerificationTimeoutMs = 10000

type scheduleLister interface {
	ListSchedules(context.Context, *connect.Request[apiv1.ListSchedulesRequest]) (*connect.Response[apiv1.ListSchedulesResponse], error)
}

func buildVerifyScheduleFiredCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:      "verify-schedule-fired",
		Usage:     "Verify an agent schedule fired successfully after a deployment timestamp",
		ArgsUsage: "--page <page> --schedule <id> --since <RFC3339 timestamp>",
		Description: `Checks AgentMetadataService/ListSchedules for a schedule's wiki-managed
last_run and last_status fields. Intended for post-deploy verification after
scheduler migrations or schedule fixes.

Example:
  wiki-cli verify-schedule-fired --page ai_assistant --schedule daily_light_check --since 2026-06-11T14:00:00Z`,
		Flags: []cli.Flag{
			urlFlag,
			cli.StringFlag{
				Name:  "page",
				Usage: "wiki page containing agent.schedules",
			},
			cli.StringFlag{
				Name:  "schedule",
				Usage: "schedule id to verify",
			},
			cli.StringFlag{
				Name:  "since",
				Usage: "minimum acceptable last_run timestamp (RFC3339)",
			},
		},
		Action: func(c *cli.Context) error {
			page := c.String("page")
			scheduleID := c.String("schedule")
			sinceRaw := c.String("since")
			if page == "" {
				return errors.New("page is required")
			}
			if scheduleID == "" {
				return errors.New("schedule is required")
			}
			if sinceRaw == "" {
				return errors.New("since is required")
			}
			since, err := time.Parse(time.RFC3339, sinceRaw)
			if err != nil {
				return fmt.Errorf("since must be an RFC3339 timestamp: %w", err)
			}
			client := apiv1connect.NewAgentMetadataServiceClient(newAgentAwareHTTPClient(http.DefaultClient), c.String("url"))
			return runVerifyScheduleFired(context.Background(), client, page, scheduleID, since)
		},
	}
}

func runVerifyScheduleFired(ctx context.Context, client scheduleLister, page, scheduleID string, since time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, scheduleVerificationTimeoutMs*time.Millisecond)
	defer cancel()

	resp, err := client.ListSchedules(ctx, connect.NewRequest(&apiv1.ListSchedulesRequest{Page: page}))
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	schedule, err := findSchedule(resp.Msg.GetSchedules(), scheduleID)
	if err != nil {
		return err
	}
	if err := verifyScheduleFired(schedule, since); err != nil {
		return err
	}
	lastRun := schedule.GetLastRun().AsTime().UTC().Format(time.RFC3339)
	if _, err := fmt.Printf("schedule %q fired at %s with status %s\n", scheduleID, lastRun, schedule.GetLastStatus()); err != nil {
		return fmt.Errorf(writeErrTemplate, err)
	}
	return nil
}

func findSchedule(schedules []*apiv1.AgentSchedule, scheduleID string) (*apiv1.AgentSchedule, error) {
	for _, schedule := range schedules {
		if schedule.GetId() == scheduleID {
			return schedule, nil
		}
	}
	return nil, fmt.Errorf("schedule %q not found", scheduleID)
}

func verifyScheduleFired(schedule *apiv1.AgentSchedule, since time.Time) error {
	if schedule == nil {
		return errors.New("schedule is required")
	}
	lastRun := schedule.GetLastRun()
	if lastRun == nil {
		return fmt.Errorf("schedule %q has never run", schedule.GetId())
	}
	lastRunTime := lastRun.AsTime()
	if lastRunTime.Before(since) {
		return fmt.Errorf("schedule %q last ran at %s before required %s",
			schedule.GetId(),
			lastRunTime.UTC().Format(time.RFC3339),
			since.UTC().Format(time.RFC3339),
		)
	}
	switch schedule.GetLastStatus() {
	case apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, apiv1.ScheduleStatus_SCHEDULE_STATUS_WARN:
		return nil
	case apiv1.ScheduleStatus_SCHEDULE_STATUS_UNSPECIFIED:
		return fmt.Errorf("schedule %q has no terminal status", schedule.GetId())
	case apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING:
		return fmt.Errorf("schedule %q is still running from %s", schedule.GetId(), lastRunTime.UTC().Format(time.RFC3339))
	default:
		return fmt.Errorf("schedule %q last status is %s: %s",
			schedule.GetId(),
			schedule.GetLastStatus(),
			schedule.GetLastErrorMessage(),
		)
	}
}

package server

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// AgentNamespaceKey is the top-level frontmatter key under which all
// agent-managed page state lives. Generic Frontmatter writes that touch this
// key are rejected; mutations go through AgentMetadataService and the typed
// stores.
const AgentNamespaceKey = "agent"

const (
	agentSchedulesKey      = "schedules"
	errReadFrontmatterFmt  = "read frontmatter for %q: %w"
	errWriteFrontmatterFmt = "write frontmatter for %q: %w"
)

// ScheduleNotFoundError is returned by the store when a referenced
// schedule_id does not exist on the page.
type ScheduleNotFoundError struct {
	Page       string
	ScheduleID string
}

// Error implements error.
func (e *ScheduleNotFoundError) Error() string {
	return fmt.Sprintf("schedule %q not found on page %q", e.ScheduleID, e.Page)
}

// agentSchedulePagesStore is the subset of PageReaderMutator the store uses.
// Only frontmatter read/write is needed; markdown access is not.
type agentSchedulePagesStore interface {
	ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)
	WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error
}

// AgentScheduleStore owns reads and writes of the agent.schedules subtree on
// each page. All callers MUST use this store rather than mutating the
// frontmatter directly so that status transitions are validated by the
// schedule state machine.
type AgentScheduleStore struct {
	pages agentSchedulePagesStore
	mu    sync.Mutex
}

// NewAgentScheduleStore constructs a store on top of the given page accessor.
func NewAgentScheduleStore(pages agentSchedulePagesStore) *AgentScheduleStore {
	return &AgentScheduleStore{pages: pages}
}

// List returns the schedules persisted on the given page. A page with no
// agent.schedules returns an empty slice and no error.
func (s *AgentScheduleStore) List(page string) ([]*apiv1.AgentSchedule, error) {
	_, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return nil, fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	return decodeSchedules(fm)
}

// Upsert creates or replaces the schedule with the given id on the page.
// Wiki-managed status fields on the supplied AgentSchedule (last_run,
// last_status, last_error_message, last_duration_seconds) are preserved from
// the existing record (if any) and the caller's values for those fields are
// silently dropped — the only legal way to mutate them is TransitionStatus.
func (s *AgentScheduleStore) Upsert(page string, schedule *apiv1.AgentSchedule) error {
	if schedule == nil {
		return errors.New("schedule is required")
	}
	if schedule.GetId() == "" {
		return errors.New("schedule.id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	existing, err := decodeSchedules(fm)
	if err != nil {
		return err
	}

	// Strip wiki-managed fields from the input; carry them over from the
	// existing record if one exists.
	clean := cloneScheduleClient(schedule)
	for _, prior := range existing {
		if prior.GetId() == clean.GetId() {
			clean.LastRun = prior.GetLastRun()
			clean.LastStatus = prior.GetLastStatus()
			clean.LastErrorMessage = prior.GetLastErrorMessage()
			clean.LastDurationSeconds = prior.GetLastDurationSeconds()
			break
		}
	}

	merged := mergeSchedule(existing, clean)
	if err := writeSchedules(fm, merged); err != nil {
		return err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// Delete removes the schedule with the given id from the page. Idempotent: if
// no schedule with that id exists, returns nil.
func (s *AgentScheduleStore) Delete(page, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	existing, err := decodeSchedules(fm)
	if err != nil {
		return err
	}

	filtered := existing[:0]
	for _, sc := range existing {
		if sc.GetId() == scheduleID {
			continue
		}
		filtered = append(filtered, sc)
	}
	if len(filtered) == len(existing) {
		// Nothing to delete; do not rewrite the file.
		return nil
	}
	if err := writeSchedules(fm, filtered); err != nil {
		return err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// TransitionStatus validates the requested status transition against the
// schedule state machine and, if legal, records it on the page along with the
// supplied error message and duration. last_run is server-stamped on every
// transition so observers can detect liveness.
func (s *AgentScheduleStore) TransitionStatus(page, scheduleID string, to apiv1.ScheduleStatus, errMessage string, durationSeconds int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	existing, err := decodeSchedules(fm)
	if err != nil {
		return err
	}

	var target *apiv1.AgentSchedule
	for _, sc := range existing {
		if sc.GetId() == scheduleID {
			target = sc
			break
		}
	}
	if target == nil {
		return &ScheduleNotFoundError{Page: page, ScheduleID: scheduleID}
	}

	if err := ValidateScheduleTransition(target.GetLastStatus(), to); err != nil {
		return err
	}

	target.LastStatus = to
	target.LastRun = timestamppb.New(time.Now().UTC())
	target.LastErrorMessage = errMessage
	target.LastDurationSeconds = durationSeconds

	if err := writeSchedules(fm, existing); err != nil {
		return err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// decodeSchedules reads agent.schedules out of a frontmatter map and returns
// the parsed AgentSchedule list. Unknown fields are dropped.
func decodeSchedules(fm wikipage.FrontMatter) ([]*apiv1.AgentSchedule, error) {
	agent, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		return nil, nil
	}
	rawSchedules, ok := agent[agentSchedulesKey]
	if !ok {
		return nil, nil
	}
	list, ok := rawSchedules.([]any)
	if !ok {
		return nil, fmt.Errorf("agent.schedules has unexpected type %T", rawSchedules)
	}

	out := make([]*apiv1.AgentSchedule, 0, len(list))
	for i, item := range list {
		raw, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("agent.schedules[%d] has unexpected type %T", i, item)
		}
		sc, err := scheduleFromMap(raw)
		if err != nil {
			return nil, fmt.Errorf("agent.schedules[%d]: %w", i, err)
		}
		out = append(out, sc)
	}
	return out, nil
}

// writeSchedules replaces agent.schedules in fm with the supplied schedule
// list. The agent.* subtree is created if missing; other top-level keys are
// untouched.
func writeSchedules(fm wikipage.FrontMatter, schedules []*apiv1.AgentSchedule) error {
	encoded := make([]any, 0, len(schedules))
	for _, sc := range schedules {
		raw, err := scheduleToMap(sc)
		if err != nil {
			return fmt.Errorf("encode schedule %q: %w", sc.GetId(), err)
		}
		encoded = append(encoded, raw)
	}

	agent, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		agent = map[string]any{}
	}
	if len(encoded) == 0 {
		delete(agent, agentSchedulesKey)
	} else {
		agent[agentSchedulesKey] = encoded
	}
	if len(agent) == 0 {
		delete(fm, AgentNamespaceKey)
		return nil
	}
	fm[AgentNamespaceKey] = agent
	return nil
}

// mergeSchedule appends or replaces a schedule by id, preserving order.
func mergeSchedule(existing []*apiv1.AgentSchedule, sc *apiv1.AgentSchedule) []*apiv1.AgentSchedule {
	for i, prior := range existing {
		if prior.GetId() == sc.GetId() {
			existing[i] = sc
			return existing
		}
	}
	return append(existing, sc)
}

// cloneScheduleClient returns a copy of sc with the wiki-managed fields cleared.
func cloneScheduleClient(sc *apiv1.AgentSchedule) *apiv1.AgentSchedule {
	return &apiv1.AgentSchedule{
		Id:       sc.GetId(),
		Cron:     sc.GetCron(),
		Prompt:   sc.GetPrompt(),
		MaxTurns: sc.GetMaxTurns(),
		Enabled:  sc.GetEnabled(),
	}
}

// scheduleToMap converts a typed AgentSchedule into the generic map[string]any
// representation used by the on-disk frontmatter.
func scheduleToMap(sc *apiv1.AgentSchedule) (map[string]any, error) {
	bytes, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}.Marshal(sc)
	if err != nil {
		return nil, err
	}
	return jsonBytesToMap(bytes)
}

// scheduleFromMap converts a generic map[string]any back into a typed
// AgentSchedule. Used when reading from the frontmatter.
func scheduleFromMap(raw map[string]any) (*apiv1.AgentSchedule, error) {
	bytes, err := mapToJSONBytes(raw)
	if err != nil {
		return nil, err
	}
	out := &apiv1.AgentSchedule{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(bytes, out); err != nil {
		return nil, err
	}
	return out, nil
}

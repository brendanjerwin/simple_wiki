// Package surveymutator owns mutations to wiki.surveys.<name> frontmatter data.
package surveymutator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Clock returns the current time. Production uses SystemClock; tests inject a
// deterministic clock.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the current wall-clock time.
type SystemClock struct{}

// Now returns time.Now.
func (SystemClock) Now() time.Time { return time.Now() }

// Mutator is the single funnel for survey configuration and response writes.
type Mutator struct {
	pages wikipage.PageReaderMutator
	clock Clock

	pageMu sync.Map
}

// Errors returned by the survey mutator.
var (
	ErrSurveyNotFound   = errors.New("survey not found")
	ErrFieldNotFound    = errors.New("survey field not found")
	ErrFieldExists      = errors.New("survey field already exists")
	ErrResponseNotFound = errors.New("survey response not found")
	ErrPageNotFound     = errors.New("page not found")
)

// New constructs a survey mutator.
func New(pages wikipage.PageReaderMutator, clock Clock) *Mutator {
	return &Mutator{pages: pages, clock: clock}
}

// GetSurvey reads the named survey.
func (m *Mutator) GetSurvey(_ context.Context, page, name string) (*apiv1.Survey, error) {
	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}
	survey := decodeSurvey(fm, name)
	if !surveyExists(fm, name) {
		return nil, ErrSurveyNotFound
	}
	return survey, nil
}

// UpsertSurvey creates or updates survey shell fields without touching responses.
func (m *Mutator) UpsertSurvey(ctx context.Context, page, name, question string, closed *bool, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurveyShell(ctx, page, name, expected, func(survey *apiv1.Survey) error {
		survey.Question = question
		if closed != nil {
			survey.Closed = *closed
		}
		return nil
	})
}

// AddField appends a new field.
func (m *Mutator) AddField(ctx context.Context, page, surveyName string, field *apiv1.SurveyField, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurvey(ctx, page, surveyName, expected, func(survey *apiv1.Survey) error {
		if field == nil || strings.TrimSpace(field.GetName()) == "" {
			return status.Error(codes.InvalidArgument, "field.name is required")
		}
		if findFieldIndex(survey, field.GetName()) >= 0 {
			return ErrFieldExists
		}
		survey.Fields = append(survey.Fields, cloneField(field))
		return nil
	})
}

// UpdateField replaces one field definition without touching responses.
func (m *Mutator) UpdateField(ctx context.Context, page, surveyName, fieldName string, field *apiv1.SurveyField, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurvey(ctx, page, surveyName, expected, func(survey *apiv1.Survey) error {
		idx := findFieldIndex(survey, fieldName)
		if idx < 0 {
			return ErrFieldNotFound
		}
		if field == nil || strings.TrimSpace(field.GetName()) == "" {
			return status.Error(codes.InvalidArgument, "field.name is required")
		}
		if otherIdx := findFieldIndex(survey, field.GetName()); otherIdx >= 0 && otherIdx != idx {
			return ErrFieldExists
		}
		survey.Fields[idx] = cloneField(field)
		return nil
	})
}

// RemoveField removes one field definition without touching responses.
func (m *Mutator) RemoveField(ctx context.Context, page, surveyName, fieldName string, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurvey(ctx, page, surveyName, expected, func(survey *apiv1.Survey) error {
		idx := findFieldIndex(survey, fieldName)
		if idx < 0 {
			return ErrFieldNotFound
		}
		survey.Fields = append(survey.Fields[:idx], survey.Fields[idx+1:]...)
		return nil
	})
}

// ReorderField moves one field to newIndex.
func (m *Mutator) ReorderField(ctx context.Context, page, surveyName, fieldName string, newIndex int, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurvey(ctx, page, surveyName, expected, func(survey *apiv1.Survey) error {
		if newIndex < 0 || newIndex >= len(survey.Fields) {
			return status.Error(codes.InvalidArgument, "new_index is out of range")
		}
		idx := findFieldIndex(survey, fieldName)
		if idx < 0 {
			return ErrFieldNotFound
		}
		field := survey.Fields[idx]
		survey.Fields = append(survey.Fields[:idx], survey.Fields[idx+1:]...)
		if newIndex >= len(survey.Fields) {
			survey.Fields = append(survey.Fields, field)
			return nil
		}
		survey.Fields = append(survey.Fields[:newIndex], append([]*apiv1.SurveyField{field}, survey.Fields[newIndex:]...)...)
		return nil
	})
}

// SubmitResponse upserts the caller's response using server-side identity and
// time. It never accepts user/submitted_at from the client.
func (m *Mutator) SubmitResponse(ctx context.Context, page, surveyName string, values *structpb.Struct, anonymous bool, identity tailscale.IdentityValue) (*apiv1.Survey, error) {
	user := identity.Name()
	if user == "" {
		return nil, status.Error(codes.PermissionDenied, "authenticated user is required")
	}
	return m.mutateSurvey(ctx, page, surveyName, nil, func(survey *apiv1.Survey) error {
		now := timestamppb.New(m.clock.Now())
		response := &apiv1.SurveyResponse{
			User:        user,
			Anonymous:   anonymous,
			SubmittedAt: now,
			Values:      values,
		}
		if response.Values == nil {
			response.Values = &structpb.Struct{Fields: map[string]*structpb.Value{}}
		}
		idx := findResponseIndex(survey, user)
		if idx >= 0 {
			survey.Responses[idx] = response
			return nil
		}
		survey.Responses = append(survey.Responses, response)
		return nil
	})
}

// ListResponses returns all responses for a survey.
func (m *Mutator) ListResponses(ctx context.Context, page, surveyName string) ([]*apiv1.SurveyResponse, error) {
	survey, err := m.GetSurvey(ctx, page, surveyName)
	if err != nil {
		return nil, err
	}
	return append([]*apiv1.SurveyResponse(nil), survey.Responses...), nil
}

// DeleteResponse removes a response by user, or by submittedAt when provided.
func (m *Mutator) DeleteResponse(ctx context.Context, page, surveyName, user string, submittedAt *time.Time, expected *time.Time) (*apiv1.Survey, error) {
	return m.mutateSurvey(ctx, page, surveyName, expected, func(survey *apiv1.Survey) error {
		idx := findResponseToDelete(survey, user, submittedAt)
		if idx < 0 {
			return ErrResponseNotFound
		}
		survey.Responses = append(survey.Responses[:idx], survey.Responses[idx+1:]...)
		return nil
	})
}

func (m *Mutator) mutateSurvey(ctx context.Context, page, name string, expected *time.Time, mutate func(*apiv1.Survey) error) (*apiv1.Survey, error) {
	return m.mutateSurveyData(ctx, page, name, expected, mutate, requireExistingSurvey)
}

func (m *Mutator) mutateSurveyShell(ctx context.Context, page, name string, expected *time.Time, mutate func(*apiv1.Survey) error) (*apiv1.Survey, error) {
	return m.mutateSurveyData(ctx, page, name, expected, mutate, allowMissingSurvey)
}

type missingSurveyPolicy int

const (
	requireExistingSurvey missingSurveyPolicy = iota
	allowMissingSurvey
)

func (m *Mutator) mutateSurveyData(ctx context.Context, page, name string, expected *time.Time, mutate func(*apiv1.Survey) error, missingPolicy missingSurveyPolicy) (*apiv1.Survey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	release := m.lockPage(page)
	defer release()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}
	if !surveyExists(fm, name) && missingPolicy == requireExistingSurvey {
		return nil, ErrSurveyNotFound
	}
	survey := decodeSurvey(fm, name)
	if err := checkExpectedUpdatedAt(survey, expected); err != nil {
		return nil, err
	}
	if err := mutate(survey); err != nil {
		return nil, err
	}
	survey.UpdatedAt = timestamppb.New(m.clock.Now())
	encodeSurvey(fm, name, survey)
	if err := m.pages.WriteFrontMatter(wikipage.PageIdentifier(page), fm); err != nil {
		return nil, fmt.Errorf("write frontmatter: %w", err)
	}
	return survey, nil
}

func (m *Mutator) readFrontMatter(page string) (wikipage.FrontMatter, error) {
	_, fm, err := m.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("read frontmatter: %w", err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	return fm, nil
}

func (m *Mutator) lockPage(page string) func() {
	v, _ := m.pageMu.LoadOrStore(page, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
	}
	mu.Lock()
	return mu.Unlock
}

func checkExpectedUpdatedAt(survey *apiv1.Survey, expected *time.Time) error {
	if expected == nil {
		return nil
	}
	if survey.GetUpdatedAt() == nil {
		return status.Error(codes.FailedPrecondition, "expected_updated_at mismatch: survey has no recorded updated_at")
	}
	if !survey.GetUpdatedAt().AsTime().Equal(*expected) {
		return status.Error(codes.FailedPrecondition, "expected_updated_at mismatch")
	}
	return nil
}

func findFieldIndex(survey *apiv1.Survey, fieldName string) int {
	for i, field := range survey.GetFields() {
		if field.GetName() == fieldName {
			return i
		}
	}
	return -1
}

func findResponseIndex(survey *apiv1.Survey, user string) int {
	for i, response := range survey.GetResponses() {
		if response.GetUser() == user {
			return i
		}
	}
	return -1
}

func findResponseToDelete(survey *apiv1.Survey, user string, submittedAt *time.Time) int {
	for i, response := range survey.GetResponses() {
		if user != "" && response.GetUser() != user {
			continue
		}
		if submittedAt != nil {
			if response.GetSubmittedAt() == nil || !response.GetSubmittedAt().AsTime().Equal(*submittedAt) {
				continue
			}
		}
		return i
	}
	return -1
}

func cloneField(field *apiv1.SurveyField) *apiv1.SurveyField {
	if field == nil {
		return nil
	}
	return &apiv1.SurveyField{
		Name:     strings.TrimSpace(field.GetName()),
		Type:     strings.TrimSpace(field.GetType()),
		Label:    field.Label,
		Required: field.Required,
		Options:  append([]string(nil), field.GetOptions()...),
		Min:      field.Min,
		Max:      field.Max,
	}
}

func sortResponses(responses []*apiv1.SurveyResponse) {
	sort.SliceStable(responses, func(i, j int) bool {
		left := responses[i]
		right := responses[j]
		if left.GetSubmittedAt() == nil || right.GetSubmittedAt() == nil {
			return left.GetUser() < right.GetUser()
		}
		return left.GetSubmittedAt().AsTime().Before(right.GetSubmittedAt().AsTime())
	})
}

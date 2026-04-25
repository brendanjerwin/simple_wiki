// Package checklistmutator owns every mutation to checklist data and the
// wiki-managed metadata that shadows it under wiki.checklists.*.
//
// Per ADR-0009 and ADR-0010, checklist mutations must funnel through this
// package so that per-item created_at/updated_at, completed_at/completed_by,
// per-list sync_token, and tombstones advance correctly. ChecklistService
// gRPC handlers, web UI form handlers, future CalDAV PUT/DELETE handlers,
// and the migration job are all expected callers.
//
// The package never accepts wiki-managed fields from input — they are
// derived from the caller's tailscale.IdentityValue and the injected
// Clock + ULID generator. Concurrent mutations are serialized through a
// per-page mutex.
package checklistmutator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Clock returns the current time. Production uses time.Now via SystemClock;
// tests inject a deterministic clock for predictable timestamps.
type Clock interface {
	Now() time.Time
}

// SystemClock returns time.Now wrapped in a Clock.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// TombstoneTTL is the minimum lifetime of a deletion tombstone before it
// becomes eligible for lazy GC. Per ADR-0009, GC runs on the next
// mutation/read of the affected list — so the actual lifetime is "at least
// 7 days, in practice possibly more depending on activity."
const TombstoneTTL = 7 * 24 * time.Hour

// SortOrderStep is the conventional spacing between adjacent items'
// sort_order values when AddItem appends to the end of a list. Sparse
// integer ordering lets ReorderItem bisect new positions without
// re-densifying the entire list.
const SortOrderStep int64 = 1000

// errUIDRequiredMsg is the InvalidArgument message returned when a
// mutator method is called with an empty uid. Pulled out so the
// repeated literal doesn't trip the "string literal appears N times"
// linter and so the wording stays consistent.
const errUIDRequiredMsg = "uid is required"

// Mutator is the single funnel for checklist mutations. Construct one per
// process via New; share across handlers.
type Mutator struct {
	pages wikipage.PageReaderMutator
	clock Clock
	ulids ulid.Generator

	mu     sync.Mutex
	pageMu map[string]*sync.Mutex
}

// New constructs a Mutator with the given dependencies.
//
// pages: the page-store backing the wiki (typically server.Site).
// clock: SystemClock in production; a fake in tests.
// ulids: ulid.NewSystemGenerator() in production; a SequenceGenerator/
// FixedGenerator in tests.
func New(pages wikipage.PageReaderMutator, clock Clock, ulids ulid.Generator) *Mutator {
	return &Mutator{
		pages:  pages,
		clock:  clock,
		ulids:  ulids,
		pageMu: make(map[string]*sync.Mutex),
	}
}

// AddItemArgs carries the user-mutable fields a caller may set when
// creating an item. Fields beyond Text are optional.
type AddItemArgs struct {
	Text         string
	Tags         []string
	Description  *string
	Due          *time.Time
	SortOrder    *int64
	AlarmPayload *string
}

// UpdateItemArgs carries the user-mutable fields a caller may set when
// updating an item. Nil-pointer fields mean "leave unchanged"; an empty-
// string pointer means "clear" only for the optional fields.
type UpdateItemArgs struct {
	Text         *string
	Tags         []string // when non-nil, replaces; nil means "leave unchanged"
	TagsSet      bool     // explicit "tags were provided" flag, since nil and empty-slice are distinguishable
	Description  *string
	DescriptionSet bool
	Due          *time.Time
	DueSet       bool
	AlarmPayload *string
	AlarmPayloadSet bool
}

// Errors returned by the mutator. RPC handlers map these to gRPC codes.
var (
	// ErrItemNotFound is returned when the requested uid does not exist on
	// the named checklist.
	ErrItemNotFound = errors.New("checklist item not found")
	// ErrListNotFound is returned when the requested checklist does not
	// exist on the page.
	ErrListNotFound = errors.New("checklist not found")
	// ErrPageNotFound is returned when the requested page does not exist.
	ErrPageNotFound = errors.New("page not found")
)

// AddItem appends a new item to the named checklist on page. The wiki
// generates the uid and stamps created_at = updated_at = clock.Now().
// automated is derived from identity.IsAgent(); completed_by is left
// unset (the new item is not checked yet).
func (m *Mutator) AddItem(_ context.Context, page, listName string, args AddItemArgs, identity tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	if listName == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "list_name is required")
	}
	if args.Text == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "text is required")
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)

	now := m.clock.Now()
	uid := m.ulids.NewULID()
	sortOrder := nextSortOrder(checklist.Items)
	if args.SortOrder != nil {
		sortOrder = *args.SortOrder
	}

	item := &apiv1.ChecklistItem{
		Uid:          uid,
		Text:         args.Text,
		Tags:         append([]string(nil), args.Tags...),
		SortOrder:    sortOrder,
		Description:  args.Description,
		AlarmPayload: args.AlarmPayload,
		CreatedAt:    timestamppb.New(now),
		UpdatedAt:    timestamppb.New(now),
		Automated:    identity.IsAgent(),
	}
	if args.Due != nil {
		item.Due = timestamppb.New(*args.Due)
	}

	checklist.Items = append(checklist.Items, item)
	sortItems(checklist.Items)
	bumpSyncToken(checklist, now)
	pruneTombstones(checklist, now)

	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, nil, err
	}
	return item, checklist, nil
}

// UpdateItem mutates user-mutable fields of an existing item. Wiki-managed
// fields on the request are ignored; updated_at is server-stamped.
func (m *Mutator) UpdateItem(_ context.Context, page, listName, uid string, args UpdateItemArgs, expectedUpdatedAt *time.Time, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, errUIDRequiredMsg)
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)
	if err := checkExpectedUpdatedAt(checklist, expectedUpdatedAt); err != nil {
		return nil, nil, err
	}

	idx, item := findItem(checklist, uid)
	if item == nil {
		return nil, nil, ErrItemNotFound
	}

	changed := false
	if args.Text != nil && *args.Text != item.Text {
		item.Text = *args.Text
		changed = true
	}
	if args.TagsSet && !slicesEqual(args.Tags, item.Tags) {
		item.Tags = append([]string(nil), args.Tags...)
		changed = true
	}
	if args.DescriptionSet {
		if !stringPtrEqual(args.Description, item.Description) {
			item.Description = args.Description
			changed = true
		}
	}
	if args.DueSet {
		if !timeAndTimestampEqual(args.Due, item.Due) {
			if args.Due == nil {
				item.Due = nil
			} else {
				item.Due = timestamppb.New(*args.Due)
			}
			changed = true
		}
	}
	if args.AlarmPayloadSet {
		if !stringPtrEqual(args.AlarmPayload, item.AlarmPayload) {
			item.AlarmPayload = args.AlarmPayload
			changed = true
		}
	}

	now := m.clock.Now()
	if changed {
		item.UpdatedAt = timestamppb.New(now)
		bumpSyncToken(checklist, now)
	}
	pruneTombstones(checklist, now)
	checklist.Items[idx] = item

	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, nil, err
	}
	return item, checklist, nil
}

// ToggleItem flips the checked field. False→true sets completed_at and
// completed_by; true→false clears both.
func (m *Mutator) ToggleItem(_ context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, identity tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, errUIDRequiredMsg)
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)
	if err := checkExpectedUpdatedAt(checklist, expectedUpdatedAt); err != nil {
		return nil, nil, err
	}

	idx, item := findItem(checklist, uid)
	if item == nil {
		return nil, nil, ErrItemNotFound
	}

	now := m.clock.Now()
	item.Checked = !item.Checked
	if item.Checked {
		item.CompletedAt = timestamppb.New(now)
		name := identity.Name()
		item.CompletedBy = &name
	} else {
		item.CompletedAt = nil
		item.CompletedBy = nil
	}
	item.UpdatedAt = timestamppb.New(now)
	item.Automated = identity.IsAgent()
	bumpSyncToken(checklist, now)
	pruneTombstones(checklist, now)
	checklist.Items[idx] = item

	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, nil, err
	}
	return item, checklist, nil
}

// DeleteItem removes uid from the named checklist and writes a tombstone.
func (m *Mutator) DeleteItem(_ context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, errUIDRequiredMsg)
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)
	if err := checkExpectedUpdatedAt(checklist, expectedUpdatedAt); err != nil {
		return nil, err
	}

	idx, item := findItem(checklist, uid)
	if item == nil {
		return nil, ErrItemNotFound
	}

	now := m.clock.Now()
	checklist.Items = append(checklist.Items[:idx], checklist.Items[idx+1:]...)
	checklist.Tombstones = append(checklist.Tombstones, &apiv1.Tombstone{
		Uid:       uid,
		DeletedAt: timestamppb.New(now),
		GcAfter:   timestamppb.New(now.Add(TombstoneTTL)),
	})
	bumpSyncToken(checklist, now)
	pruneTombstones(checklist, now)

	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, err
	}
	return checklist, nil
}

// ReorderItem updates an item's sort_order. When the requested value
// would collide with another item, the mutator re-densifies adjacent
// values just enough to make room.
func (m *Mutator) ReorderItem(_ context.Context, page, listName, uid string, newSortOrder int64, expectedUpdatedAt *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, errUIDRequiredMsg)
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)
	if err := checkExpectedUpdatedAt(checklist, expectedUpdatedAt); err != nil {
		return nil, err
	}

	idx, item := findItem(checklist, uid)
	if item == nil {
		return nil, ErrItemNotFound
	}

	now := m.clock.Now()
	if item.SortOrder != newSortOrder {
		item.SortOrder = newSortOrder
		item.UpdatedAt = timestamppb.New(now)
		// Resolve any collision by re-densifying adjacent items only.
		densifyAroundSortOrder(checklist.Items, idx)
		sortItems(checklist.Items)
		bumpSyncToken(checklist, now)
	}
	pruneTombstones(checklist, now)

	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, err
	}
	return checklist, nil
}

// ListItems returns the named checklist with all items and any
// surviving tombstones. Read-only — does not write back.
func (m *Mutator) ListItems(_ context.Context, page, listName string) (*apiv1.Checklist, error) {
	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}

	checklist := decodeChecklist(fm, listName, m.clock)
	return checklist, nil
}

// GetChecklists enumerates every checklist on the page.
func (m *Mutator) GetChecklists(_ context.Context, page string) ([]*apiv1.Checklist, error) {
	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}

	names := listChecklistNames(fm)
	out := make([]*apiv1.Checklist, 0, len(names))
	for _, name := range names {
		out = append(out, decodeChecklist(fm, name, m.clock))
	}
	return out, nil
}

// readChecklistForMutation decodes the named checklist and promotes any
// legacy items lacking a uid by assigning a fresh ULID and stamping
// created_at = updated_at = now. This handles two cases:
//
//   - Pages whose checklists were last touched by a raw MergeFrontmatter
//     (where checklists.* user data is still mutable, but the caller has
//     no way to mint ULIDs).
//   - Pages created post-startup that the eager migration job did not
//     see; the next mutation through the funnel cleans them up.
//
// Mutating callers use this; ListItems / GetChecklists do not (they are
// read-only and would otherwise leak fresh uids to the wire on every
// poll).
func (m *Mutator) readChecklistForMutation(fm wikipage.FrontMatter, listName string) *apiv1.Checklist {
	checklist := decodeChecklist(fm, listName, m.clock)
	now := timestamppb.New(m.clock.Now())
	for _, item := range checklist.Items {
		if item.Uid == "" {
			item.Uid = m.ulids.NewULID()
			// The codec synthesized created_at/updated_at if the item had
			// no wiki-managed metadata, but the timestamps were keyed by
			// the empty uid in storage. Reset them with the real uid so
			// the next persist writes them under the correct key.
			if item.CreatedAt == nil {
				item.CreatedAt = now
			}
			if item.UpdatedAt == nil {
				item.UpdatedAt = now
			}
		}
	}
	return checklist
}

// readFrontMatter reads the page's frontmatter, mapping not-found into
// ErrPageNotFound. An empty page (no frontmatter) returns an empty map
// rather than an error so callers can lazy-create checklists.
func (m *Mutator) readFrontMatter(page string) (wikipage.FrontMatter, error) {
	id := wikipage.PageIdentifier(page)
	_, fm, err := m.pages.ReadFrontMatter(id)
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

// persist encodes checklist back into fm and writes the result.
func (m *Mutator) persist(page string, fm wikipage.FrontMatter, listName string, checklist *apiv1.Checklist) error {
	encodeChecklist(fm, listName, checklist)
	id := wikipage.PageIdentifier(page)
	if err := m.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	return nil
}

// lockPage acquires the per-page mutex and returns a release function.
func (m *Mutator) lockPage(page string) func() {
	m.mu.Lock()
	mu, ok := m.pageMu[page]
	if !ok {
		mu = &sync.Mutex{}
		m.pageMu[page] = mu
	}
	m.mu.Unlock()
	mu.Lock()
	return mu.Unlock
}

// checkExpectedUpdatedAt enforces optimistic concurrency. nil means
// "no precondition specified" — caller did not opt in.
func checkExpectedUpdatedAt(checklist *apiv1.Checklist, expected *time.Time) error {
	if expected == nil {
		return nil
	}
	if checklist.UpdatedAt == nil {
		// New checklist — caller's expectation is moot.
		return status.Error(codes.FailedPrecondition, "expected_updated_at mismatch: list has no recorded updated_at")
	}
	if !checklist.UpdatedAt.AsTime().Equal(*expected) {
		return status.Errorf(codes.FailedPrecondition, "expected_updated_at mismatch: server has %s", checklist.UpdatedAt.AsTime().Format(time.RFC3339Nano))
	}
	return nil
}

// findItem returns (index, item) or (-1, nil) when uid is not found.
func findItem(checklist *apiv1.Checklist, uid string) (int, *apiv1.ChecklistItem) {
	for i, it := range checklist.Items {
		if it.Uid == uid {
			return i, it
		}
	}
	return -1, nil
}

// sortItems orders items by SortOrder ascending, with ULID as tiebreaker
// for deterministic ordering.
func sortItems(items []*apiv1.ChecklistItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].Uid < items[j].Uid
	})
}

// nextSortOrder returns max-existing(items.SortOrder) + SortOrderStep,
// or SortOrderStep when items is empty.
func nextSortOrder(items []*apiv1.ChecklistItem) int64 {
	var maxOrder int64
	for _, it := range items {
		if it.SortOrder > maxOrder {
			maxOrder = it.SortOrder
		}
	}
	return maxOrder + SortOrderStep
}

// densifyAroundSortOrder resolves a sort_order collision at items[movedIdx]
// by adjusting adjacent items by +/- 1. The full list is sorted afterwards
// by the caller.
func densifyAroundSortOrder(items []*apiv1.ChecklistItem, movedIdx int) {
	target := items[movedIdx].SortOrder
	for i, it := range items {
		if i == movedIdx {
			continue
		}
		if it.SortOrder == target {
			it.SortOrder++
		}
	}
}

// bumpSyncToken advances the per-list sync token and records the most
// recent updated_at.
func bumpSyncToken(checklist *apiv1.Checklist, now time.Time) {
	checklist.SyncToken++
	checklist.UpdatedAt = timestamppb.New(now)
}

// pruneTombstones drops tombstones whose gc_after has passed.
func pruneTombstones(checklist *apiv1.Checklist, now time.Time) {
	if len(checklist.Tombstones) == 0 {
		return
	}
	kept := checklist.Tombstones[:0]
	for _, t := range checklist.Tombstones {
		if t.GcAfter == nil || t.GcAfter.AsTime().After(now) {
			kept = append(kept, t)
		}
	}
	checklist.Tombstones = kept
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timeAndTimestampEqual(a *time.Time, b *timestamppb.Timestamp) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(b.AsTime())
}

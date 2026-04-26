package checklistmutator

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// UpsertFromCalDAVArgs carries the user-mutable fields a CalDAV PUT may
// set on a single VTODO. It deliberately omits wiki-managed fields
// (created_at, updated_at, completed_at, completed_by, automated) — the
// mutator re-derives those. CompletedAt is the one exception: when a
// CalDAV client explicitly stamps a COMPLETED time on a transition into
// checked=true, the mutator may use it instead of clock.Now() to honor
// "I completed this offline at 9am, syncing at noon."
type UpsertFromCalDAVArgs struct {
	Text         string
	Tags         []string
	Description  *string
	Due          *time.Time
	AlarmPayload *string
	Checked      bool
	// CompletedAt is consulted only on a checked false→true transition.
	// nil means "stamp now". Has no effect on existing-checked items.
	CompletedAt *time.Time
	// Created is consulted only when uid is unknown (creating a new item).
	// nil means "stamp now". Has no effect on existing items.
	Created *time.Time
	// SortOrder is the wiki-side sparse ordering value the client
	// communicated (typically derived from PRIORITY for tasks.org-style
	// clients, or X-APPLE-SORT-ORDER for Apple Reminders). nil means
	// "the client didn't express an order" — on create, mutator
	// appends at nextSortOrder; on update, mutator leaves the stored
	// value alone.
	SortOrder *int64
}

// IfNoneMatchAny is the literal `*` value RFC 4918 §10.4 / RFC 7232 §3.2
// uses to mean "the resource must NOT exist." Pulled out so the literal
// doesn't drift if a typo creeps in.
const IfNoneMatchAny = "*"

// UpsertFromCalDAV creates or updates a single checklist item atomically,
// applying every field change in one sync_token bump. Used by the CalDAV
// PUT handler so a single round-trip from a phone (which may change text,
// tags, checked state, due date, and alarm together) doesn't fragment
// into multiple ETag-changing operations.
//
// Preconditions follow RFC 7232 / RFC 4918 semantics:
//   - ifMatch == "":         no precondition.
//   - ifMatch != "":         the existing item's ETag (rfc3339nano of
//                            updated_at) must equal ifMatch. If the item
//                            does not exist, returns FailedPrecondition.
//   - ifNoneMatch == "":     no precondition.
//   - ifNoneMatch == "*":    the item must NOT exist. If it does, returns
//                            FailedPrecondition. (Create-only PUT.)
func (m *Mutator) UpsertFromCalDAV(
	_ context.Context,
	page, listName, uid string,
	args UpsertFromCalDAVArgs,
	ifMatch string,
	ifNoneMatch string,
	identity tailscale.IdentityValue,
) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	listName = wikipage.NormalizeListName(listName)
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, errUIDRequiredMsg)
	}
	if listName == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "list_name is required")
	}

	unlock := m.lockPage(page)
	defer unlock()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, nil, err
	}

	checklist := m.readChecklistForMutation(fm, listName)
	idx, item := findItem(checklist, uid)
	now := m.clock.Now()

	if item != nil {
		updatedItem, err := m.upsertExistingItem(checklist, idx, item, args, ifMatch, ifNoneMatch, identity, now)
		if err != nil {
			return nil, nil, err
		}
		if err := m.persist(page, fm, listName, checklist); err != nil {
			return nil, nil, err
		}
		return updatedItem, checklist, nil
	}

	if ifMatch != "" {
		return nil, nil, status.Errorf(codes.FailedPrecondition, "If-Match set but uid %s does not exist", uid)
	}
	newItem := m.upsertNewItem(checklist, uid, args, identity, now)
	if err := m.persist(page, fm, listName, checklist); err != nil {
		return nil, nil, err
	}
	return newItem, checklist, nil
}

// upsertExistingItem handles the update branch of UpsertFromCalDAV.
// Returns the (possibly mutated) item or an error mapped from a failed
// precondition. The caller is responsible for persisting the checklist.
func (*Mutator) upsertExistingItem(
	checklist *apiv1.Checklist,
	idx int,
	item *apiv1.ChecklistItem,
	args UpsertFromCalDAVArgs,
	ifMatch, ifNoneMatch string,
	identity tailscale.IdentityValue,
	now time.Time,
) (*apiv1.ChecklistItem, error) {
	if ifNoneMatch == IfNoneMatchAny {
		return nil, status.Errorf(codes.FailedPrecondition, "If-None-Match: * but uid %s already exists", item.Uid)
	}
	if ifMatch != "" {
		currentETag := item.UpdatedAt.AsTime().Format(time.RFC3339Nano)
		if ifMatch != currentETag {
			return nil, status.Errorf(codes.FailedPrecondition, "If-Match mismatch: server has %s", currentETag)
		}
	}

	userChanged := applyUserMutableFields(item, asUpdateItemArgs(args))
	checkedChanged := applyCheckedTransition(item, args, identity, now)
	sortOrderChanged := false
	if args.SortOrder != nil && *args.SortOrder != item.SortOrder {
		item.SortOrder = *args.SortOrder
		sortOrderChanged = true
	}

	if userChanged || checkedChanged || sortOrderChanged {
		item.UpdatedAt = timestamppb.New(now)
		item.Automated = identity.IsAgent()
		bumpSyncToken(checklist, now)
	}
	if sortOrderChanged {
		// Re-densify around the moved item the same way ReorderItem
		// does so a SortOrder collision doesn't leave the list with
		// duplicate values that confuse later writes.
		densifyAroundSortOrder(checklist.Items, idx)
		sortItems(checklist.Items)
	}
	pruneTombstones(checklist, now)
	if !sortOrderChanged {
		checklist.Items[idx] = item
	}
	return item, nil
}

// upsertNewItem handles the create branch of UpsertFromCalDAV. The
// caller is responsible for persisting the checklist.
func (*Mutator) upsertNewItem(
	checklist *apiv1.Checklist,
	uid string,
	args UpsertFromCalDAVArgs,
	identity tailscale.IdentityValue,
	now time.Time,
) *apiv1.ChecklistItem {
	createdAt := now
	if args.Created != nil {
		createdAt = *args.Created
	}
	sortOrder := nextSortOrder(checklist.Items)
	if args.SortOrder != nil {
		sortOrder = *args.SortOrder
	}
	newItem := &apiv1.ChecklistItem{
		Uid:          uid,
		Text:         args.Text,
		Checked:      args.Checked,
		Tags:         append([]string(nil), args.Tags...),
		SortOrder:    sortOrder,
		Description:  args.Description,
		AlarmPayload: args.AlarmPayload,
		CreatedAt:    timestamppb.New(createdAt),
		UpdatedAt:    timestamppb.New(now),
		Automated:    identity.IsAgent(),
	}
	if args.Due != nil {
		newItem.Due = timestamppb.New(*args.Due)
	}
	if args.Checked {
		stampCompleted(newItem, args.CompletedAt, identity, now)
	}
	checklist.Items = append(checklist.Items, newItem)
	sortItems(checklist.Items)
	bumpSyncToken(checklist, now)
	pruneTombstones(checklist, now)
	return newItem
}

// applyCheckedTransition flips item.Checked to args.Checked when they
// differ, stamping completed_at/completed_by on false→true and clearing
// them on true→false. Returns whether anything changed.
func applyCheckedTransition(item *apiv1.ChecklistItem, args UpsertFromCalDAVArgs, identity tailscale.IdentityValue, now time.Time) bool {
	if item.Checked == args.Checked {
		return false
	}
	item.Checked = args.Checked
	if args.Checked {
		stampCompleted(item, args.CompletedAt, identity, now)
	} else {
		item.CompletedAt = nil
		item.CompletedBy = nil
	}
	return true
}

// stampCompleted writes completed_at and completed_by on a transition
// into checked=true. Honors an explicit timestamp from the CalDAV client
// (offline completion) when provided; otherwise stamps clock.Now().
func stampCompleted(item *apiv1.ChecklistItem, explicit *time.Time, identity tailscale.IdentityValue, now time.Time) {
	if explicit != nil {
		item.CompletedAt = timestamppb.New(*explicit)
	} else {
		item.CompletedAt = timestamppb.New(now)
	}
	name := identity.Name()
	item.CompletedBy = &name
}

// asUpdateItemArgs adapts the CalDAV-shaped args to the existing
// applyUserMutableFields helper signature. CalDAV always asserts a value
// for every user-mutable field on every PUT (the body is the new state),
// so every field is "set" with respect to the helper.
func asUpdateItemArgs(args UpsertFromCalDAVArgs) UpdateItemArgs {
	return UpdateItemArgs{
		Text:            &args.Text,
		Tags:            args.Tags,
		TagsSet:         true,
		Description:     args.Description,
		DescriptionSet:  true,
		Due:             args.Due,
		DueSet:          true,
		AlarmPayload:    args.AlarmPayload,
		AlarmPayloadSet: true,
	}
}

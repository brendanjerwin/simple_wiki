//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errDeadLetterProgrammed is the suite-local sentinel the dead-letter
// tests use when programming a fake to fail. Distinct from the other
// suite sentinels so a MatchError(...) failure points at the right
// suite.
var errDeadLetterProgrammed = errors.New("dead-letter programmed failure")

// deadLetterFixedNow is the timestamp the dead-letter tests pin into
// the FakeClock so NextAttemptAt assertions have an exact reference
// point.
var deadLetterFixedNow = time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)

// captureLogger is a Logger that records every Info/Warn/Error format
// string into Lines. Lets tests assert on the structured-log surface
// (e.g., "dead_letter_threshold_breached" appears at count >= 10).
type captureLogger struct {
	mu    sync.Mutex
	Lines []capturedLine
}

type capturedLine struct {
	Level  string
	Format string
}

func (l *captureLogger) Info(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Lines = append(l.Lines, capturedLine{Level: "Info", Format: format})
}

func (l *captureLogger) Warn(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Lines = append(l.Lines, capturedLine{Level: "Warn", Format: format})
}

func (l *captureLogger) Error(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Lines = append(l.Lines, capturedLine{Level: "Error", Format: format})
}

func (l *captureLogger) snapshot() []capturedLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]capturedLine, len(l.Lines))
	copy(out, l.Lines)
	return out
}

// pushFailuresOf reads the engine's push_failures subtree out of an
// AdapterState. The subtree is a map[string]any keyed by uid, where
// each entry is map[string]any{"count": int, "next_attempt_at": "..."}.
// Tests use this to assert on the persisted bookkeeping shape without
// importing internal helpers.
func pushFailuresOf(state connectors.AdapterState) map[string]map[string]any {
	out := map[string]map[string]any{}
	if state == nil {
		return out
	}
	raw, ok := state["push_failures"]
	if !ok || raw == nil {
		return out
	}
	asMap, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for uid, entryAny := range asMap {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		out[uid] = entry
	}
	return out
}

var _ = Describe("Engine dead-letter retry bookkeeping", func() {
	const (
		profileID = wikipage.PageIdentifier("alice_profile")
		page      = "groceries"
		listName  = "this_week"
		ownerKind = connectors.ConnectorKindGoogleTasks
		uid       = "uid-1"
	)

	var (
		fa     *enginetesting.FakeAdapter
		fbs    *enginetesting.FakeBindingStore
		lease  *connectors.LeaseTable
		clock  *enginetesting.FakeClock
		logger *captureLogger
		eng    *engine.Engine

		baseBinding connectors.Binding
	)

	BeforeEach(func() {
		fa = &enginetesting.FakeAdapter{ConnectorKind: ownerKind}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady()
		clock = enginetesting.NewFakeClock(deadLetterFixedNow)
		logger = &captureLogger{}

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			&recordingChecklistReader{checklist: &apiv1.Checklist{}},
			&trackingChecklistMutator{recordingChecklistMutator: &recordingChecklistMutator{}},
			&recordingSuppressor{},
			logger, clock, fbs,
		)
		Expect(err).NotTo(HaveOccurred())

		baseBinding = connectors.Binding{
			ProfileID: profileID, Page: page, ListName: listName,
			RemoteHandle: "tasklist-1",
			State:        connectors.BindingStateActive,
			AdapterState: connectors.AdapterState{
				// Pre-existing item_id_map subtree must be preserved
				// across recordPushFailure / recordPushSuccess writes.
				"item_id_map": map[string]string{
					"uid-1": "ref-1",
					"uid-2": "ref-2",
				},
			},
		}
	})

	Describe("recordPushFailure", func() {
		When("called for the first time on a uid with no prior record", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				updated = eng.RecordPushFailureForTest(baseBinding, uid, "outbound_inserted", errDeadLetterProgrammed)
			})

			It("should set count to 1", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				Expect(rec["count"]).To(Equal(1))
			})

			It("should set next_attempt_at to now + 60s", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				expected := deadLetterFixedNow.Add(60 * time.Second).UTC().Format(time.RFC3339)
				Expect(rec["next_attempt_at"]).To(Equal(expected))
			})

			It("should preserve the item_id_map subtree", func() {
				Expect(updated.AdapterState["item_id_map"]).To(Equal(map[string]string{
					"uid-1": "ref-1",
					"uid-2": "ref-2",
				}))
			})

			It("should not log dead_letter_threshold_breached", func() {
				for _, line := range logger.snapshot() {
					Expect(line.Format).NotTo(ContainSubstring("dead_letter_threshold_breached"))
				}
			})
		})

		When("called twice in a row on the same uid", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				once := eng.RecordPushFailureForTest(baseBinding, uid, "outbound_inserted", errDeadLetterProgrammed)
				updated = eng.RecordPushFailureForTest(once, uid, "outbound_inserted", errDeadLetterProgrammed)
			})

			It("should set count to 2", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				Expect(rec["count"]).To(Equal(2))
			})

			It("should set next_attempt_at to now + 120s (60s * 2^1)", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				expected := deadLetterFixedNow.Add(120 * time.Second).UTC().Format(time.RFC3339)
				Expect(rec["next_attempt_at"]).To(Equal(expected))
			})
		})

		When("called once on each of two distinct uids", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				once := eng.RecordPushFailureForTest(baseBinding, "uid-a", "outbound_inserted", errDeadLetterProgrammed)
				updated = eng.RecordPushFailureForTest(once, "uid-b", "outbound_patched", errDeadLetterProgrammed)
			})

			It("should track uid-a with count 1", func() {
				rec := pushFailuresOf(updated.AdapterState)["uid-a"]
				Expect(rec["count"]).To(Equal(1))
			})

			It("should track uid-b with count 1", func() {
				rec := pushFailuresOf(updated.AdapterState)["uid-b"]
				Expect(rec["count"]).To(Equal(1))
			})
		})

		When("the post-increment count meets deadLetterThreshold", func() {
			var (
				updated   connectors.Binding
				breached  bool
				logLines  []capturedLine
			)

			BeforeEach(func() {
				cur := baseBinding
				for i := 0; i < engine.DeadLetterThresholdForTest; i++ {
					cur = eng.RecordPushFailureForTest(cur, uid, "outbound_patched", errDeadLetterProgrammed)
				}
				updated = cur
				logLines = logger.snapshot()
				for _, line := range logLines {
					if line.Format != "" && containsSubstring(line.Format, "dead_letter_threshold_breached") {
						breached = true
						break
					}
				}
			})

			It("should record count equal to the threshold", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				Expect(rec["count"]).To(Equal(engine.DeadLetterThresholdForTest))
			})

			It("should still populate next_attempt_at", func() {
				rec := pushFailuresOf(updated.AdapterState)[uid]
				Expect(rec["next_attempt_at"]).NotTo(BeEmpty())
			})

			It("should emit a dead_letter_threshold_breached log event", func() {
				Expect(breached).To(BeTrue())
			})
		})
	})

	Describe("recordPushSuccess", func() {
		When("there is an existing failure record for the uid", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				with := eng.RecordPushFailureForTest(baseBinding, uid, "outbound_inserted", errDeadLetterProgrammed)
				updated = eng.RecordPushSuccessForTest(with, uid)
			})

			It("should remove the uid's failure record", func() {
				_, present := pushFailuresOf(updated.AdapterState)[uid]
				Expect(present).To(BeFalse())
			})

			It("should preserve the item_id_map subtree", func() {
				Expect(updated.AdapterState["item_id_map"]).To(Equal(map[string]string{
					"uid-1": "ref-1",
					"uid-2": "ref-2",
				}))
			})
		})

		When("there are existing failure records for two distinct uids", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				step := eng.RecordPushFailureForTest(baseBinding, "uid-a", "outbound_inserted", errDeadLetterProgrammed)
				step = eng.RecordPushFailureForTest(step, "uid-b", "outbound_patched", errDeadLetterProgrammed)
				updated = eng.RecordPushSuccessForTest(step, "uid-a")
			})

			It("should remove uid-a's record", func() {
				_, present := pushFailuresOf(updated.AdapterState)["uid-a"]
				Expect(present).To(BeFalse())
			})

			It("should preserve uid-b's record", func() {
				rec, present := pushFailuresOf(updated.AdapterState)["uid-b"]
				Expect(present).To(BeTrue())
				Expect(rec["count"]).To(Equal(1))
			})
		})

		When("called for a uid with no existing record", func() {
			var updated connectors.Binding

			BeforeEach(func() {
				updated = eng.RecordPushSuccessForTest(baseBinding, "uid-never-failed")
			})

			It("should be a no-op (no record present)", func() {
				_, present := pushFailuresOf(updated.AdapterState)["uid-never-failed"]
				Expect(present).To(BeFalse())
			})

			It("should preserve the item_id_map subtree", func() {
				Expect(updated.AdapterState["item_id_map"]).To(Equal(map[string]string{
					"uid-1": "ref-1",
					"uid-2": "ref-2",
				}))
			})
		})
	})

	Describe("shouldSkipPush", func() {
		When("there is no failure record for the uid", func() {
			var skip bool
			var reason string

			BeforeEach(func() {
				skip, reason = eng.ShouldSkipPushForTest(baseBinding, uid)
			})

			It("should not skip", func() {
				Expect(skip).To(BeFalse())
			})

			It("should return an empty reason", func() {
				Expect(reason).To(BeEmpty())
			})
		})

		When("the failure record's next_attempt_at is in the future", func() {
			var (
				bindingWithFailure connectors.Binding
				skip               bool
				reason             string
			)

			BeforeEach(func() {
				bindingWithFailure = eng.RecordPushFailureForTest(baseBinding, uid, "outbound_patched", errDeadLetterProgrammed)
				// clock pinned at deadLetterFixedNow; backoff window
				// is 60s, so next_attempt_at = now + 60s > now.
				skip, reason = eng.ShouldSkipPushForTest(bindingWithFailure, uid)
			})

			It("should skip", func() {
				Expect(skip).To(BeTrue())
			})

			It("should return reason=backoff", func() {
				Expect(reason).To(Equal("backoff"))
			})
		})

		When("the backoff window has elapsed and count is below threshold", func() {
			var (
				bindingWithFailure connectors.Binding
				skip               bool
				reason             string
			)

			BeforeEach(func() {
				bindingWithFailure = eng.RecordPushFailureForTest(baseBinding, uid, "outbound_patched", errDeadLetterProgrammed)
				// Advance clock past next_attempt_at (60s) so backoff
				// window has elapsed.
				clock.SetNow(deadLetterFixedNow.Add(2 * time.Minute))
				skip, reason = eng.ShouldSkipPushForTest(bindingWithFailure, uid)
			})

			It("should not skip", func() {
				Expect(skip).To(BeFalse())
			})

			It("should return an empty reason", func() {
				Expect(reason).To(BeEmpty())
			})
		})

		When("count meets deadLetterThreshold", func() {
			var (
				bindingDeadLettered connectors.Binding
				skip                bool
				reason              string
			)

			BeforeEach(func() {
				cur := baseBinding
				for i := 0; i < engine.DeadLetterThresholdForTest; i++ {
					cur = eng.RecordPushFailureForTest(cur, uid, "outbound_patched", errDeadLetterProgrammed)
				}
				bindingDeadLettered = cur
				// Advance clock far past any backoff window; threshold
				// branch must dominate.
				clock.SetNow(deadLetterFixedNow.Add(48 * time.Hour))
				skip, reason = eng.ShouldSkipPushForTest(bindingDeadLettered, uid)
			})

			It("should skip", func() {
				Expect(skip).To(BeTrue())
			})

			It("should return reason=dead_letter", func() {
				Expect(reason).To(Equal("dead_letter"))
			})
		})

		When("two uids have records, one in backoff and one healthy", func() {
			var (
				bindingMixed connectors.Binding
			)

			BeforeEach(func() {
				bindingMixed = eng.RecordPushFailureForTest(baseBinding, "uid-failed", "outbound_patched", errDeadLetterProgrammed)
			})

			It("should skip the failed uid", func() {
				skip, _ := eng.ShouldSkipPushForTest(bindingMixed, "uid-failed")
				Expect(skip).To(BeTrue())
			})

			It("should not skip a different uid", func() {
				skip, _ := eng.ShouldSkipPushForTest(bindingMixed, "uid-healthy")
				Expect(skip).To(BeFalse())
			})
		})
	})

	Describe("reconcile integration: skip-when-not-yet-eligible", func() {
		const reconcileUID = "uid-future"

		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		When("a bound item has next_attempt_at in the future", func() {
			BeforeEach(func() {
				// Seed a binding with one item_id_map entry and a
				// pre-existing future-dated push_failure for the
				// same uid.
				bindingState := connectors.AdapterState{
					"item_id_map": map[string]string{
						reconcileUID: "ref-future",
					},
					"push_failures": map[string]any{
						reconcileUID: map[string]any{
							"count":           3,
							"next_attempt_at": deadLetterFixedNow.Add(10 * time.Minute).UTC().Format(time.RFC3339),
						},
					},
				}

				binding := connectors.Binding{
					ProfileID: profileID, Page: page, ListName: listName,
					RemoteHandle:         "tasklist-1",
					State:                connectors.BindingStateActive,
					LastSuccessfulSyncAt: deadLetterFixedNow.Add(-1 * time.Hour),
					AdapterState:         bindingState,
				}
				fbs.SeedBinding(binding, ownerKind)

				// Replace engine with one whose checklist reader returns
				// a checklist that contains the uid (so the outbound
				// path would otherwise call PatchRemote on it).
				cl := &apiv1.Checklist{
					Items: []*apiv1.ChecklistItem{
						{Uid: reconcileUID, Text: "future item"},
					},
				}
				reader := &recordingChecklistReader{checklist: cl}
				mutator := &trackingChecklistMutator{recordingChecklistMutator: &recordingChecklistMutator{}}
				supr := &recordingSuppressor{}
				fa = &enginetesting.FakeAdapter{ConnectorKind: ownerKind}
				var err error
				eng, err = engine.NewEngine(
					fa, lease,
					reader, mutator, supr,
					logger, clock, fbs,
				)
				Expect(err).NotTo(HaveOccurred())

				_ = eng.Sync(ctx, connectors.BindingKey{
					ProfileID: string(profileID),
					Page:      page,
					ListName:  listName,
				})
			})

			It("should not call PatchRemote for the gated uid", func() {
				Expect(fa.RecordedPatchRemote).To(BeEmpty())
			})

			It("should not call DeleteRemote for the gated uid", func() {
				Expect(fa.RecordedDeleteRemote).To(BeEmpty())
			})
		})

		When("the outbound PatchRemote returns ErrorClassRetryable", func() {
			var savedBinding connectors.Binding

			BeforeEach(func() {
				bindingState := connectors.AdapterState{
					"item_id_map": map[string]string{
						reconcileUID: "ref-retryable",
					},
				}
				binding := connectors.Binding{
					ProfileID: profileID, Page: page, ListName: listName,
					RemoteHandle:         "tasklist-1",
					State:                connectors.BindingStateActive,
					LastSuccessfulSyncAt: deadLetterFixedNow.Add(-1 * time.Hour),
					AdapterState:         bindingState,
				}
				fbs.SeedBinding(binding, ownerKind)

				cl := &apiv1.Checklist{
					Items: []*apiv1.ChecklistItem{
						{Uid: reconcileUID, Text: "retryable item"},
					},
				}
				reader := &recordingChecklistReader{checklist: cl}
				mutator := &trackingChecklistMutator{recordingChecklistMutator: &recordingChecklistMutator{}}
				supr := &recordingSuppressor{}
				fa = &enginetesting.FakeAdapter{ConnectorKind: ownerKind}
				fa.SetPatchRemoteResponse("", errDeadLetterProgrammed)
				fa.SetClassifyErrorResponse(connectors.ErrorClassRetryable)

				var err error
				eng, err = engine.NewEngine(
					fa, lease,
					reader, mutator, supr,
					logger, clock, fbs,
				)
				Expect(err).NotTo(HaveOccurred())

				_ = eng.Sync(ctx, connectors.BindingKey{
					ProfileID: string(profileID),
					Page:      page,
					ListName:  listName,
				})
				if len(fbs.RecordedSaveBinding) > 0 {
					savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
				}
			})

			It("should record a push_failure entry for the uid in the saved binding", func() {
				rec, present := pushFailuresOf(savedBinding.AdapterState)[reconcileUID]
				Expect(present).To(BeTrue())
				Expect(rec["count"]).To(Equal(1))
			})
		})
	})
})

// containsSubstring is a tiny helper that lets the threshold-breach
// test check whether the captured log format contains a target substring
// without pulling in strings.Contains under a dot-import alias. Kept
// local so the test file stays self-contained.
func containsSubstring(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

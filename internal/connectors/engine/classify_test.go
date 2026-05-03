//revive:disable:dot-imports
package engine_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
)

func TestEngine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "engine")
}

// makeChecklist is a tiny fixture for engine tests. The op-log is the
// only field the classifier reads.
func makeChecklist(events ...*apiv1.ChecklistEvent) *apiv1.Checklist {
	return &apiv1.Checklist{Events: events}
}

func event(seq int64, uid, src string) *apiv1.ChecklistEvent {
	return &apiv1.ChecklistEvent{Seq: seq, Uid: uid, Src: src, Op: "toggle"}
}

var _ = Describe("Classify", func() {
	When("the checklist is nil", func() {
		It("should return an empty map", func() {
			Expect(engine.Classify(nil, engine.SubscriptionCursor{}, "google_tasks")).To(BeEmpty())
		})
	})

	When("no events exist", func() {
		It("should return an empty map", func() {
			Expect(engine.Classify(makeChecklist(), engine.SubscriptionCursor{}, "google_tasks")).To(BeEmpty())
		})
	})

	When("all events are at or below the cursor", func() {
		It("should ignore them (already-consumed history)", func() {
			cl := makeChecklist(
				event(1, "u1", "user:alice@example.com"),
				event(2, "u2", "connector:google_tasks:apply"),
			)
			cursor := engine.SubscriptionCursor{LastSyncedSeq: 5}
			Expect(engine.Classify(cl, cursor, "google_tasks")).To(BeEmpty())
		})
	})

	When("a user-source event exists since the cursor", func() {
		var result map[string]engine.EventClassification

		BeforeEach(func() {
			cl := makeChecklist(event(7, "u1", "user:alice@example.com"))
			result = engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 5}, "google_tasks")
		})

		It("should report WikiDiverged=true for the uid", func() {
			Expect(result).To(HaveKey("u1"))
			Expect(result["u1"].WikiDiverged).To(BeTrue())
		})

		It("should record the latest event's src for diagnostics", func() {
			Expect(result["u1"].LatestEventSource).To(Equal("user:alice@example.com"))
		})
	})

	When("only this connector's own apply events exist since the cursor", func() {
		It("should report WikiDiverged=false (idempotent re-fetch from cursor-safety-buffer)", func() {
			cl := makeChecklist(
				event(7, "u1", "connector:google_tasks:apply"),
				event(8, "u1", "connector:google_tasks:apply"),
			)
			result := engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 5}, "google_tasks")
			Expect(result).To(HaveKey("u1"))
			Expect(result["u1"].WikiDiverged).To(BeFalse())
		})
	})

	When("a different connector's apply event exists since the cursor", func() {
		It("should report WikiDiverged=true (cross-connector — defer to the other connector's authority)", func() {
			cl := makeChecklist(event(7, "u1", "connector:google_keep:apply"))
			result := engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 5}, "google_tasks")
			Expect(result).To(HaveKey("u1"))
			Expect(result["u1"].WikiDiverged).To(BeTrue())
		})
	})

	When("a migration baseline event exists since the cursor", func() {
		It("should report WikiDiverged=true (treat as non-self, fail-safe)", func() {
			cl := makeChecklist(event(7, "u1", "migration:initial_baseline"))
			result := engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 5}, "google_tasks")
			Expect(result["u1"].WikiDiverged).To(BeTrue())
		})
	})

	When("self-writes follow a user edit (the bug-class this rule fixes)", func() {
		var result map[string]engine.EventClassification

		BeforeEach(func() {
			// User edits at seq=10. Connector's own apply lands at
			// seq=11 (push_recovery from 412), then a cursor-safety-
			// buffer re-fetch produces seq=12. Cursor is at seq=9.
			// The user-edit's divergence must STICK across subsequent
			// self-writes.
			cl := makeChecklist(
				event(10, "u1", "user:alice@example.com"),
				event(11, "u1", "connector:google_tasks:push_recovery"),
				event(12, "u1", "connector:google_tasks:apply"),
			)
			result = engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 9}, "google_tasks")
		})

		It("should report WikiDiverged=true (a non-self event existed since cursor)", func() {
			Expect(result["u1"].WikiDiverged).To(BeTrue())
		})

		It("should record the latest seq seen", func() {
			Expect(result["u1"].LatestEventSeq).To(Equal(int64(12)))
		})
	})

	When("multiple uids have events", func() {
		It("should classify each uid independently", func() {
			cl := makeChecklist(
				event(7, "u1", "user:alice@example.com"),
				event(8, "u2", "connector:google_tasks:apply"),
				event(9, "u3", "connector:google_keep:apply"),
			)
			result := engine.Classify(cl, engine.SubscriptionCursor{LastSyncedSeq: 5}, "google_tasks")
			Expect(result["u1"].WikiDiverged).To(BeTrue())
			Expect(result["u2"].WikiDiverged).To(BeFalse())
			Expect(result["u3"].WikiDiverged).To(BeTrue())
		})
	})

	When("an event has empty uid", func() {
		It("should be skipped", func() {
			cl := makeChecklist(&apiv1.ChecklistEvent{Seq: 7, Uid: "", Src: "user:a@b"})
			Expect(engine.Classify(cl, engine.SubscriptionCursor{}, "google_tasks")).To(BeEmpty())
		})
	})
})

var _ = Describe("SourcePrefixForKind", func() {
	It("should produce the connector:<kind>: prefix", func() {
		Expect(engine.SourcePrefixForKind("google_tasks")).To(Equal("connector:google_tasks:"))
	})
})

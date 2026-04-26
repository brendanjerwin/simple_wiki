//revive:disable:dot-imports
// Internal tests for unexported codec functions. These run inside the
// checklistmutator package so they can exercise every private helper
// without exporting it.  The TestMutator suite runner in mutator_test.go
// (package checklistmutator_test) picks up these Describe blocks via
// Ginkgo's shared global suite.
package checklistmutator

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// internalClock implements Clock for codec-level tests.
// fakeClock lives in mutator_test.go (package checklistmutator_test), so we
// define our own lightweight version here.
type internalClock struct{ t time.Time }

func (c internalClock) Now() time.Time { return c.t }

var (
	codecNow   = time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	codecClock = internalClock{t: codecNow}
)

var _ = Describe("readMap", func() {
	When("m is nil", func() {
		It("should return nil", func() {
			Expect(readMap(nil, "key")).To(BeNil())
		})
	})

	When("the key is missing", func() {
		It("should return nil", func() {
			Expect(readMap(map[string]any{"other": 1}, "key")).To(BeNil())
		})
	})

	When("the value is not a map[string]any", func() {
		It("should return nil", func() {
			Expect(readMap(map[string]any{"key": "string"}, "key")).To(BeNil())
		})
	})

	When("the value is a map[string]any", func() {
		It("should return the inner map", func() {
			inner := map[string]any{"x": 1}
			Expect(readMap(map[string]any{"key": inner}, "key")).To(Equal(inner))
		})
	})
})

var _ = Describe("readSlice", func() {
	When("m is nil", func() {
		It("should return nil", func() {
			Expect(readSlice(nil, "key")).To(BeNil())
		})
	})

	When("the key is missing", func() {
		It("should return nil", func() {
			Expect(readSlice(map[string]any{}, "key")).To(BeNil())
		})
	})

	When("the value is a []any", func() {
		It("should return it", func() {
			s := []any{"a", "b"}
			Expect(readSlice(map[string]any{"key": s}, "key")).To(Equal(s))
		})
	})

	When("the value is not a []any", func() {
		It("should return nil", func() {
			Expect(readSlice(map[string]any{"key": "str"}, "key")).To(BeNil())
		})
	})
})

var _ = Describe("readInt64", func() {
	When("m is nil", func() {
		It("should return (0, false)", func() {
			v, ok := readInt64(nil, "k")
			Expect(ok).To(BeFalse())
			Expect(v).To(Equal(int64(0)))
		})
	})

	When("key holds an int64", func() {
		It("should return it", func() {
			v, ok := readInt64(map[string]any{"k": int64(42)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(42)))
		})
	})

	When("key holds an int", func() {
		It("should coerce to int64", func() {
			v, ok := readInt64(map[string]any{"k": int(7)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(7)))
		})
	})

	When("key holds a float64", func() {
		It("should coerce to int64", func() {
			v, ok := readInt64(map[string]any{"k": float64(3.9)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(3)))
		})
	})

	When("key is missing", func() {
		It("should return (0, false)", func() {
			v, ok := readInt64(map[string]any{}, "k")
			Expect(ok).To(BeFalse())
			Expect(v).To(Equal(int64(0)))
		})
	})

	When("key holds an unsupported type", func() {
		It("should return (0, false)", func() {
			v, ok := readInt64(map[string]any{"k": "string"}, "k")
			Expect(ok).To(BeFalse())
			Expect(v).To(Equal(int64(0)))
		})
	})
})

var _ = Describe("readTimestampValue", func() {
	When("value is a valid RFC3339Nano string", func() {
		It("should parse and return the timestamp", func() {
			t := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
			ts, ok := readTimestampValue(t.Format(time.RFC3339Nano))
			Expect(ok).To(BeTrue())
			Expect(ts.AsTime().Equal(t)).To(BeTrue())
		})
	})

	When("value is an invalid string", func() {
		It("should return (nil, false)", func() {
			ts, ok := readTimestampValue("not-a-time")
			Expect(ok).To(BeFalse())
			Expect(ts).To(BeNil())
		})
	})

	When("value is a time.Time", func() {
		It("should return the timestamp", func() {
			t := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
			ts, ok := readTimestampValue(t)
			Expect(ok).To(BeTrue())
			Expect(ts.AsTime().Equal(t)).To(BeTrue())
		})
	})

	When("value is nil", func() {
		It("should return (nil, false)", func() {
			ts, ok := readTimestampValue(nil)
			Expect(ok).To(BeFalse())
			Expect(ts).To(BeNil())
		})
	})

	When("value is an integer", func() {
		It("should return (nil, false)", func() {
			ts, ok := readTimestampValue(int64(999))
			Expect(ok).To(BeFalse())
			Expect(ts).To(BeNil())
		})
	})
})

var _ = Describe("stringValue", func() {
	When("m is nil", func() {
		It("should return empty string", func() {
			Expect(stringValue(nil, "k")).To(BeEmpty())
		})
	})

	When("key holds a string", func() {
		It("should return it", func() {
			Expect(stringValue(map[string]any{"k": "hello"}, "k")).To(Equal("hello"))
		})
	})

	When("key is missing", func() {
		It("should return empty string", func() {
			Expect(stringValue(map[string]any{}, "k")).To(BeEmpty())
		})
	})

	When("value is not a string", func() {
		It("should return empty string", func() {
			Expect(stringValue(map[string]any{"k": 42}, "k")).To(BeEmpty())
		})
	})
})

var _ = Describe("boolValue", func() {
	When("m is nil", func() {
		It("should return false", func() {
			Expect(boolValue(nil, "k")).To(BeFalse())
		})
	})

	When("key holds true", func() {
		It("should return true", func() {
			Expect(boolValue(map[string]any{"k": true}, "k")).To(BeTrue())
		})
	})

	When("key holds false", func() {
		It("should return false", func() {
			Expect(boolValue(map[string]any{"k": false}, "k")).To(BeFalse())
		})
	})

	When("key is missing", func() {
		It("should return false", func() {
			Expect(boolValue(map[string]any{}, "k")).To(BeFalse())
		})
	})

	When("value is not a bool", func() {
		It("should return false", func() {
			Expect(boolValue(map[string]any{"k": "yes"}, "k")).To(BeFalse())
		})
	})
})

var _ = Describe("int64Value", func() {
	When("key holds an int64", func() {
		It("should return it", func() {
			Expect(int64Value(map[string]any{"k": int64(99)}, "k")).To(Equal(int64(99)))
		})
	})

	When("key is missing", func() {
		It("should return 0", func() {
			Expect(int64Value(map[string]any{}, "k")).To(Equal(int64(0)))
		})
	})
})

var _ = Describe("stringSlice", func() {
	When("value is a []string", func() {
		It("should return a copy", func() {
			m := map[string]any{"k": []string{"a", "b"}}
			Expect(stringSlice(m, "k")).To(Equal([]string{"a", "b"}))
		})
	})

	When("value is a []any of strings", func() {
		It("should convert each element", func() {
			m := map[string]any{"k": []any{"x", "y"}}
			Expect(stringSlice(m, "k")).To(Equal([]string{"x", "y"}))
		})
	})

	When("value is a []any with non-string elements", func() {
		It("should skip non-strings", func() {
			m := map[string]any{"k": []any{"a", 42, "b"}}
			Expect(stringSlice(m, "k")).To(Equal([]string{"a", "b"}))
		})
	})

	When("key is missing", func() {
		It("should return nil", func() {
			Expect(stringSlice(map[string]any{}, "k")).To(BeNil())
		})
	})
})

var _ = Describe("ensureMap", func() {
	When("key already holds a map", func() {
		It("should return the existing map", func() {
			existing := map[string]any{"inner": 1}
			outer := map[string]any{"k": existing}
			Expect(ensureMap(outer, "k")).To(Equal(existing))
		})
	})

	When("key is missing", func() {
		It("should create and store a new empty map", func() {
			outer := map[string]any{}
			result := ensureMap(outer, "k")
			Expect(result).NotTo(BeNil())
			Expect(outer["k"]).To(Equal(result))
		})
	})

	When("key holds a non-map value", func() {
		It("should replace it with a new empty map", func() {
			outer := map[string]any{"k": "not-a-map"}
			result := ensureMap(outer, "k")
			Expect(result).NotTo(BeNil())
			Expect(outer["k"]).To(Equal(result))
		})
	})
})

var _ = Describe("decodeTombstones", func() {
	When("raw is nil", func() {
		It("should return nil", func() {
			Expect(decodeTombstones(nil)).To(BeNil())
		})
	})

	When("raw is empty", func() {
		It("should return nil", func() {
			Expect(decodeTombstones([]any{})).To(BeNil())
		})
	})

	When("raw contains valid tombstone maps", func() {
		var result []*apiv1.Tombstone

		BeforeEach(func() {
			deletedAt := codecNow
			gcAfter := deletedAt.Add(TombstoneTTL)
			raw := []any{
				map[string]any{
					"uid":        "uid-x",
					"deleted_at": deletedAt.Format(time.RFC3339Nano),
					"gc_after":   gcAfter.Format(time.RFC3339Nano),
				},
			}
			result = decodeTombstones(raw)
		})

		It("should parse the uid", func() {
			Expect(result).To(HaveLen(1))
			Expect(result[0].Uid).To(Equal("uid-x"))
		})

		It("should parse deleted_at", func() {
			Expect(result[0].DeletedAt).NotTo(BeNil())
		})

		It("should parse gc_after", func() {
			Expect(result[0].GcAfter).NotTo(BeNil())
		})
	})

	When("raw contains non-map elements", func() {
		It("should skip them", func() {
			raw := []any{"not-a-map", 42}
			Expect(decodeTombstones(raw)).To(BeEmpty())
		})
	})

	When("tombstone has no deleted_at or gc_after", func() {
		It("should decode with nil timestamps", func() {
			raw := []any{map[string]any{"uid": "u"}}
			result := decodeTombstones(raw)
			Expect(result).To(HaveLen(1))
			Expect(result[0].DeletedAt).To(BeNil())
			Expect(result[0].GcAfter).To(BeNil())
		})
	})
})

var _ = Describe("decodeItem", func() {
	When("item has all standard fields", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			createdAt := codecNow.Add(-time.Hour)
			updatedAt := codecNow.Add(-30 * time.Minute)
			completedAt := codecNow.Add(-15 * time.Minute)
			completedBy := "alice"
			desc := "do it"
			alarm := "cal:payload"

			m := map[string]any{
				"uid":          "abc123",
				"text":         "Buy milk",
				"checked":      true,
				"tags":         []any{"grocery"},
				"sort_order":   int64(1000),
				"automated":    true,
				"description":  desc,
				"due":          codecNow.Format(time.RFC3339Nano),
				"alarm_payload": alarm,
				"created_at":   createdAt.Format(time.RFC3339Nano),
				"updated_at":   updatedAt.Format(time.RFC3339Nano),
				"completed_at": completedAt.Format(time.RFC3339Nano),
				"completed_by": completedBy,
			}
			item = decodeItem(m, codecNow)
		})

		It("should decode uid", func() {
			Expect(item.Uid).To(Equal("abc123"))
		})

		It("should decode text", func() {
			Expect(item.Text).To(Equal("Buy milk"))
		})

		It("should decode checked", func() {
			Expect(item.Checked).To(BeTrue())
		})

		It("should decode automated", func() {
			Expect(item.Automated).To(BeTrue())
		})

		It("should decode description", func() {
			Expect(item.Description).NotTo(BeNil())
			Expect(*item.Description).To(Equal("do it"))
		})

		It("should decode alarm_payload", func() {
			Expect(item.AlarmPayload).NotTo(BeNil())
			Expect(*item.AlarmPayload).To(Equal("cal:payload"))
		})

		It("should decode due", func() {
			Expect(item.Due).NotTo(BeNil())
		})

		It("should decode created_at from stored value", func() {
			Expect(item.CreatedAt).NotTo(BeNil())
			// created_at was set 1h before codecNow so must differ from codecNow
			Expect(item.CreatedAt.AsTime().Equal(codecNow)).To(BeFalse())
		})

		It("should decode completed_by", func() {
			Expect(item.CompletedBy).NotTo(BeNil())
			Expect(*item.CompletedBy).To(Equal("alice"))
		})
	})

	When("item has no created_at", func() {
		It("should synthesize created_at from now", func() {
			m := map[string]any{"uid": "u", "text": "T"}
			item := decodeItem(m, codecNow)
			Expect(item.CreatedAt).NotTo(BeNil())
			Expect(item.CreatedAt.AsTime().Equal(codecNow)).To(BeTrue())
		})
	})

	When("item has no updated_at", func() {
		It("should synthesize updated_at from now", func() {
			m := map[string]any{"uid": "u", "text": "T"}
			item := decodeItem(m, codecNow)
			Expect(item.UpdatedAt).NotTo(BeNil())
			Expect(item.UpdatedAt.AsTime().Equal(codecNow)).To(BeTrue())
		})
	})

	When("description is empty string", func() {
		It("should leave Description nil", func() {
			m := map[string]any{"uid": "u", "text": "T", "description": ""}
			item := decodeItem(m, codecNow)
			Expect(item.Description).To(BeNil())
		})
	})

	When("alarm_payload is empty string", func() {
		It("should leave AlarmPayload nil", func() {
			m := map[string]any{"uid": "u", "text": "T", "alarm_payload": ""}
			item := decodeItem(m, codecNow)
			Expect(item.AlarmPayload).To(BeNil())
		})
	})
})

var _ = Describe("decodeLegacyItem", func() {
	When("item has text, checked, and description", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			m := map[string]any{
				"text":        "legacy item",
				"checked":     true,
				"description": "legacy desc",
			}
			item = decodeLegacyItem(m, codecNow)
		})

		It("should have empty Uid", func() {
			Expect(item.Uid).To(BeEmpty())
		})

		It("should decode text", func() {
			Expect(item.Text).To(Equal("legacy item"))
		})

		It("should decode checked", func() {
			Expect(item.Checked).To(BeTrue())
		})

		It("should synthesize created_at from now", func() {
			Expect(item.CreatedAt.AsTime().Equal(codecNow)).To(BeTrue())
		})

		It("should decode description", func() {
			Expect(item.Description).NotTo(BeNil())
			Expect(*item.Description).To(Equal("legacy desc"))
		})
	})

	When("description is empty", func() {
		It("should leave Description nil", func() {
			m := map[string]any{"text": "T", "description": ""}
			item := decodeLegacyItem(m, codecNow)
			Expect(item.Description).To(BeNil())
		})
	})
})

var _ = Describe("encodeItem", func() {
	When("item has only required fields", func() {
		var m map[string]any

		BeforeEach(func() {
			item := &apiv1.ChecklistItem{
				Uid:       "u1",
				Text:      "Buy bread",
				Checked:   false,
				SortOrder: int64(1000),
				Automated: false,
			}
			m = encodeItem(item)
		})

		It("should encode uid", func() {
			Expect(m["uid"]).To(Equal("u1"))
		})

		It("should encode text", func() {
			Expect(m["text"]).To(Equal("Buy bread"))
		})

		It("should not include tags when empty", func() {
			Expect(m).NotTo(HaveKey("tags"))
		})

		It("should not include description when nil", func() {
			Expect(m).NotTo(HaveKey("description"))
		})

		It("should not include alarm_payload when nil", func() {
			Expect(m).NotTo(HaveKey("alarm_payload"))
		})

		It("should not include due when nil", func() {
			Expect(m).NotTo(HaveKey("due"))
		})

		It("should not include completed_at when nil", func() {
			Expect(m).NotTo(HaveKey("completed_at"))
		})

		It("should not include completed_by when nil", func() {
			Expect(m).NotTo(HaveKey("completed_by"))
		})
	})

	When("item has all optional fields set", func() {
		var m map[string]any

		BeforeEach(func() {
			desc := "desc"
			alarm := "alarm"
			completedBy := "bob"
			item := &apiv1.ChecklistItem{
				Uid:          "u2",
				Text:         "Task",
				Tags:         []string{"work", "urgent"},
				Description:  &desc,
				AlarmPayload: &alarm,
				Due:          timestamppb.New(codecNow),
				CreatedAt:    timestamppb.New(codecNow),
				UpdatedAt:    timestamppb.New(codecNow),
				CompletedAt:  timestamppb.New(codecNow),
				CompletedBy:  &completedBy,
			}
			m = encodeItem(item)
		})

		It("should encode tags", func() {
			Expect(m).To(HaveKey("tags"))
		})

		It("should encode description", func() {
			Expect(m["description"]).To(Equal("desc"))
		})

		It("should encode alarm_payload", func() {
			Expect(m["alarm_payload"]).To(Equal("alarm"))
		})

		It("should encode due", func() {
			Expect(m).To(HaveKey("due"))
		})

		It("should encode created_at", func() {
			Expect(m).To(HaveKey("created_at"))
		})

		It("should encode completed_at", func() {
			Expect(m).To(HaveKey("completed_at"))
		})

		It("should encode completed_by", func() {
			Expect(m["completed_by"]).To(Equal("bob"))
		})
	})
})

var _ = Describe("encodeTombstones", func() {
	When("tombstones are given", func() {
		It("should sort by deleted_at ascending", func() {
			t1 := codecNow.Add(-time.Hour)
			t2 := codecNow
			tombstones := []*apiv1.Tombstone{
				{Uid: "later", DeletedAt: timestamppb.New(t2), GcAfter: timestamppb.New(t2.Add(TombstoneTTL))},
				{Uid: "earlier", DeletedAt: timestamppb.New(t1), GcAfter: timestamppb.New(t1.Add(TombstoneTTL))},
			}
			encoded := encodeTombstones(tombstones)
			Expect(encoded).To(HaveLen(2))
			first := encoded[0].(map[string]any)
			Expect(first["uid"]).To(Equal("earlier"))
		})
	})

	When("a tombstone has nil GcAfter", func() {
		It("should skip the gc_after field", func() {
			t1 := codecNow
			tombstones := []*apiv1.Tombstone{
				{Uid: "no-gc", DeletedAt: timestamppb.New(t1), GcAfter: nil},
			}
			encoded := encodeTombstones(tombstones)
			m := encoded[0].(map[string]any)
			Expect(m).To(HaveKey("deleted_at"))
			Expect(m).NotTo(HaveKey("gc_after"))
		})
	})

	When("a tombstone has nil DeletedAt", func() {
		It("should skip the deleted_at field", func() {
			tombstones := []*apiv1.Tombstone{
				{Uid: "no-del", DeletedAt: nil, GcAfter: timestamppb.New(codecNow)},
			}
			encoded := encodeTombstones(tombstones)
			m := encoded[0].(map[string]any)
			Expect(m).NotTo(HaveKey("deleted_at"))
			Expect(m).To(HaveKey("gc_after"))
		})
	})
})

var _ = Describe("listChecklistNames", func() {
	When("both wiki and legacy namespaces have lists", func() {
		It("should return the sorted union without duplicates", func() {
			fm := wikipage.FrontMatter{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"shopping": map[string]any{},
						"work":     map[string]any{},
					},
				},
				"checklists": map[string]any{
					"legacy": map[string]any{},
					"work":   map[string]any{},
				},
			}
			names := listChecklistNames(fm)
			Expect(names).To(ConsistOf("legacy", "shopping", "work"))
		})
	})

	When("only wiki namespace has lists", func() {
		It("should return wiki list names sorted", func() {
			fm := wikipage.FrontMatter{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"beta":  map[string]any{},
						"alpha": map[string]any{},
					},
				},
			}
			Expect(listChecklistNames(fm)).To(Equal([]string{"alpha", "beta"}))
		})
	})

	When("only legacy namespace has lists", func() {
		It("should return legacy list names", func() {
			fm := wikipage.FrontMatter{
				"checklists": map[string]any{
					"old": map[string]any{},
				},
			}
			Expect(listChecklistNames(fm)).To(Equal([]string{"old"}))
		})
	})

	When("frontmatter is empty", func() {
		It("should return an empty slice", func() {
			Expect(listChecklistNames(wikipage.FrontMatter{})).To(BeEmpty())
		})
	})
})

var _ = Describe("decodeChecklist", func() {
	When("frontmatter has items in the wiki namespace", func() {
		It("should decode from wiki.checklists.<name>.items[]", func() {
			fm := wikipage.FrontMatter{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"groceries": map[string]any{
							"sync_token": int64(3),
							"updated_at": codecNow.Format(time.RFC3339Nano),
							"items": []any{
								map[string]any{
									"uid": "uid1", "text": "milk", "checked": false,
									"sort_order": int64(1000),
									"created_at": codecNow.Format(time.RFC3339Nano),
									"updated_at": codecNow.Format(time.RFC3339Nano),
								},
							},
						},
					},
				},
			}
			cl := decodeChecklist(fm, "groceries", codecClock)
			Expect(cl.Name).To(Equal("groceries"))
			Expect(cl.SyncToken).To(Equal(int64(3)))
			Expect(cl.Items).To(HaveLen(1))
			Expect(cl.Items[0].Uid).To(Equal("uid1"))
		})
	})

	When("wiki namespace is empty and legacy items exist", func() {
		It("should fall back to legacy items", func() {
			fm := wikipage.FrontMatter{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "bread", "checked": true},
						},
					},
				},
			}
			cl := decodeChecklist(fm, "groceries", codecClock)
			Expect(cl.Items).To(HaveLen(1))
			Expect(cl.Items[0].Text).To(Equal("bread"))
			Expect(cl.Items[0].Uid).To(BeEmpty())
		})
	})

	When("frontmatter has no items anywhere", func() {
		It("should return an empty checklist", func() {
			cl := decodeChecklist(wikipage.FrontMatter{}, "nonexistent", codecClock)
			Expect(cl.Items).To(BeEmpty())
			Expect(cl.Tombstones).To(BeEmpty())
		})
	})

	When("wiki items slice contains a non-map element", func() {
		It("should skip the non-map element", func() {
			fm := wikipage.FrontMatter{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"list": map[string]any{
							"items": []any{
								"not-a-map",
								map[string]any{"uid": "u1", "text": "real",
									"created_at": codecNow.Format(time.RFC3339Nano),
									"updated_at": codecNow.Format(time.RFC3339Nano)},
							},
						},
					},
				},
			}
			cl := decodeChecklist(fm, "list", codecClock)
			Expect(cl.Items).To(HaveLen(1))
			Expect(cl.Items[0].Uid).To(Equal("u1"))
		})
	})

	When("legacy items slice contains a non-map element", func() {
		It("should skip the non-map element", func() {
			fm := wikipage.FrontMatter{
				"checklists": map[string]any{
					"list": map[string]any{
						"items": []any{
							"not-a-map",
							map[string]any{"text": "real"},
						},
					},
				},
			}
			cl := decodeChecklist(fm, "list", codecClock)
			Expect(cl.Items).To(HaveLen(1))
		})
	})

	When("checklist has tombstones", func() {
		It("should decode tombstones", func() {
			fm := wikipage.FrontMatter{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"list": map[string]any{
							"items": []any{},
							"tombstones": []any{
								map[string]any{
									"uid":        "dead",
									"deleted_at": codecNow.Format(time.RFC3339Nano),
									"gc_after":   codecNow.Add(TombstoneTTL).Format(time.RFC3339Nano),
								},
							},
						},
					},
				},
			}
			cl := decodeChecklist(fm, "list", codecClock)
			Expect(cl.Tombstones).To(HaveLen(1))
			Expect(cl.Tombstones[0].Uid).To(Equal("dead"))
		})
	})
})

var _ = Describe("encodeChecklist", func() {
	When("encoding a checklist with items and tombstones", func() {
		var fm wikipage.FrontMatter

		BeforeEach(func() {
			fm = wikipage.FrontMatter{}
			cl := &apiv1.Checklist{
				Name:      "groceries",
				SyncToken: 5,
				UpdatedAt: timestamppb.New(codecNow),
				Items: []*apiv1.ChecklistItem{
					{Uid: "u1", Text: "milk", SortOrder: 1000,
						CreatedAt: timestamppb.New(codecNow),
						UpdatedAt: timestamppb.New(codecNow)},
				},
				Tombstones: []*apiv1.Tombstone{
					{Uid: "gone", DeletedAt: timestamppb.New(codecNow),
						GcAfter: timestamppb.New(codecNow.Add(TombstoneTTL))},
				},
			}
			encodeChecklist(fm, "groceries", cl)
		})

		It("should write items under wiki.checklists.groceries.items", func() {
			wiki := fm["wiki"].(map[string]any)
			checklists := wiki["checklists"].(map[string]any)
			groceries := checklists["groceries"].(map[string]any)
			items := groceries["items"].([]any)
			Expect(items).To(HaveLen(1))
		})

		It("should write sync_token", func() {
			wiki := fm["wiki"].(map[string]any)
			checklists := wiki["checklists"].(map[string]any)
			groceries := checklists["groceries"].(map[string]any)
			Expect(groceries["sync_token"]).To(Equal(int64(5)))
		})

		It("should write updated_at", func() {
			wiki := fm["wiki"].(map[string]any)
			checklists := wiki["checklists"].(map[string]any)
			groceries := checklists["groceries"].(map[string]any)
			Expect(groceries["updated_at"]).NotTo(BeEmpty())
		})

		It("should write tombstones", func() {
			wiki := fm["wiki"].(map[string]any)
			checklists := wiki["checklists"].(map[string]any)
			groceries := checklists["groceries"].(map[string]any)
			Expect(groceries["tombstones"]).NotTo(BeNil())
		})
	})

	When("legacy checklists entry exists only for the encoded list", func() {
		var fm wikipage.FrontMatter

		BeforeEach(func() {
			fm = wikipage.FrontMatter{
				"checklists": map[string]any{
					"groceries": map[string]any{"items": []any{}},
				},
			}
			cl := &apiv1.Checklist{Name: "groceries", SyncToken: 1, UpdatedAt: timestamppb.New(codecNow)}
			encodeChecklist(fm, "groceries", cl)
		})

		It("should remove the checklists key entirely when it becomes empty", func() {
			Expect(fm).NotTo(HaveKey("checklists"))
		})
	})

	When("legacy checklists entry has other lists besides the encoded one", func() {
		var fm wikipage.FrontMatter

		BeforeEach(func() {
			fm = wikipage.FrontMatter{
				"checklists": map[string]any{
					"groceries": map[string]any{"items": []any{}},
					"other":     map[string]any{"items": []any{}},
				},
			}
			cl := &apiv1.Checklist{Name: "groceries", SyncToken: 1, UpdatedAt: timestamppb.New(codecNow)}
			encodeChecklist(fm, "groceries", cl)
		})

		It("should remove only the encoded list from legacy", func() {
			legacy := fm["checklists"].(map[string]any)
			Expect(legacy).NotTo(HaveKey("groceries"))
		})

		It("should preserve other legacy lists", func() {
			legacy := fm["checklists"].(map[string]any)
			Expect(legacy).To(HaveKey("other"))
		})
	})

	When("the checklist has no tombstones", func() {
		It("should delete the tombstones key", func() {
			fm := wikipage.FrontMatter{}
			cl := &apiv1.Checklist{Name: "list", SyncToken: 1, UpdatedAt: timestamppb.New(codecNow)}
			encodeChecklist(fm, "list", cl)
			wiki := fm["wiki"].(map[string]any)
			checklists := wiki["checklists"].(map[string]any)
			list := checklists["list"].(map[string]any)
			Expect(list).NotTo(HaveKey("tombstones"))
		})
	})
})

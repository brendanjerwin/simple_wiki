//revive:disable:dot-imports
package icalcodec_test

import (
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/caldav/icalcodec"
)

func TestIcalcodec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "icalcodec")
}

// fixedNow returns a deterministic clock for DTSTAMP assertions.
func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

var _ = Describe("RenderItem", func() {
	var (
		now      time.Time
		item     *apiv1.ChecklistItem
		page     string
		listName string
		baseURL  string
		body     string
	)

	BeforeEach(func() {
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		page = "shopping"
		listName = "this-week"
		baseURL = "https://wiki.example.com"

		text := "Buy milk"
		desc := "Kirsten likes the kirkland brand"
		item = &apiv1.ChecklistItem{
			Uid:         "01HXAAAAAAAAAAAAAAAAAAAAAA",
			Text:        text,
			Checked:     false,
			Tags:        []string{"urgent", "grocery"},
			SortOrder:   1500,
			Description: &desc,
			CreatedAt:   timestamppb.New(now.Add(-2 * time.Hour)),
			UpdatedAt:   timestamppb.New(now.Add(-30 * time.Minute)),
		}
	})

	When("a basic unchecked item is rendered", func() {
		BeforeEach(func() {
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should produce a non-empty VCALENDAR document", func() {
			Expect(body).NotTo(BeEmpty())
			Expect(body).To(ContainSubstring("BEGIN:VCALENDAR"))
			Expect(body).To(ContainSubstring("END:VCALENDAR"))
		})

		It("should include exactly one VTODO component", func() {
			Expect(strings.Count(body, "BEGIN:VTODO")).To(Equal(1))
			Expect(strings.Count(body, "END:VTODO")).To(Equal(1))
		})

		It("should emit VERSION:2.0", func() {
			Expect(body).To(ContainSubstring("VERSION:2.0"))
		})

		It("should emit a stable PRODID", func() {
			Expect(body).To(ContainSubstring("PRODID:-//simple_wiki//CalDAV//EN"))
		})

		It("should emit the UID", func() {
			Expect(body).To(ContainSubstring("UID:01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should emit SUMMARY from item.Text", func() {
			Expect(body).To(ContainSubstring("SUMMARY:Buy milk"))
		})

		It("should emit STATUS:NEEDS-ACTION when checked is false", func() {
			Expect(body).To(ContainSubstring("STATUS:NEEDS-ACTION"))
		})

		It("should emit PERCENT-COMPLETE:0 for an unchecked item", func() {
			Expect(body).To(ContainSubstring("PERCENT-COMPLETE:0"))
		})

		It("should emit a CATEGORIES line carrying both tags", func() {
			Expect(body).To(MatchRegexp(`CATEGORIES:[^\r\n]*urgent`))
			Expect(body).To(MatchRegexp(`CATEGORIES:[^\r\n]*grocery`))
		})

		It("should emit X-APPLE-SORT-ORDER from item.SortOrder", func() {
			Expect(body).To(ContainSubstring("X-APPLE-SORT-ORDER:1500"))
		})

		It("should emit PRIORITY:0 (undefined) so iOS doesn't render a '!' badge on every item", func() {
			Expect(body).To(MatchRegexp(`(?m)^PRIORITY:0\b`))
		})

		It("should emit a URL property pointing back to the wiki page", func() {
			Expect(body).To(ContainSubstring("URL:https://wiki.example.com/shopping/view"))
		})

		It("should emit DESCRIPTION from item.Description", func() {
			Expect(body).To(ContainSubstring("DESCRIPTION:Kirsten likes the kirkland brand"))
		})

		It("should emit DTSTAMP", func() {
			Expect(body).To(ContainSubstring("DTSTAMP:"))
		})

		It("should emit CREATED from item.CreatedAt", func() {
			Expect(body).To(ContainSubstring("CREATED:"))
		})

		It("should emit LAST-MODIFIED from item.UpdatedAt", func() {
			Expect(body).To(ContainSubstring("LAST-MODIFIED:"))
		})

		It("should NOT include a COMPLETED line for an unchecked item", func() {
			Expect(body).NotTo(ContainSubstring("COMPLETED:"))
		})
	})

	When("the item is checked", func() {
		BeforeEach(func() {
			item.Checked = true
			completedAt := now.Add(-15 * time.Minute)
			item.CompletedAt = timestamppb.New(completedAt)
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should emit STATUS:COMPLETED", func() {
			Expect(body).To(ContainSubstring("STATUS:COMPLETED"))
		})

		It("should emit PERCENT-COMPLETE:100", func() {
			Expect(body).To(ContainSubstring("PERCENT-COMPLETE:100"))
		})

		It("should emit a COMPLETED timestamp", func() {
			Expect(body).To(ContainSubstring("COMPLETED:"))
		})
	})

	When("the item has no tags", func() {
		BeforeEach(func() {
			item.Tags = nil
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should NOT emit a CATEGORIES line", func() {
			Expect(body).NotTo(ContainSubstring("CATEGORIES:"))
		})
	})

	When("the item has no description", func() {
		BeforeEach(func() {
			item.Description = nil
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should NOT emit a DESCRIPTION line", func() {
			Expect(body).NotTo(ContainSubstring("DESCRIPTION:"))
		})
	})

	When("the item has a due date", func() {
		BeforeEach(func() {
			due := time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC)
			item.Due = timestamppb.New(due)
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should emit a DUE property", func() {
			Expect(body).To(ContainSubstring("DUE:"))
		})
	})

	When("baseURL has a trailing slash", func() {
		BeforeEach(func() {
			body = string(icalcodec.RenderItem(item, page, listName, "https://wiki.example.com/", fixedNow(now)))
		})

		It("should still produce a single-slash URL property", func() {
			Expect(body).To(ContainSubstring("URL:https://wiki.example.com/shopping/view"))
		})
	})

	When("sort_order varies", func() {
		// Outbound PRIORITY is intentionally constant 0 (undefined).
		// Ordering rides on X-APPLE-SORT-ORDER so iOS doesn't render
		// a "!" priority badge on items the user never marked.
		It("should still emit PRIORITY:0 for sort_order=1000", func() {
			item.SortOrder = 1000
			out := string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
			Expect(out).To(MatchRegexp(`(?m)^PRIORITY:0\b`))
		})

		It("should still emit PRIORITY:0 for sort_order=9000", func() {
			item.SortOrder = 9000
			out := string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
			Expect(out).To(MatchRegexp(`(?m)^PRIORITY:0\b`))
		})

		It("should emit X-APPLE-SORT-ORDER carrying the actual order", func() {
			item.SortOrder = 5000
			out := string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
			Expect(out).To(ContainSubstring("X-APPLE-SORT-ORDER:5000"))
		})
	})

	When("the item has an alarm_payload", func() {
		BeforeEach(func() {
			alarm := `{"trigger":"-PT15M"}`
			item.AlarmPayload = &alarm
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should embed a VALARM in the VTODO", func() {
			Expect(body).To(ContainSubstring("BEGIN:VALARM"))
			Expect(body).To(ContainSubstring("END:VALARM"))
			Expect(body).To(ContainSubstring("ACTION:DISPLAY"))
			Expect(body).To(ContainSubstring("TRIGGER:-PT15M"))
		})
	})

	When("the item has a malformed alarm_payload", func() {
		BeforeEach(func() {
			alarm := "not json"
			item.AlarmPayload = &alarm
			body = string(icalcodec.RenderItem(item, page, listName, baseURL, fixedNow(now)))
		})

		It("should silently drop the alarm rather than fail the whole item", func() {
			Expect(body).NotTo(ContainSubstring("BEGIN:VALARM"))
			Expect(body).To(ContainSubstring("BEGIN:VTODO"))
		})
	})
})

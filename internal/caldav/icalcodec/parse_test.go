//revive:disable:dot-imports
package icalcodec_test

import (
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav/icalcodec"
)

// buildVTODO assembles a minimal-but-valid VCALENDAR/VTODO body from a
// list of VTODO property lines. Tests pass only the lines they care
// about; helpers below add UID/SUMMARY where the test isn't testing
// their absence.
func buildVTODO(props ...string) []byte {
	parts := []string{
		"BEGIN:VCALENDAR\r\n",
		"VERSION:2.0\r\n",
		"PRODID:-//test//EN\r\n",
		"BEGIN:VTODO\r\n",
	}
	for _, p := range props {
		parts = append(parts, p, "\r\n")
	}
	parts = append(parts, "END:VTODO\r\n", "END:VCALENDAR\r\n")
	return []byte(strings.Join(parts, ""))
}

// withDefaults prepends a UID and SUMMARY line so tests that don't care
// about either still produce a parseable VTODO.
func withDefaults(props ...string) []byte {
	defaults := []string{
		"UID:01HXAAAAAAAAAAAAAAAAAAAAAA",
		"SUMMARY:Buy milk",
	}
	return buildVTODO(append(defaults, props...)...)
}

var _ = Describe("ParseVTODO", func() {
	When("body is empty", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(nil)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("body is malformed iCalendar", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO([]byte("NOT A CALENDAR"))
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("VCALENDAR contains no VTODO", func() {
		var err error

		BeforeEach(func() {
			body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\nEND:VCALENDAR\r\n")
			_, err = icalcodec.ParseVTODO(body)
		})

		It("should return ErrNoVTODO", func() {
			Expect(errors.Is(err, icalcodec.ErrNoVTODO)).To(BeTrue())
		})
	})

	When("VCALENDAR contains a VTIMEZONE but no VTODO", func() {
		var err error

		BeforeEach(func() {
			body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
				"BEGIN:VTIMEZONE\r\nTZID:UTC\r\nEND:VTIMEZONE\r\n" +
				"END:VCALENDAR\r\n")
			_, err = icalcodec.ParseVTODO(body)
		})

		It("should return ErrNoVTODO", func() {
			Expect(errors.Is(err, icalcodec.ErrNoVTODO)).To(BeTrue())
		})
	})

	When("VCALENDAR contains two VTODOs", func() {
		var err error

		BeforeEach(func() {
			body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
				"BEGIN:VTODO\r\nUID:a\r\nSUMMARY:one\r\nEND:VTODO\r\n" +
				"BEGIN:VTODO\r\nUID:b\r\nSUMMARY:two\r\nEND:VTODO\r\n" +
				"END:VCALENDAR\r\n")
			_, err = icalcodec.ParseVTODO(body)
		})

		It("should return ErrMultipleVTODOs", func() {
			Expect(errors.Is(err, icalcodec.ErrMultipleVTODOs)).To(BeTrue())
		})
	})

	When("VTODO has no UID", func() {
		var err error

		BeforeEach(func() {
			body := buildVTODO("SUMMARY:Buy milk")
			_, err = icalcodec.ParseVTODO(body)
		})

		It("should return ErrMissingUID", func() {
			Expect(errors.Is(err, icalcodec.ErrMissingUID)).To(BeTrue())
		})
	})

	When("a basic unchecked VTODO is parsed", func() {
		var (
			parsed icalcodec.ParsedVTODO
			err    error
		)

		BeforeEach(func() {
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"STATUS:NEEDS-ACTION",
			))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate UID", func() {
			Expect(parsed.UID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should populate Text from SUMMARY", func() {
			Expect(parsed.Text).To(Equal("Buy milk"))
		})

		It("should set Checked to false", func() {
			Expect(parsed.Checked).To(BeFalse())
		})

		It("should leave CompletedAt nil", func() {
			Expect(parsed.CompletedAt).To(BeNil())
		})

		It("should leave Description nil", func() {
			Expect(parsed.Description).To(BeNil())
		})

		It("should leave Due nil", func() {
			Expect(parsed.Due).To(BeNil())
		})

		It("should leave AlarmPayload nil", func() {
			Expect(parsed.AlarmPayload).To(BeNil())
		})

		It("should leave SortOrder nil", func() {
			Expect(parsed.SortOrder).To(BeNil())
		})
	})

	When("STATUS is COMPLETED with a COMPLETED timestamp", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"STATUS:COMPLETED",
				"COMPLETED:20260425T120000Z",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set Checked to true", func() {
			Expect(parsed.Checked).To(BeTrue())
		})

		It("should populate CompletedAt", func() {
			Expect(parsed.CompletedAt).NotTo(BeNil())
			Expect(parsed.CompletedAt.UTC()).To(Equal(time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)))
		})
	})

	When("STATUS is given in mixed case", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"STATUS:Completed",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should still recognize the completed state", func() {
			Expect(parsed.Checked).To(BeTrue())
		})
	})

	When("CATEGORIES contains multiple comma-separated tags", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"CATEGORIES:Urgent,GROCERY",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should normalize the tags to lowercase", func() {
			Expect(parsed.Tags).To(ConsistOf("urgent", "grocery"))
		})
	})

	When("DESCRIPTION contains inline #tags", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"DESCRIPTION:buy #milk and #eggs at the store",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should extract milk into Tags", func() {
			Expect(parsed.Tags).To(ContainElement("milk"))
		})

		It("should extract eggs into Tags", func() {
			Expect(parsed.Tags).To(ContainElement("eggs"))
		})
	})

	When("CATEGORIES and DESCRIPTION both contribute tags", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"CATEGORIES:urgent",
				"DESCRIPTION:remember the #milk",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should union both sources", func() {
			Expect(parsed.Tags).To(ConsistOf("urgent", "milk"))
		})
	})

	When("CATEGORIES and DESCRIPTION repeat the same tag", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"CATEGORIES:urgent",
				"DESCRIPTION:this is #urgent",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should deduplicate the tag", func() {
			Expect(parsed.Tags).To(ConsistOf("urgent"))
		})
	})

	When("X-APPLE-SORT-ORDER is present", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"X-APPLE-SORT-ORDER:1500",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate SortOrder from the X-APPLE value", func() {
			Expect(parsed.SortOrder).NotTo(BeNil())
			Expect(*parsed.SortOrder).To(Equal(int64(1500)))
		})
	})

	When("only PRIORITY is present (no X-APPLE-SORT-ORDER)", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"PRIORITY:3",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should map PRIORITY:3 to sort_order 3000 so it round-trips with the renderer", func() {
			Expect(parsed.SortOrder).NotTo(BeNil())
			Expect(*parsed.SortOrder).To(Equal(int64(3000)))
		})
	})

	When("PRIORITY:0 (RFC 5545 'undefined') is present without X-APPLE-SORT-ORDER", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"PRIORITY:0",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should leave SortOrder nil so the mutator preserves the existing order", func() {
			Expect(parsed.SortOrder).To(BeNil())
		})
	})

	When("both X-APPLE-SORT-ORDER and PRIORITY are present", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"X-APPLE-SORT-ORDER:9000",
				"PRIORITY:5",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prefer X-APPLE-SORT-ORDER", func() {
			Expect(parsed.SortOrder).NotTo(BeNil())
			Expect(*parsed.SortOrder).To(Equal(int64(9000)))
		})
	})

	When("X-APPLE-SORT-ORDER is non-numeric and PRIORITY is absent", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"X-APPLE-SORT-ORDER:not-a-number",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should leave SortOrder nil", func() {
			Expect(parsed.SortOrder).To(BeNil())
		})
	})

	When("DESCRIPTION is within the size cap", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"DESCRIPTION:hello world",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate Description", func() {
			Expect(parsed.Description).NotTo(BeNil())
			Expect(*parsed.Description).To(Equal("hello world"))
		})
	})

	When("DESCRIPTION exceeds the 64KB cap", func() {
		var err error

		BeforeEach(func() {
			huge := strings.Repeat("a", icalcodec.DescriptionMaxBytes+1)
			body := withDefaults("DESCRIPTION:" + huge)
			_, err = icalcodec.ParseVTODO(body)
		})

		It("should return ErrDescriptionTooLarge", func() {
			Expect(errors.Is(err, icalcodec.ErrDescriptionTooLarge)).To(BeTrue())
		})
	})

	When("DESCRIPTION raw wire value exceeds the cap because of escape sequences but the unescaped text fits", func() {
		var (
			parsed icalcodec.ParsedVTODO
			err    error
		)

		BeforeEach(func() {
			// Each "\n" escape is 2 wire bytes for 1 unescaped byte. A
			// payload of ~half the cap of "\n" pairs has unescaped text
			// well under the cap but raw wire length over it.
			pair := `\n`
			repeats := (icalcodec.DescriptionMaxBytes / 2) + 1
			body := withDefaults("DESCRIPTION:" + strings.Repeat(pair, repeats))
			parsed, err = icalcodec.ParseVTODO(body)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate the description", func() {
			Expect(parsed.Description).NotTo(BeNil())
		})
	})

	When("DUE is set", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"DUE:20260501T170000Z",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate Due", func() {
			Expect(parsed.Due).NotTo(BeNil())
			Expect(parsed.Due.UTC()).To(Equal(time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC)))
		})
	})

	When("CREATED is set", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			var err error
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"CREATED:20260420T100000Z",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate Created", func() {
			Expect(parsed.Created).NotTo(BeNil())
			Expect(parsed.Created.UTC()).To(Equal(time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)))
		})
	})

	When("a VALARM child is present", func() {
		var parsed icalcodec.ParsedVTODO

		BeforeEach(func() {
			body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
				"BEGIN:VTODO\r\nUID:01HXAAAAAAAAAAAAAAAAAAAAAA\r\nSUMMARY:Buy milk\r\n" +
				"BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Buy milk\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\n" +
				"END:VTODO\r\nEND:VCALENDAR\r\n")
			var err error
			parsed, err = icalcodec.ParseVTODO(body)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should populate AlarmPayload", func() {
			Expect(parsed.AlarmPayload).NotTo(BeNil())
		})

		It("should encode the alarm as JSON containing the trigger", func() {
			Expect(*parsed.AlarmPayload).To(ContainSubstring(`"trigger":"-PT15M"`))
		})
	})

	When("RRULE is present", func() {
		var (
			parsed icalcodec.ParsedVTODO
			err    error
		)

		BeforeEach(func() {
			parsed, err = icalcodec.ParseVTODO(withDefaults(
				"RRULE:FREQ=DAILY;COUNT=5",
			))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should still populate UID", func() {
			Expect(parsed.UID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})
	})

	When("GEO is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"GEO:37.7749;-122.4194",
			))
		})

		It("should silently strip GEO without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("RELATED-TO is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"RELATED-TO:other-uid",
			))
		})

		It("should silently strip RELATED-TO without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("LOCATION is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"LOCATION:Kitchen",
			))
		})

		It("should silently strip LOCATION without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("CLASS is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"CLASS:PRIVATE",
			))
		})

		It("should silently strip CLASS without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("ORGANIZER is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"ORGANIZER:mailto:someone@example.com",
			))
		})

		It("should silently strip ORGANIZER without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("ATTENDEE is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"ATTENDEE:mailto:other@example.com",
			))
		})

		It("should silently strip ATTENDEE without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("an unknown X-* property is present", func() {
		var err error

		BeforeEach(func() {
			_, err = icalcodec.ParseVTODO(withDefaults(
				"X-WHATEVER:something",
			))
		})

		It("should silently strip the X- property without returning an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

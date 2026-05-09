//revive:disable:dot-imports
package translator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks/translator"
)

var _ = Describe("WikiUIDMarker", func() {
	When("uid is empty", func() {
		var marker string

		BeforeEach(func() {
			marker = translator.WikiUIDMarker("")
		})

		It("should return empty string", func() {
			Expect(marker).To(Equal(""))
		})
	})

	When("uid is a ULID", func() {
		var marker string

		BeforeEach(func() {
			marker = translator.WikiUIDMarker("01HXXXXXXXXXXXXXXXXXXXXXXX")
		})

		It("should start with a newline", func() {
			Expect(marker).To(HavePrefix("\n"))
		})

		It("should contain the uid", func() {
			Expect(marker).To(ContainSubstring("01HXXXXXXXXXXXXXXXXXXXXXXX"))
		})

		It("should embed the wiki:uid= sentinel", func() {
			Expect(marker).To(ContainSubstring("wiki:uid=01HXXXXXXXXXXXXXXXXXXXXXXX"))
		})

		It("should embed the zero-width prefix character", func() {
			// U+200B ZERO WIDTH SPACE renders invisibly so the marker
			// line doesn't visually clutter the Tasks UI.
			Expect(marker).To(ContainSubstring("\u200b"))
		})
	})
})

var _ = Describe("StripWikiUIDMarker", func() {
	When("notes contain a trailing wiki uid marker", func() {
		var (
			notes   string
			cleaned string
			uid     string
			found   bool
		)

		BeforeEach(func() {
			notes = "fresh local" + translator.WikiUIDMarker("01HABC")
			cleaned, uid, found = translator.StripWikiUIDMarker(notes)
		})

		It("should report found", func() {
			Expect(found).To(BeTrue())
		})

		It("should return cleaned notes without the marker", func() {
			Expect(cleaned).To(Equal("fresh local"))
		})

		It("should extract the uid", func() {
			Expect(uid).To(Equal("01HABC"))
		})
	})

	When("notes have no marker", func() {
		var (
			cleaned string
			uid     string
			found   bool
		)

		BeforeEach(func() {
			cleaned, uid, found = translator.StripWikiUIDMarker("just a description")
		})

		It("should report not found", func() {
			Expect(found).To(BeFalse())
		})

		It("should pass notes through unchanged", func() {
			Expect(cleaned).To(Equal("just a description"))
		})

		It("should return empty uid", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("notes are empty", func() {
		var (
			cleaned string
			uid     string
			found   bool
		)

		BeforeEach(func() {
			cleaned, uid, found = translator.StripWikiUIDMarker("")
		})

		It("should report not found", func() {
			Expect(found).To(BeFalse())
		})

		It("should return empty cleaned", func() {
			Expect(cleaned).To(Equal(""))
		})

		It("should return empty uid", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("a marker appears mid-notes followed by user content", func() {
		var (
			notes      string
			cleaned    string
			extractedUID string
			found      bool
		)

		BeforeEach(func() {
			// User pasted a wiki excerpt into the description and
			// then kept typing — we must not strip a non-trailing
			// marker, because doing so would corrupt user content.
			notes = "context: example" + translator.WikiUIDMarker("BOGUS") + "\nuser kept typing here"
			cleaned, extractedUID, found = translator.StripWikiUIDMarker(notes)
		})

		It("should report not found", func() {
			Expect(found).To(BeFalse())
		})

		It("should leave notes unchanged", func() {
			Expect(cleaned).To(Equal(notes))
		})

		It("should not extract a uid", func() {
			Expect(extractedUID).To(Equal(""))
		})
	})
})

//revive:disable:dot-imports
package icalcodec_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/emersion/go-ical"

	"github.com/brendanjerwin/simple_wiki/internal/caldav/icalcodec"
)

var _ = Describe("RenderAlarm", func() {
	When("payload is empty", func() {
		It("should return an error", func() {
			_, err := icalcodec.RenderAlarm("", "fallback")
			Expect(err).To(HaveOccurred())
		})
	})

	When("payload is malformed JSON", func() {
		It("should return an error", func() {
			_, err := icalcodec.RenderAlarm("not json", "fallback")
			Expect(err).To(HaveOccurred())
		})
	})

	When("payload has no trigger", func() {
		It("should return an error", func() {
			_, err := icalcodec.RenderAlarm(`{"description":"x"}`, "fallback")
			Expect(err).To(HaveOccurred())
		})
	})

	When("payload is a relative duration", func() {
		var alarm *ical.Component

		BeforeEach(func() {
			var err error
			alarm, err = icalcodec.RenderAlarm(`{"trigger":"-PT15M"}`, "Buy milk")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce a VALARM component", func() {
			Expect(alarm.Name).To(Equal(ical.CompAlarm))
		})

		It("should emit ACTION:DISPLAY", func() {
			action, err := alarm.Props.Text(ical.PropAction)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).To(Equal("DISPLAY"))
		})

		It("should set DESCRIPTION from fallback when payload omits it", func() {
			desc, err := alarm.Props.Text(ical.PropDescription)
			Expect(err).NotTo(HaveOccurred())
			Expect(desc).To(Equal("Buy milk"))
		})

		It("should emit a TRIGGER property with the duration value", func() {
			trig := alarm.Props.Get(ical.PropTrigger)
			Expect(trig).NotTo(BeNil())
			Expect(trig.Value).To(Equal("-PT15M"))
		})
	})

	When("payload includes an explicit description", func() {
		It("should emit that description, not the fallback", func() {
			alarm, err := icalcodec.RenderAlarm(`{"trigger":"-PT15M","description":"Heads up"}`, "Buy milk")
			Expect(err).NotTo(HaveOccurred())
			desc, err := alarm.Props.Text(ical.PropDescription)
			Expect(err).NotTo(HaveOccurred())
			Expect(desc).To(Equal("Heads up"))
		})
	})

	When("payload is an absolute timestamp", func() {
		var alarm *ical.Component

		BeforeEach(func() {
			var err error
			alarm, err = icalcodec.RenderAlarm(`{"trigger":"2026-04-30T17:00:00Z","absolute":true}`, "Buy milk")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should emit TRIGGER with VALUE=DATE-TIME", func() {
			trig := alarm.Props.Get(ical.PropTrigger)
			Expect(trig).NotTo(BeNil())
			Expect(trig.Params.Get(ical.ParamValue)).To(Equal(string(ical.ValueDateTime)))
		})

		It("should serialize the value in iCal datetime form", func() {
			trig := alarm.Props.Get(ical.PropTrigger)
			Expect(trig.Value).To(Equal("20260430T170000Z"))
		})
	})

	When("payload is an absolute trigger but the timestamp is malformed", func() {
		It("should return an error", func() {
			_, err := icalcodec.RenderAlarm(`{"trigger":"not-a-time","absolute":true}`, "Buy milk")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("ParseAlarm", func() {
	When("alarm is nil", func() {
		It("should return an error", func() {
			_, err := icalcodec.ParseAlarm(nil)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the component is not a VALARM", func() {
		It("should return an error", func() {
			comp := ical.NewComponent("VEVENT")
			_, err := icalcodec.ParseAlarm(comp)
			Expect(err).To(HaveOccurred())
		})
	})

	When("VALARM has no TRIGGER", func() {
		It("should return an error", func() {
			comp := ical.NewComponent(ical.CompAlarm)
			_, err := icalcodec.ParseAlarm(comp)
			Expect(err).To(HaveOccurred())
		})
	})

	When("VALARM has a relative duration TRIGGER", func() {
		It("should round-trip the payload", func() {
			rendered, err := icalcodec.RenderAlarm(`{"trigger":"-PT15M"}`, "Buy milk")
			Expect(err).NotTo(HaveOccurred())
			payload, err := icalcodec.ParseAlarm(rendered)
			Expect(err).NotTo(HaveOccurred())
			Expect(payload).To(MatchJSON(`{"trigger":"-PT15M","description":"Buy milk"}`))
		})
	})

	When("VALARM has an absolute TRIGGER", func() {
		It("should round-trip the payload with absolute=true", func() {
			rendered, err := icalcodec.RenderAlarm(`{"trigger":"2026-04-30T17:00:00Z","absolute":true,"description":"Heads up"}`, "")
			Expect(err).NotTo(HaveOccurred())
			payload, err := icalcodec.ParseAlarm(rendered)
			Expect(err).NotTo(HaveOccurred())
			Expect(payload).To(MatchJSON(`{"trigger":"2026-04-30T17:00:00Z","absolute":true,"description":"Heads up"}`))
		})
	})
})

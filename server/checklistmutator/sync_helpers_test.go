//revive:disable:dot-imports
package checklistmutator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("syncIdentityFor", func() {
	When("the binding owner email is provided", func() {
		identity := syncIdentityFor("alice@example.com")

		It("should set Name() to the owner email so completed_by surfaces it", func() {
			Expect(identity.Name()).To(Equal("alice@example.com"))
		})

		It("should report IsAgent()=false so the checklist UI does not collapse to 'an agent'", func() {
			Expect(identity.IsAgent()).To(BeFalse())
		})

		It("should report IsAnonymous()=false so the upsert path treats it as a real principal", func() {
			Expect(identity.IsAnonymous()).To(BeFalse())
		})
	})

	When("the binding owner email is empty", func() {
		identity := syncIdentityFor("")

		It("should fall back to the system loginName so callers still get a stable string", func() {
			Expect(identity.Name()).To(Equal("system:keep-sync"))
		})

		It("should still report IsAgent()=false (the cron is transport, not actor)", func() {
			Expect(identity.IsAgent()).To(BeFalse())
		})
	})
})

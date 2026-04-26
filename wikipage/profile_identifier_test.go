package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("ProfileIdentifierFor", func() {
	When("login is a typical email address", func() {
		var (
			id  wikipage.PageIdentifier
			err error
		)

		BeforeEach(func() {
			id, err = wikipage.ProfileIdentifierFor("brendanjerwin@gmail.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should sanitize the address into a single-segment identifier", func() {
			Expect(id).To(Equal(wikipage.PageIdentifier("profile_brendanjerwin_gmail_com")))
		})
	})

	When("login contains '+' tagging and a multi-part TLD", func() {
		var (
			id  wikipage.PageIdentifier
			err error
		)

		BeforeEach(func() {
			id, err = wikipage.ProfileIdentifierFor("alice+tag@example.co.uk")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should collapse all non-alphanumerics to single underscores", func() {
			Expect(id).To(Equal(wikipage.PageIdentifier("profile_alice_tag_example_co_uk")))
		})
	})

	When("login has mixed case", func() {
		var (
			id  wikipage.PageIdentifier
			err error
		)

		BeforeEach(func() {
			id, err = wikipage.ProfileIdentifierFor("Bob.SMITH@Example.com")
		})

		It("should lowercase the result", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(wikipage.PageIdentifier("profile_bob_smith_example_com")))
		})
	})

	When("login is empty", func() {
		var (
			id  wikipage.PageIdentifier
			err error
		)

		BeforeEach(func() {
			id, err = wikipage.ProfileIdentifierFor("")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty identifier", func() {
			Expect(id).To(BeEmpty())
		})
	})

	When("login contains only non-alphanumeric characters", func() {
		var (
			id  wikipage.PageIdentifier
			err error
		)

		BeforeEach(func() {
			id, err = wikipage.ProfileIdentifierFor("@@@...")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty identifier", func() {
			Expect(id).To(BeEmpty())
		})
	})
})

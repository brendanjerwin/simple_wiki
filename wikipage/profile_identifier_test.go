package wikipage_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// hashSuffixHexLen mirrors profileIdentifierHashLen in the production code.
// Defined locally so the test isn't reaching into an unexported constant —
// the value is the actual API contract (the visible URL suffix length).
const hashSuffixHexLen = 8

// expectedHashSuffix mirrors the hash construction in ProfileIdentifierFor
// so the test expectations track the implementation by construction rather
// than by hard-coded magic strings.
func expectedHashSuffix(login string) string {
	digest := sha256.Sum256([]byte(login))
	return hex.EncodeToString(digest[:])[:hashSuffixHexLen]
}

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

		It("should sanitize the address into a single-segment identifier with hash suffix", func() {
			expected := "profile_brendanjerwin_gmail_com_" + expectedHashSuffix("brendanjerwin@gmail.com")
			Expect(id).To(Equal(wikipage.PageIdentifier(expected)))
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

		It("should collapse all non-alphanumerics to single underscores and append the hash", func() {
			expected := "profile_alice_tag_example_co_uk_" + expectedHashSuffix("alice+tag@example.co.uk")
			Expect(id).To(Equal(wikipage.PageIdentifier(expected)))
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

		It("should lowercase the sanitized portion (hash is over the original login)", func() {
			Expect(err).NotTo(HaveOccurred())
			expected := "profile_bob_smith_example_com_" + expectedHashSuffix("Bob.SMITH@Example.com")
			Expect(id).To(Equal(wikipage.PageIdentifier(expected)))
		})
	})

	When("two distinct logins sanitize to the same string", func() {
		var (
			a, b wikipage.PageIdentifier
		)

		BeforeEach(func() {
			// Both collapse to "user_example_com" before the hash; the hash
			// disambiguates them so they don't share a profile page.
			var err error
			a, err = wikipage.ProfileIdentifierFor("user@example.com")
			Expect(err).NotTo(HaveOccurred())
			b, err = wikipage.ProfileIdentifierFor("user.example@com")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce different identifiers", func() {
			Expect(a).NotTo(Equal(b))
		})

		It("should keep the same sanitized stem", func() {
			stem := "profile_user_example_com_"
			Expect(string(a)).To(HavePrefix(stem))
			Expect(string(b)).To(HavePrefix(stem))
		})
	})

	When("the login is the literal string \"template\"", func() {
		var id wikipage.PageIdentifier

		BeforeEach(func() {
			var err error
			id, err = wikipage.ProfileIdentifierFor("template")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not collide with the shipped profile_template system page", func() {
			Expect(string(id)).NotTo(Equal("profile_template"))
		})

		It("should still carry the profile_ prefix", func() {
			Expect(string(id)).To(HavePrefix("profile_template_"))
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

	When("the produced identifier is inspected for shape", func() {
		It("should never contain the prefix as the entire string (defensive)", func() {
			id, err := wikipage.ProfileIdentifierFor("a")
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimPrefix(string(id), "profile_")).NotTo(BeEmpty())
		})
	})
})

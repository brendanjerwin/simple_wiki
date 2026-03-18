//revive:disable:dot-imports
package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGenerateCookieSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("generateRandomCookieSecret", func() {
	It("should return a 64-character lowercase hex string (32 bytes)", func() {
		secret, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())
		Expect(secret).To(MatchRegexp(`^[0-9a-f]{64}$`))
	})

	It("should return different secrets on two successive calls", func() {
		secret1, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())

		secret2, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())

		Expect(secret1).NotTo(Equal(secret2))
	})
})

var _ = Describe("resolveCookieSecret", func() {
	When("a non-empty secret is provided", func() {
		It("should return the provided secret unchanged and generated=false", func() {
			secret, generated, err := resolveCookieSecret("my-explicit-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal("my-explicit-secret"))
			Expect(generated).To(BeFalse())
		})
	})

	When("an empty secret is provided", func() {
		It("should return a valid random secret and generated=true", func() {
			secret, generated, err := resolveCookieSecret("")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(generated).To(BeTrue())
		})
	})
})

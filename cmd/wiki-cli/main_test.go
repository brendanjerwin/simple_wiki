package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestWikiCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "wiki-cli Suite")
}

var _ = Describe("commitsMatch", func() {

	When("the server reports a raw hash matching the CLI commit", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "adbef9d2abc123")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the server reports a tagged format like 'v3.5.0 (adbef9d)'", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123def456", "v3.5.0 (adbef9d)")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the server reports a short hash that is a prefix of the CLI commit", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123def456", "adbef9d2")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the CLI commit is shorter than the server hash", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2", "adbef9d2abc123def456")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the commits do not match at all", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "ffff1234567890")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the server reports a tagged format with a non-matching hash", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "v3.5.0 (ffff123)")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the server reports a tagged format with no closing parenthesis", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "v3.5.0 (adbef9d")
		})

		It("should fall back to full string comparison and return false", func() {
			Expect(result).To(BeFalse())
		})
	})
})

package base32tools_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Base32Tools Suite")
}

var _ = Describe("Base32Tools", func() {
	Describe("Base32 encoding/decoding", func() {
		Describe("EncodeToBase32", func() {
			var (
				input  string
				result string
			)

			BeforeEach(func() {
				input = "hello"
				result = base32tools.EncodeToBase32(input)
			})

			It("should encode a string to base32", func() {
				Expect(result).To(Equal("NBSWY3DP"))
			})
		})

		Describe("DecodeFromBase32", func() {
			var (
				input  string
				result string
				err    error
			)

			BeforeEach(func() {
				input = "NBSWY3DP"
				result, err = base32tools.DecodeFromBase32(input)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should decode a base32 string", func() {
				Expect(result).To(Equal("hello"))
			})
		})
	})
})

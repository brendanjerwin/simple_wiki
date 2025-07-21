package base32tools_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/internal/testutils"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	testutils.EnforceDevboxInCI()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Base32Tools Suite")
}

var _ = Describe("Base32Tools", func() {
	Describe("Base32 encoding/decoding", func() {
		Describe("EncodeToBase32", func() {
			It("should encode a string to base32", func() {
				Expect(base32tools.EncodeToBase32("hello")).To(Equal("NBSWY3DP"))
			})
		})
		Describe("DecodeFromBase32", func() {
			It("should decode a base32 string", func() {
				str, err := base32tools.DecodeFromBase32("NBSWY3DP")
				Expect(err).NotTo(HaveOccurred())
				Expect(str).To(Equal("hello"))
			})
		})
	})
})

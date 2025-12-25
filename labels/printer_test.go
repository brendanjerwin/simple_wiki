//revive:disable:dot-imports
package labels_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("ParseConnectivityMode", func() {
	Context("with valid modes", func() {
		It("should parse USB mode", func() {
			mode, err := labels.ParseConnectivityMode("USB")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal(labels.USB))
		})

		It("should parse usb mode (lowercase)", func() {
			mode, err := labels.ParseConnectivityMode("usb")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal(labels.USB))
		})

		It("should parse LP mode", func() {
			mode, err := labels.ParseConnectivityMode("LP")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal(labels.LP))
		})

		It("should parse lp mode (lowercase)", func() {
			mode, err := labels.ParseConnectivityMode("lp")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal(labels.LP))
		})
	})

	Context("with invalid modes", func() {
		It("should return error for invalid mode", func() {
			_, err := labels.ParseConnectivityMode("invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid connectivity mode"))
		})

		It("should return error for empty string", func() {
			_, err := labels.ParseConnectivityMode("")
			Expect(err).To(HaveOccurred())
		})
	})
})

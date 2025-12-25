//revive:disable:dot-imports
package labels_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("ParseConnectivityMode", func() {
	Context("with valid modes", func() {
		When("parsing USB mode", func() {
			var mode labels.ConnectivityMode
			var err error

			BeforeEach(func() {
				mode, err = labels.ParseConnectivityMode("USB")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return USB mode", func() {
				Expect(mode).To(Equal(labels.USB))
			})
		})

		When("parsing usb mode (lowercase)", func() {
			var mode labels.ConnectivityMode
			var err error

			BeforeEach(func() {
				mode, err = labels.ParseConnectivityMode("usb")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return USB mode", func() {
				Expect(mode).To(Equal(labels.USB))
			})
		})

		When("parsing LP mode", func() {
			var mode labels.ConnectivityMode
			var err error

			BeforeEach(func() {
				mode, err = labels.ParseConnectivityMode("LP")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return LP mode", func() {
				Expect(mode).To(Equal(labels.LP))
			})
		})

		When("parsing lp mode (lowercase)", func() {
			var mode labels.ConnectivityMode
			var err error

			BeforeEach(func() {
				mode, err = labels.ParseConnectivityMode("lp")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return LP mode", func() {
				Expect(mode).To(Equal(labels.LP))
			})
		})
	})

	Context("with invalid modes", func() {
		When("parsing invalid mode", func() {
			var err error

			BeforeEach(func() {
				_, err = labels.ParseConnectivityMode("invalid")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include error details", func() {
				Expect(err.Error()).To(ContainSubstring("invalid connectivity mode"))
			})
		})

		When("parsing empty string", func() {
			var err error

			BeforeEach(func() {
				_, err = labels.ParseConnectivityMode("")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

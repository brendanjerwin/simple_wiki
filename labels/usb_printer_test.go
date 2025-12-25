//revive:disable:dot-imports
package labels_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("USBPrinter", func() {
	Describe("GetUSBPrinter", func() {
		Context("when vendor ID is not specified", func() {
			It("should return ErrorVendorNotSpecified", func() {
				config := labels.PrinterConfig{
					ConnectivityMode: labels.USB,
					USBVendor:        0,
					USBProduct:       0x0001,
				}

				_, err := labels.GetUSBPrinter(config)
				Expect(err).To(Equal(labels.ErrorVendorNotSpecified))
			})
		})

		Context("with valid vendor ID", func() {
			It("should not return vendor error", func() {
				config := labels.PrinterConfig{
					ConnectivityMode: labels.USB,
					USBVendor:        0x0A5F,
					USBProduct:       0x0001,
				}

				// This will likely fail because no USB device is connected
				_, err := labels.GetUSBPrinter(config)
				
				// If there's an error, it should not be the vendor error
				// If no error, that's also fine (device found)
				if err != nil {
					Expect(err).NotTo(Equal(labels.ErrorVendorNotSpecified))
				}
			})
		})
	})
})

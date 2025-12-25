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
				
				// This test validates vendor ID parameter handling specifically.
				// The assertion verifies that if an error occurs, it's NOT due to missing vendor ID.
				// In hardware-dependent tests, both nil (device found) and other errors (device issues)
				// are acceptable outcomes - we only care that the vendor ID parameter was processed correctly.
				// This conditional pattern is intentional for hardware-dependent validation.
				if err != nil {
					Expect(err).NotTo(Equal(labels.ErrorVendorNotSpecified))
				}
			})
		})
	})
})

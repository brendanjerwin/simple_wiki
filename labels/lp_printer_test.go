//revive:disable:dot-imports
package labels_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("LPPrinter", func() {
	Describe("GetLPPrinter", func() {
		var config labels.PrinterConfig
		var printer labels.Printer
		var err error

		BeforeEach(func() {
			config = labels.PrinterConfig{
				ConnectivityMode: labels.LP,
				LPPrinterName:    "test_printer",
			}
			printer, err = labels.GetLPPrinter(config)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a printer instance", func() {
			Expect(printer).NotTo(BeNil())
		})
	})

	Describe("Close", func() {
		var config labels.PrinterConfig
		var printer labels.Printer
		var setupErr error
		var closeErr error

		BeforeEach(func() {
			config = labels.PrinterConfig{
				ConnectivityMode: labels.LP,
				LPPrinterName:    "test_printer",
			}
			printer, setupErr = labels.GetLPPrinter(config)
			
			// Only proceed if setup succeeded
			if setupErr == nil {
				closeErr = printer.Close()
			}
		})

		It("should not error during setup", func() {
			Expect(setupErr).NotTo(HaveOccurred())
		})

		It("should close without error", func() {
			Expect(closeErr).NotTo(HaveOccurred())
		})
	})
})

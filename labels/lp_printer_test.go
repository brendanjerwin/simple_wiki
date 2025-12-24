//revive:disable:dot-imports
package labels_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("LPPrinter", func() {
	Describe("GetLPPrinter", func() {
		It("should create LP printer with given name", func() {
			config := labels.PrinterConfig{
				ConnectivityMode: labels.LP,
				LPPrinterName:    "test_printer",
			}

			printer, err := labels.GetLPPrinter(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(printer).NotTo(BeNil())
		})
	})

	Describe("Close", func() {
		It("should close without error", func() {
			config := labels.PrinterConfig{
				ConnectivityMode: labels.LP,
				LPPrinterName:    "test_printer",
			}

			printer, err := labels.GetLPPrinter(config)
			Expect(err).NotTo(HaveOccurred())

			err = printer.Close()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

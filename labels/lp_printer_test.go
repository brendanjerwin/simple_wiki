//revive:disable:dot-imports
package labels_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/labels"
)

var _ = Describe("LPPrinter", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "lp_printer_test_*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { os.RemoveAll(tmpDir) })
	})

	// setPath temporarily replaces PATH and schedules restoration via DeferCleanup.
	setPath := func(newPath string) {
		original := os.Getenv("PATH")
		Expect(os.Setenv("PATH", newPath)).To(Succeed())
		DeferCleanup(func() { Expect(os.Setenv("PATH", original)).To(Succeed()) })
	}

	// prependTmpDir prepends tmpDir to PATH so fake executables placed there take priority.
	prependTmpDir := func() {
		setPath(tmpDir + ":" + os.Getenv("PATH"))
	}

	Describe("GetLPPrinter", func() {
		When("lp is not in PATH", func() {
			BeforeEach(func() {
				setPath("")
			})

			It("should return an error containing 'lp not found'", func() {
				config := labels.PrinterConfig{
					ConnectivityMode: labels.LP,
					LPPrinterName:    "test_printer",
				}
				_, err := labels.GetLPPrinter(config)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("lp not found"))
			})
		})

		When("lp is in PATH", func() {
			BeforeEach(func() {
				fakeLp := filepath.Join(tmpDir, "lp")
				err := os.WriteFile(fakeLp, []byte("#!/bin/sh\n"), 0755)
				Expect(err).NotTo(HaveOccurred())
				prependTmpDir()
			})

			It("should return a non-nil printer without error", func() {
				config := labels.PrinterConfig{
					ConnectivityMode: labels.LP,
					LPPrinterName:    "test_printer",
				}
				printer, err := labels.GetLPPrinter(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(printer).NotTo(BeNil())
			})
		})
	})

	Describe("Write", func() {
		When("lp runs successfully", func() {
			var (
				printer labels.Printer
				logFile string
			)

			BeforeEach(func() {
				logFile = filepath.Join(tmpDir, "stdin.log")
				fakeLp := filepath.Join(tmpDir, "lp")
				script := "#!/bin/sh\ncat > " + logFile + "\n"
				err := os.WriteFile(fakeLp, []byte(script), 0755)
				Expect(err).NotTo(HaveOccurred())
				prependTmpDir()

				printer, err = labels.GetLPPrinter(labels.PrinterConfig{LPPrinterName: "test_printer"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the number of bytes written without error", func() {
				data := []byte("print job data")
				n, err := printer.Write(data)
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(Equal(len(data)))
			})

			It("should pass data to lp via stdin", func() {
				data := []byte("print job data")
				_, err := printer.Write(data)
				Expect(err).NotTo(HaveOccurred())

				content, err := os.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(data))
			})
		})

		When("lp exits with a non-zero status", func() {
			var printer labels.Printer

			BeforeEach(func() {
				fakeLp := filepath.Join(tmpDir, "lp")
				err := os.WriteFile(fakeLp, []byte("#!/bin/sh\nexit 1\n"), 0755)
				Expect(err).NotTo(HaveOccurred())
				prependTmpDir()

				printer, err = labels.GetLPPrinter(labels.PrinterConfig{LPPrinterName: "test_printer"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error", func() {
				_, err := printer.Write([]byte("data"))
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Close", func() {
		BeforeEach(func() {
			fakeLp := filepath.Join(tmpDir, "lp")
			err := os.WriteFile(fakeLp, []byte("#!/bin/sh\n"), 0755)
			Expect(err).NotTo(HaveOccurred())
			prependTmpDir()
		})

		It("should close without error", func() {
			printer, err := labels.GetLPPrinter(labels.PrinterConfig{LPPrinterName: "test_printer"})
			Expect(err).NotTo(HaveOccurred())
			Expect(printer.Close()).NotTo(HaveOccurred())
		})
	})
})

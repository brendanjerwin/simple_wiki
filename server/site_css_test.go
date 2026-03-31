//revive:disable:dot-imports
package server

import (
	"os"
	"path"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoadCustomCSS", func() {
	var s *Site
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "css-test")
		Expect(err).NotTo(HaveOccurred())
		s = &Site{
			Logger: lumber.NewConsoleLogger(lumber.WARN),
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	When("cssFile is empty", func() {
		var err error

		BeforeEach(func() {
			err = s.LoadCustomCSS("")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not modify s.CSS", func() {
			Expect(s.CSS).To(BeNil())
		})
	})

	When("the CSS file does not exist", func() {
		var err error

		BeforeEach(func() {
			err = s.LoadCustomCSS("/nonexistent/path/custom.css")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an error wrapping the read failure", func() {
			Expect(err.Error()).To(ContainSubstring("failed to read CSS file"))
		})

		It("should return an error mentioning the file path", func() {
			Expect(err.Error()).To(ContainSubstring("custom.css"))
		})
	})

	When("the CSS file exists and is readable", func() {
		var err error
		var cssContent []byte

		BeforeEach(func() {
			cssContent = []byte("body { color: red; }")
			cssFile := path.Join(tempDir, "custom.css")
			writeErr := os.WriteFile(cssFile, cssContent, 0644)
			Expect(writeErr).NotTo(HaveOccurred())
			err = s.LoadCustomCSS(cssFile)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should assign the CSS content to s.CSS", func() {
			Expect(s.CSS).To(Equal(cssContent))
		})
	})
})

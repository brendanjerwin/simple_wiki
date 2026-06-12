package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReadMutationBaseline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "read-mutation-baseline Suite")
}

var _ = Describe("runMutationBaseline", func() {
	var (
		dir      string
		path     string
		stdout   bytes.Buffer
		stderr   bytes.Buffer
		exitCode int
	)

	BeforeEach(func() {
		var err error
		dir = GinkgoT().TempDir()
		path = filepath.Join(dir, "baseline.json")
		err = os.WriteFile(path, []byte(`{"minimum_patch_efficacy":85.0}`), 0o600)
		Expect(err).NotTo(HaveOccurred())
		stdout.Reset()
		stderr.Reset()
	})

	When("the baseline file is valid", func() {
		BeforeEach(func() {
			exitCode = runMutationBaseline([]string{path}, &stdout, &stderr)
		})

		It("should succeed", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("should print the threshold", func() {
			Expect(stdout.String()).To(Equal("85.0\n"))
		})

		It("should not print errors", func() {
			Expect(stderr.String()).To(BeEmpty())
		})
	})

	When("a supported field is requested explicitly", func() {
		BeforeEach(func() {
			exitCode = runMutationBaseline([]string{"-field", "minimum_patch_efficacy", path}, &stdout, &stderr)
		})

		It("should succeed", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("should print the threshold", func() {
			Expect(stdout.String()).To(Equal("85.0\n"))
		})
	})

	When("no baseline path is supplied", func() {
		BeforeEach(func() {
			exitCode = runMutationBaseline(nil, &stdout, &stderr)
		})

		It("should fail", func() {
			Expect(exitCode).To(Equal(1))
		})

		It("should print usage", func() {
			Expect(stderr.String()).To(ContainSubstring("usage: read-mutation-baseline"))
		})
	})

	When("the requested field is unsupported", func() {
		BeforeEach(func() {
			exitCode = runMutationBaseline([]string{"-field", "unknown", path}, &stdout, &stderr)
		})

		It("should fail", func() {
			Expect(exitCode).To(Equal(1))
		})

		It("should explain the unsupported field", func() {
			Expect(stderr.String()).To(ContainSubstring(`unsupported field "unknown"`))
		})
	})

	When("the baseline file is missing", func() {
		BeforeEach(func() {
			exitCode = runMutationBaseline([]string{filepath.Join(dir, "missing.json")}, &stdout, &stderr)
		})

		It("should fail", func() {
			Expect(exitCode).To(Equal(1))
		})

		It("should explain the read failure", func() {
			Expect(stderr.String()).To(ContainSubstring("read mutation baseline"))
		})
	})

	When("the baseline JSON is invalid", func() {
		BeforeEach(func() {
			err := os.WriteFile(path, []byte(`not json`), 0o600)
			Expect(err).NotTo(HaveOccurred())
			exitCode = runMutationBaseline([]string{path}, &stdout, &stderr)
		})

		It("should fail", func() {
			Expect(exitCode).To(Equal(1))
		})

		It("should explain the parse failure", func() {
			Expect(stderr.String()).To(ContainSubstring("parse mutation baseline"))
		})
	})

	When("the threshold is outside the valid range", func() {
		BeforeEach(func() {
			err := os.WriteFile(path, []byte(`{"minimum_patch_efficacy":101}`), 0o600)
			Expect(err).NotTo(HaveOccurred())
			exitCode = runMutationBaseline([]string{path}, &stdout, &stderr)
		})

		It("should fail", func() {
			Expect(exitCode).To(Equal(1))
		})

		It("should explain the invalid threshold", func() {
			Expect(stderr.String()).To(ContainSubstring("minimum_patch_efficacy must be greater than 0 and at most 100"))
		})
	})
})

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cliBinaryName", func() {
	It("returns the platform-specific name served by the wiki", func() {
		Expect(cliBinaryName()).To(Equal("wiki-cli-" + strings.ToLower(runtime.GOOS) + "-" + runtime.GOARCH))
	})
})

var _ = Describe("downloadMatchingBinary", func() {
	var (
		tmpDir  string
		exePath string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "selfupdate-test")
		Expect(err).NotTo(HaveOccurred())
		exePath = tmpDir + "/wiki-cli-test"
		Expect(os.WriteFile(exePath, []byte("old-binary"), 0o755)).To(Succeed())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	When("the server serves a binary", func() {
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/cli/wiki-cli-") {
					_, _ = w.Write([]byte("new-binary-bytes"))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
		})

		AfterEach(func() {
			server.Close()
		})

		It("writes the body to a sibling temp file", func() {
			newPath, err := downloadMatchingBinary(server.URL, exePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(newPath).To(HavePrefix(tmpDir + "/.wiki-cli-"))
			body, readErr := os.ReadFile(newPath)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("new-binary-bytes"))
		})
	})

	When("the server is unreachable", func() {
		It("returns an error and leaves no temp file", func() {
			_, err := downloadMatchingBinary("http://127.0.0.1:1", exePath)
			Expect(err).To(HaveOccurred())
			entries, _ := os.ReadDir(tmpDir)
			Expect(entries).To(HaveLen(1)) // only the original exe
		})
	})

	When("the server returns non-200", func() {
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			}))
		})

		AfterEach(func() {
			server.Close()
		})

		It("returns an error", func() {
			_, err := downloadMatchingBinary(server.URL, exePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("502"))
		})
	})
})

var _ = Describe("swapBinary", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "swap-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("replaces the target's contents atomically and removes the temp file", func() {
		exePath := tmpDir + "/bin"
		Expect(os.WriteFile(exePath, []byte("old"), 0o755)).To(Succeed())
		newPath := tmpDir + "/.update"
		Expect(os.WriteFile(newPath, []byte("new"), 0o755)).To(Succeed())

		Expect(swapBinary(exePath, newPath)).To(Succeed())

		body, err := os.ReadFile(exePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("new"))
		_, statErr := os.Stat(newPath)
		Expect(statErr).To(HaveOccurred(), "temp file should have been renamed away")
	})
})

var _ = Describe("selfUpdate end-to-end", func() {
	var (
		tmpDir  string
		exePath string
		server  *httptest.Server
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "selfupdate-e2e")
		Expect(err).NotTo(HaveOccurred())
		exePath = tmpDir + "/wiki-cli-linux-amd64"
		Expect(os.WriteFile(exePath, []byte("old-binary"), 0o755)).To(Succeed())
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("new-binary"))
		}))
	})

	AfterEach(func() {
		server.Close()
		_ = os.RemoveAll(tmpDir)
	})

	It("downloads, swaps, and reports re-exec required", func() {
		updated, err := selfUpdateForPath(server.URL, exePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(updated).To(BeTrue())

		body, readErr := os.ReadFile(exePath)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("new-binary"))
	})
})

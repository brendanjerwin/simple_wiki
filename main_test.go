//revive:disable:dot-imports
package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

func TestGenerateCookieSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("generateRandomCookieSecret", func() {
	It("should encode bytes from the reader as a lowercase hex string", func() {
		// All-zeros reader → deterministic output of 64 '0' characters.
		reader := bytes.NewReader(make([]byte, 32))
		secret, err := generateRandomCookieSecret(reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret).To(Equal("0000000000000000000000000000000000000000000000000000000000000000"))
	})

	It("should return different secrets on two successive calls with crypto/rand", func() {
		secret1, err := generateRandomCookieSecret(rand.Reader)
		Expect(err).NotTo(HaveOccurred())

		secret2, err := generateRandomCookieSecret(rand.Reader)
		Expect(err).NotTo(HaveOccurred())

		Expect(secret1).NotTo(Equal(secret2))
	})

	It("should propagate an error from the reader", func() {
		errReader := &errorReader{err: errors.New("simulated read failure")}
		_, err := generateRandomCookieSecret(errReader)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to generate random cookie secret"))
	})
})

// errorReader is an io.Reader that always returns an error.
type errorReader struct{ err error }

func (e *errorReader) Read(_ []byte) (int, error) { return 0, e.err }

var _ = Describe("resolveCookieSecret", func() {
	When("a non-empty secret is provided", func() {
		It("should return the provided secret unchanged and generated=false", func() {
			secret, generated, err := resolveCookieSecret("my-explicit-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal("my-explicit-secret"))
			Expect(generated).To(BeFalse())
		})
	})

	When("an empty secret is provided", func() {
		It("should return a valid random secret and generated=true", func() {
			secret, generated, err := resolveCookieSecret("")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(generated).To(BeTrue())
		})
	})
})

var _ = Describe("getFlags", func() {
	It("should include a cookie-secret flag with an empty default value", func() {
		flags := getFlags()
		var foundFlag cli.StringFlag
		foundOk := false
		for _, f := range flags {
			if sf, ok := f.(cli.StringFlag); ok && sf.Name == "cookie-secret" {
				foundFlag = sf
				foundOk = true
				break
			}
		}
		Expect(foundOk).To(BeTrue(), "cookie-secret flag not found")
		Expect(foundFlag.Value).To(Equal(""), "cookie-secret default should be empty, not 'secret'")
	})
})

var _ = Describe("createSite", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_createsite_test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	newContext := func(cookieSecret string) *cli.Context {
		flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
		flagSet.String("data", tmpDir, "")
		flagSet.String("cookie-secret", cookieSecret, "")
		flagSet.String("css", "", "")
		flagSet.String("default-page", "home", "")
		flagSet.Int("debounce", 0, "")
		flagSet.Bool("debug", false, "")
		flagSet.Bool("block-file-uploads", false, "")
		flagSet.Uint("max-upload-mb", 10, "")
		flagSet.Uint("max-document-length", 10000, "")
		return cli.NewContext(nil, flagSet, nil)
	}

	When("no cookie-secret is provided", func() {
		It("should create a site with a generated random secret", func() {
			site, err := createSite(newContext(""))
			Expect(err).NotTo(HaveOccurred())
			Expect(site).NotTo(BeNil())
			Expect(site.SessionStore).NotTo(BeNil())
			Expect(site.PathToData).To(Equal(tmpDir))
		})
	})

	When("an explicit cookie-secret is provided", func() {
		It("should create a site using that secret", func() {
			site, err := createSite(newContext("explicit-test-secret"))
			Expect(err).NotTo(HaveOccurred())
			Expect(site).NotTo(BeNil())
			Expect(site.SessionStore).NotTo(BeNil())
			Expect(site.PathToData).To(Equal(tmpDir))
		})
	})

	When("the CSS file does not exist", func() {
		var err error

		BeforeEach(func() {
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
			flagSet.String("data", tmpDir, "")
			flagSet.String("cookie-secret", "test-secret", "")
			flagSet.String("css", "/nonexistent/path/custom.css", "")
			flagSet.String("default-page", "home", "")
			flagSet.Int("debounce", 0, "")
			flagSet.Bool("debug", false, "")
			flagSet.Bool("block-file-uploads", false, "")
			flagSet.Uint("max-upload-mb", 10, "")
			flagSet.Uint("max-document-length", 10000, "")
			ctx := cli.NewContext(nil, flagSet, nil)
			_, err = createSite(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should mention the CSS file in the error", func() {
			Expect(err.Error()).To(ContainSubstring("custom.css"))
		})
	})
})

var _ = Describe("getCommitHash", func() {
	var savedCommit string

	BeforeEach(func() {
		savedCommit = commit
		DeferCleanup(func() { commit = savedCommit })
	})

	setPath := func(newPath string) {
		original := os.Getenv("PATH")
		Expect(os.Setenv("PATH", newPath)).To(Succeed())
		DeferCleanup(func() { Expect(os.Setenv("PATH", original)).To(Succeed()) })
	}

	When("commit was set at build time (not 'n/a')", func() {
		BeforeEach(func() {
			commit = "deadbeef1234"
		})

		It("should return the build-time commit value without invoking git", func() {
			Expect(getCommitHash()).To(Equal("deadbeef1234"))
		})
	})

	When("commit is 'n/a' and git is not in PATH", func() {
		BeforeEach(func() {
			commit = "n/a"
			setPath("")
		})

		It("should return 'dev'", func() {
			Expect(getCommitHash()).To(Equal("dev"))
		})
	})

	When("commit is 'n/a' and git is found but rev-parse fails", func() {
		var tmpDir string

		BeforeEach(func() {
			commit = "n/a"
			var err error
			tmpDir, err = os.MkdirTemp("", "git_test_*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { os.RemoveAll(tmpDir) })

			fakeGit := filepath.Join(tmpDir, "git")
			err = os.WriteFile(fakeGit, []byte("#!/bin/sh\nexit 1\n"), 0755)
			Expect(err).NotTo(HaveOccurred())

			setPath(tmpDir)
		})

		It("should return 'dev'", func() {
			Expect(getCommitHash()).To(Equal("dev"))
		})
	})

	When("commit is 'n/a' and git rev-parse succeeds", func() {
		var tmpDir string

		BeforeEach(func() {
			commit = "n/a"
			var err error
			tmpDir, err = os.MkdirTemp("", "git_test_*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { os.RemoveAll(tmpDir) })

			fakeGit := filepath.Join(tmpDir, "git")
			err = os.WriteFile(fakeGit, []byte("#!/bin/sh\necho 'abcdef1234567890abcdef1234567890abcdef12'\n"), 0755)
			Expect(err).NotTo(HaveOccurred())

			setPath(tmpDir)
		})

		It("should return the trimmed output from git", func() {
			Expect(getCommitHash()).To(Equal("abcdef1234567890abcdef1234567890abcdef12"))
		})
	})
})

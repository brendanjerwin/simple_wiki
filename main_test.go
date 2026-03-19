//revive:disable:dot-imports
package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"flag"
	"os"
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
		var found *cli.StringFlag
		for _, f := range flags {
			if sf, ok := f.(cli.StringFlag); ok && sf.Name == "cookie-secret" {
				found = &sf
				break
			}
		}
		Expect(found).NotTo(BeNil(), "cookie-secret flag not found")
		Expect(found.Value).To(Equal(""), "cookie-secret default should be empty, not 'secret'")
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
		})
	})

	When("an explicit cookie-secret is provided", func() {
		It("should create a site using that secret", func() {
			site, err := createSite(newContext("explicit-test-secret"))
			Expect(err).NotTo(HaveOccurred())
			Expect(site).NotTo(BeNil())
		})
	})
})

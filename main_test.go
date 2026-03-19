//revive:disable:dot-imports
package main

import (
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
	It("should return a 64-character lowercase hex string (32 bytes)", func() {
		secret, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())
		Expect(secret).To(MatchRegexp(`^[0-9a-f]{64}$`))
	})

	It("should return different secrets on two successive calls", func() {
		secret1, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())

		secret2, err := generateRandomCookieSecret()
		Expect(err).NotTo(HaveOccurred())

		Expect(secret1).NotTo(Equal(secret2))
	})
})

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

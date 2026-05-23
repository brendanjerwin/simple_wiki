package pagestore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// uppercaseCanonicalizer is a stand-in for the real format canonicalizer
// (Phase 4). It capitalizes the entire byte sequence so tests can verify
// the decorator runs the transform and returns the transformed bytes
// without touching the on-disk file.
type uppercaseCanonicalizer struct{}

func (uppercaseCanonicalizer) Canonicalize(content []byte) ([]byte, error) {
	return []byte(strings.ToUpper(string(content))), nil
}

var _ = Describe("CanonicalReader", func() {
	var (
		tempDir string
		store   *Store
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "canonicalreader-test")
		Expect(err).NotTo(HaveOccurred())
		store = NewStore(tempDir)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	writeFile := func(id, contents string) {
		fp := filepath.Join(tempDir, base32tools.EncodeToBase32(strings.ToLower(id))+".md")
		Expect(os.WriteFile(fp, []byte(contents), 0644)).To(Succeed())
	}

	When("the canonicalizer is a noop", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writeFile("noop-page", "lowercase body\n")
			reader := NewCanonicalReader(NoopCanonicalizer{}, store)

			var err error
			p, err = reader.ReadPage("noop-page")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the bytes byte-identical to disk", func() {
			Expect(p.Text).To(Equal("lowercase body\n"))
		})
	})

	When("the canonicalizer transforms the content", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writeFile("transform-page", "lowercase body\n")
			reader := NewCanonicalReader(uppercaseCanonicalizer{}, store)

			var err error
			p, err = reader.ReadPage("transform-page")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return canonicalized (uppercased) bytes to the caller", func() {
			Expect(p.Text).To(Equal("LOWERCASE BODY\n"))
		})

		It("should NOT modify the on-disk file", func() {
			fp := filepath.Join(tempDir, base32tools.EncodeToBase32("transform-page")+".md")
			diskBytes, err := os.ReadFile(fp)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(diskBytes)).To(Equal("lowercase body\n"))
		})
	})

	When("the page does not exist on disk", func() {
		var (
			p   *wikipage.Page
			err error
		)

		BeforeEach(func() {
			reader := NewCanonicalReader(uppercaseCanonicalizer{}, store)
			p, err = reader.ReadPage("ghost")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty new page (no canonicalize attempted)", func() {
			Expect(p).NotTo(BeNil())
			Expect(p.WasLoadedFromDisk).To(BeFalse())
		})
	})

	When("the canonicalizer argument is nil", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writeFile("nil-canonicalizer", "content\n")
			reader := NewCanonicalReader(nil, store)

			var err error
			p, err = reader.ReadPage("nil-canonicalizer")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should default to NoopCanonicalizer (return bytes unchanged)", func() {
			Expect(p.Text).To(Equal("content\n"))
		})
	})
})

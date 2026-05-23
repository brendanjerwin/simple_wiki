package pagestore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// asciiLowerToUpperOffset is the byte offset between ASCII lowercase and
// uppercase: 'a' - 'A' = 32. Pulled out for the test canonicalizer below.
const asciiLowerToUpperOffset = 'a' - 'A'

// uppercaseCanonicalizerForStoreTest is a stand-in canonicalizer that
// uppercases all input bytes. Used to verify SetCanonicalizer actually
// swaps the active canonicalizer in (and the new one runs on writes).
type uppercaseCanonicalizerForStoreTest struct{}

func (uppercaseCanonicalizerForStoreTest) Canonicalize(content []byte) ([]byte, error) {
	out := make([]byte, len(content))
	for i, b := range content {
		if b >= 'a' && b <= 'z' {
			out[i] = b - asciiLowerToUpperOffset
		} else {
			out[i] = b
		}
	}
	return out, nil
}

var _ = Describe("Store accessors", func() {
	Describe("PathToData", func() {
		It("should return the path the store was constructed with", func() {
			s := NewStore("/tmp/some-test-path")
			Expect(s.PathToData()).To(Equal("/tmp/some-test-path"))
		})
	})

	Describe("SetCanonicalizer", func() {
		When("nil is passed", func() {
			It("should fall back to NoopCanonicalizer (defensive)", func() {
				s := NewStore("/tmp")
				s.SetCanonicalizer(nil)
				// Direct introspection isn't possible without exposing the
				// field; verify behavior instead — the noop returns input
				// unchanged.
				out, err := s.canonicalizer.Canonicalize([]byte("hello"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(Equal("hello"))
			})
		})

		When("a real canonicalizer is passed", func() {
			It("should install the canonicalizer on the store", func() {
				s := NewStore("/tmp")
				s.SetCanonicalizer(uppercaseCanonicalizerForStoreTest{})
				out, err := s.canonicalizer.Canonicalize([]byte("hello"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(Equal("HELLO"))
			})
		})
	})
})

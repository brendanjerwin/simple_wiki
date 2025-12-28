//revive:disable:dot-imports
package wikiidentifiers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWikiIdentifiers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WikiIdentifiers Suite")
}

var _ = Describe("MungeIdentifier", func() {
	Describe("when identifier contains UUID", func() {
		var (
			identifier string
			result     string
			err        error
		)

		BeforeEach(func() {
			identifier = "Page-12345678-1234-1234-A123-123456789ABC"
			result, err = MungeIdentifier(identifier)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return lowercase identifier", func() {
			Expect(result).To(Equal("page-12345678-1234-1234-a123-123456789abc"))
		})
	})

	Describe("when identifier is regular text", func() {
		Describe("when converting MyPage", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "MyPage"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert to snake_case and lowercase", func() {
				Expect(result).To(Equal("my_page"))
			})
		})

		Describe("when handling CamelCase", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "SomeCamelCaseIdentifier"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle CamelCase", func() {
				Expect(result).To(Equal("some_camel_case_identifier"))
			})
		})

		Describe("when handling mixed case and numbers", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "lab_wallbins_L3"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle mixed case and numbers", func() {
				Expect(result).To(Equal("lab_wallbins_l3"))
			})
		})

		Describe("when handling already snake_case identifiers", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "already_snake_case"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle already snake_case identifiers", func() {
				Expect(result).To(Equal("already_snake_case"))
			})
		})

		Describe("when handling identifiers with numbers", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "TestPage123"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle identifiers with numbers", func() {
				Expect(result).To(Equal("test_page123"))
			})
		})

		Describe("when handling identifiers starting with numbers", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "123TestPage"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle identifiers starting with numbers", func() {
				Expect(result).To(Equal("123test_page"))
			})
		})
	})

	Describe("when identifier has special characters", func() {
		Describe("when handling underscores", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "test_identifier_with_underscores"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle underscores", func() {
				Expect(result).To(Equal("test_identifier_with_underscores"))
			})
		})

		Describe("when handling hyphens", func() {
			var (
				identifier string
				result     string
				err        error
			)

			BeforeEach(func() {
				identifier = "test-identifier-with-hyphens"
				result, err = MungeIdentifier(identifier)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle hyphens", func() {
				Expect(result).To(Equal("test_identifier_with_hyphens"))
			})
		})
	})

	Describe("Full pipeline tests", func() {
		Describe("when showing the full munging and base32 encoding pipeline for lab_wallbins_L3", func() {
			var (
				original string
				munged   string
				encoded  string
				err      error
			)

			BeforeEach(func() {
				original = "lab_wallbins_L3"
				munged, err = MungeIdentifier(original)
				Expect(err).NotTo(HaveOccurred())
				encoded = base32tools.EncodeToBase32(munged)
				
				// Log the full pipeline for debugging
				GinkgoWriter.Printf("Original: %s\n", original)
				GinkgoWriter.Printf("Munged: %s\n", munged)
				GinkgoWriter.Printf("Base32 encoded: %s\n", encoded)
			})

			It("should munge to lowercase", func() {
				Expect(munged).To(Equal("lab_wallbins_l3"))
			})

			It("should encode to non-empty base32", func() {
				Expect(encoded).NotTo(BeEmpty())
			})
		})

		Describe("when decoding existing filenames", func() {
			var (
				existingFiles []string
				decodedFiles  map[string]string
			)

			BeforeEach(func() {
				// Some existing filenames from the data directory
				existingFiles = []string{
					"NRQWEX3DMFRGS3TFOQZA====",
					"NRQWEX3XMFWGYX3CNFXHG===",
					"NRQWEX3TNVQWY3C7OBQXE5DT",
				}
				
				decodedFiles = make(map[string]string)
				for _, filename := range existingFiles {
					// Remove the .json/.md extension if present
					base32Part := strings.Split(filename, ".")[0]
					decoded, err := base32tools.DecodeFromBase32(base32Part)
					Expect(err).NotTo(HaveOccurred())
					decodedFiles[base32Part] = decoded
					
					GinkgoWriter.Printf("Base32: %s -> Decoded: '%s'\n", base32Part, decoded)
				}
			})

			It("should decode all existing filenames without errors", func() {
				Expect(decodedFiles).To(HaveLen(len(existingFiles)))
				for filename := range decodedFiles {
					Expect(decodedFiles[filename]).NotTo(BeEmpty())
				}
			})
		})
	})

	// Table-driven tests for comprehensive identifier munging
	Describe("URL-safety and Unicode sanitization", func() {
		type testCase struct {
			name     string
			input    string
			expected string
			hasError bool
		}

		// ASCII test cases
		asciiTests := []testCase{
			// Forward slashes
			{name: "forward slash", input: "foo/bar", expected: "foo_bar"},
			{name: "multiple slashes", input: "a/b/c", expected: "a_b_c"},
			{name: "leading slash", input: "/foo", expected: "foo"},
			{name: "trailing slash", input: "foo/", expected: "foo"},

			// Backslashes
			{name: "backslash", input: "foo\\bar", expected: "foo_bar"},
			{name: "multiple backslashes", input: "a\\b\\c", expected: "a_b_c"},

			// URL-problematic characters
			{name: "question mark", input: "foo?bar", expected: "foo_bar"},
			{name: "hash", input: "foo#hash", expected: "foo_hash"},
			{name: "ampersand", input: "foo&bar", expected: "foo_bar"},
			{name: "equals", input: "foo=bar", expected: "foo_bar"},
			{name: "percent", input: "foo%20bar", expected: "foo_20bar"},
			{name: "at sign", input: "user@domain", expected: "user_domain"},
			{name: "colon", input: "foo:bar", expected: "foo_bar"},
			{name: "period", input: "foo.bar", expected: "foo_bar"},

			// Sub-delimiters (RFC 3986)
			{name: "semicolon", input: "foo;bar", expected: "foo_bar"},
			{name: "parentheses", input: "foo(1)", expected: "foo_1"},
			{name: "brackets", input: "foo[0]", expected: "foo_0"},
			{name: "exclamation", input: "hello!", expected: "hello"},
			{name: "asterisk", input: "foo*bar", expected: "foo_bar"},
			{name: "plus", input: "foo+bar", expected: "foo_bar"},
			{name: "comma", input: "foo,bar", expected: "foo_bar"},

			// Whitespace
			{name: "space", input: "foo bar", expected: "foo_bar"},
			{name: "leading spaces", input: "  foo", expected: "foo"},
			{name: "trailing spaces", input: "foo  ", expected: "foo"},
			{name: "tab", input: "foo\tbar", expected: "foo_bar"},
			{name: "newline", input: "foo\nbar", expected: "foo_bar"},

			// Consecutive underscores
			{name: "double underscore", input: "foo__bar", expected: "foo_bar"},
			{name: "triple underscore", input: "foo___bar", expected: "foo_bar"},
			{name: "leading underscores", input: "___foo", expected: "foo"},

			// Explicit underscores (should be preserved when already valid)
			{name: "leading and trailing single underscore", input: "_foo_bar_", expected: "_foo_bar_"},

			// Trailing dunderscore (backwards compat - preserved only if original input had it)
			{name: "trailing dunderscore", input: "foo__", expected: "foo__"},
			{name: "trailing triple underscore", input: "foo___", expected: "foo__"},
			{name: "dunderscore after underscore content", input: "foo_bar__", expected: "foo_bar__"},
			{name: "quad underscore after underscore content", input: "foo_bar____", expected: "foo_bar__"},
			{name: "dunderscore example", input: "___something __ with dunderscore _____", expected: "something_with_dunderscore__"},

			// Empty/invalid inputs
			{name: "empty string", input: "", expected: "", hasError: true},
			{name: "only slashes", input: "///", expected: "", hasError: true},
			{name: "only question marks", input: "???", expected: "", hasError: true},
			{name: "only spaces", input: "   ", expected: "", hasError: true},
			{name: "only underscore", input: "_", expected: "", hasError: true},

			// Already valid
			{name: "already valid", input: "valid_id", expected: "valid_id"},
			{name: "with numbers", input: "with_123_numbers", expected: "with_123_numbers"},
			{name: "single char", input: "a", expected: "a"},
			{name: "single uppercase", input: "A", expected: "a"},
			{name: "only numbers", input: "123", expected: "123"},

			// Complex combinations
			{name: "email-like", input: "test@email.com", expected: "test_email_com"},
			{name: "url-like", input: "http://foo", expected: "http_foo"},
			{name: "complex", input: "a/b@c:d.e", expected: "a_b_c_d_e"},

			// Other problematic chars
			{name: "angle brackets", input: "foo<bar>", expected: "foo_bar"},
			{name: "curly braces", input: "foo{bar}", expected: "foo_bar"},
			{name: "pipe", input: "foo|bar", expected: "foo_bar"},
			{name: "caret", input: "foo^bar", expected: "foo_bar"},
			{name: "backtick", input: "foo`bar", expected: "foo_bar"},
			{name: "tilde", input: "foo~bar", expected: "foo_bar"},
			{name: "double quote", input: "foo\"bar", expected: "foo_bar"},
			{name: "single quote", input: "foo'bar", expected: "foo_bar"},
		}

		// Unicode letter/digit test cases (should be preserved)
		unicodeTests := []testCase{
			{name: "Japanese hiragana", input: "こんにちは", expected: "こんにちは"},
			{name: "Japanese with space", input: "東京 駅", expected: "東京_駅"},
			{name: "Chinese", input: "北京市", expected: "北京市"},
			{name: "Arabic", input: "مرحبا", expected: "مرحبا"},
			{name: "Greek", input: "αβγδ", expected: "αβγδ"},
			{name: "Cyrillic", input: "москва", expected: "москва"},
			{name: "Mixed scripts", input: "hello世界", expected: "hello世界"},
			{name: "Accented", input: "café", expected: "café"},
			{name: "Arabic-Indic digits", input: "٠١٢٣", expected: "٠١٢٣"},
		}

		// Unicode punctuation test cases (should be replaced with underscore)
		unicodePunctTests := []testCase{
			{name: "full-width exclaim", input: "东京！", expected: "东京"},
			{name: "Chinese comma", input: "一、二", expected: "一_二"},
			{name: "Chinese period", input: "完了。", expected: "完了"},
			{name: "Japanese brackets", input: "「名前」", expected: "名前"},
			{name: "full-width parens", input: "（内容）", expected: "内容"},
			{name: "curly quotes", input: "\u201Chello\u201D", expected: "hello"},
			{name: "single curly", input: "\u2018test\u2019", expected: "test"},
			{name: "em dash", input: "a—b", expected: "a_b"},
			{name: "en dash", input: "a–b", expected: "a_b"},
			{name: "ellipsis", input: "wait…", expected: "wait"},
			{name: "middle dot", input: "a·b", expected: "a_b"},
			{name: "bullet", input: "a•b", expected: "a_b"},
			{name: "euro sign", input: "100€", expected: "100"},
			{name: "yen sign", input: "100¥", expected: "100"},
			{name: "plus/minus", input: "±5", expected: "5"},
			{name: "math operators", input: "a×b÷c", expected: "a_b_c"},
			{name: "arrows only", input: "→←", expected: "", hasError: true},
			{name: "Korean punctuation", input: "안녕！", expected: "안녕"},
			{name: "Thai punctuation", input: "สวัสดี。", expected: "สวัสดี"},
		}

		// Adversarial Unicode test cases
		adversarialTests := []testCase{
			{name: "zero-width space", input: "foo\u200Bbar", expected: "foobar"},
			{name: "zero-width joiner", input: "foo\u200Dbar", expected: "foobar"},
			{name: "bidi override", input: "\u202Efoo", expected: "foo"},
			{name: "soft hyphen", input: "foo\u00ADbar", expected: "foobar"},
			{name: "NBSP", input: "foo\u00A0bar", expected: "foo_bar"},
			{name: "line separator", input: "foo\u2028bar", expected: "foo_bar"},
			{name: "emoji", input: "hello👋world", expected: "hello_world"},
			{name: "private use", input: "foo\uE000bar", expected: "foobar"},
			{name: "control char NUL", input: "foo\x00bar", expected: "foobar"},
			{name: "only adversarial", input: "\u200B\u200C\u200D", expected: "", hasError: true},
		}

		// NFKC normalization test cases
		nfkcTests := []testCase{
			{name: "stylized text", input: "𝓗𝓮𝓵𝓵𝓸", expected: "hello"},
			{name: "decomposed accent", input: "cafe\u0301", expected: "café"},
			{name: "full-width letters", input: "ＡＢＣ", expected: "abc"},
			{name: "circled digits", input: "①②③", expected: "123"},
		}

		runTestCases := func(tests []testCase) {
			for _, tc := range tests {
				tc := tc // capture range variable

				When(fmt.Sprintf("input is %q (%s)", tc.input, tc.name), func() {
					var (
						result string
						err    error
					)

					BeforeEach(func() {
						result, err = MungeIdentifier(tc.input)
					})

					if tc.hasError {
						It("should return an error", func() {
							Expect(err).To(HaveOccurred())
						})
					} else {
						It("should not return an error", func() {
							Expect(err).NotTo(HaveOccurred())
						})

						It(fmt.Sprintf("should return %q", tc.expected), func() {
							Expect(result).To(Equal(tc.expected))
						})

						When("re-munging the result", func() {
							var (
								remunged   string
								remungeErr error
							)

							BeforeEach(func() {
								remunged, remungeErr = MungeIdentifier(result)
							})

							It("should not return an error", func() {
								Expect(remungeErr).NotTo(HaveOccurred())
							})

							It("should return the same result (idempotent)", func() {
								Expect(remunged).To(Equal(result))
							})
						})
					}
				})
			}
		}

		Describe("ASCII character handling", func() {
			runTestCases(asciiTests)
		})

		Describe("Unicode letter and digit preservation", func() {
			runTestCases(unicodeTests)
		})

		Describe("Unicode punctuation replacement", func() {
			runTestCases(unicodePunctTests)
		})

		Describe("Adversarial Unicode removal", func() {
			runTestCases(adversarialTests)
		})

		Describe("NFKC normalization", func() {
			runTestCases(nfkcTests)
		})
	})

	// UUID handling tests
	Describe("UUID preservation", func() {
		When("identifier contains a UUID", func() {
			var (
				result string
				err    error
			)

			BeforeEach(func() {
				result, err = MungeIdentifier("page-12345678-1234-1234-a123-123456789abc")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve hyphens in UUID", func() {
				Expect(result).To(Equal("page-12345678-1234-1234-a123-123456789abc"))
			})

			When("re-munging the result", func() {
				var (
					remunged   string
					remungeErr error
				)

				BeforeEach(func() {
					remunged, remungeErr = MungeIdentifier(result)
				})

				It("should not return an error", func() {
					Expect(remungeErr).NotTo(HaveOccurred())
				})

				It("should return the same result (idempotent)", func() {
					Expect(remunged).To(Equal(result))
				})
			})
		})

		When("UUID has uppercase letters", func() {
			var (
				result string
				err    error
			)

			BeforeEach(func() {
				result, err = MungeIdentifier("Page-12345678-1234-1234-A123-123456789ABC")
			})

			It("should lowercase the UUID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal("page-12345678-1234-1234-a123-123456789abc"))
			})
		})
	})
})

var _ = Describe("isAdversarial", func() {
	Describe("when checking format characters (Cf)", func() {
		When("rune is zero-width space", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\u200B')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is zero-width joiner", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\u200D')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is bidi override", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\u202E')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is soft hyphen", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\u00AD')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("when checking private use characters (Co)", func() {
		When("rune is private use area start", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\uE000')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is private use area middle", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\uF000')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("when checking surrogate characters (Cs)", func() {
		When("rune is high surrogate", func() {
			var result bool

			BeforeEach(func() {
				// High surrogate U+D800 - technically invalid in UTF-8 but we check anyway
				result = isAdversarial(rune(0xD800))
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is low surrogate", func() {
			var result bool

			BeforeEach(func() {
				// Low surrogate U+DC00 - technically invalid in UTF-8 but we check anyway
				result = isAdversarial(rune(0xDC00))
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("when checking control characters", func() {
		When("rune is NUL", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\x00')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is BEL", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\x07')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is backspace", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\x08')
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("rune is tab (whitespace-like)", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\t')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is newline (whitespace-like)", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\n')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is carriage return (whitespace-like)", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('\r')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("when checking normal characters", func() {
		When("rune is lowercase ASCII letter", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('a')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is uppercase ASCII letter", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('Z')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is digit zero", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('0')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is digit nine", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('9')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is underscore", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('_')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is hyphen", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('-')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is Japanese kanji", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('日')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is Greek letter", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial('α')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("rune is space", func() {
			var result bool

			BeforeEach(func() {
				result = isAdversarial(' ')
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})
	})
})

var _ = Describe("containsAdversarialUnicode", func() {
	Describe("when string contains adversarial characters", func() {
		When("string has zero-width space", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo\u200Bbar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("string has bidi override", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("\u202Etest")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("string has NUL character", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo\x00bar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("when string contains disallowed non-adversarial characters", func() {
		When("string has space", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo bar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("string has slash", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo/bar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("string has punctuation", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo.bar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("string has symbols", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo@bar")
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("when string contains only allowed characters", func() {
		When("string has ASCII letters", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foobar")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("string has underscores", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo_bar")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("string has hyphens", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo-bar")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("string has digits", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("foo123")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("string has Unicode letters", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("日本語")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("string is empty", func() {
			var result bool

			BeforeEach(func() {
				result = containsAdversarialUnicode("")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})
	})
})

var _ = Describe("collapseUnderscores", func() {
	When("input has double underscores", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("foo__bar")
		})

		It("should collapse to single underscore", func() {
			Expect(result).To(Equal("foo_bar"))
		})
	})

	When("input has triple underscores", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("foo___bar")
		})

		It("should collapse to single underscore", func() {
			Expect(result).To(Equal("foo_bar"))
		})
	})

	When("input has multiple groups of underscores", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("a__b___c")
		})

		It("should collapse each group to single underscore", func() {
			Expect(result).To(Equal("a_b_c"))
		})
	})

	When("input is empty string", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("")
		})

		It("should return empty string", func() {
			Expect(result).To(Equal(""))
		})
	})

	When("input has single underscore", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("_")
		})

		It("should return single underscore", func() {
			Expect(result).To(Equal("_"))
		})
	})

	When("input has no underscores", func() {
		var result string

		BeforeEach(func() {
			result = collapseUnderscores("foobar")
		})

		It("should return unchanged", func() {
			Expect(result).To(Equal("foobar"))
		})
	})
})

var _ = Describe("hasTrailingDunderscore", func() {
	When("input ends with double underscore", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("foo__")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("input ends with triple underscore", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("foo___")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("input ends with single underscore", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("foo_")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("input has no underscore", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("foo")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("input is empty string", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("input is single character", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("a")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("double underscore is at start not end", func() {
		var result bool

		BeforeEach(func() {
			result = hasTrailingDunderscore("__foo")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})
})

var _ = Describe("replaceProblematicChars", func() {
	When("input has underscore", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("foo_bar")
		})

		It("should preserve underscore", func() {
			Expect(result).To(Equal("foo_bar"))
		})
	})

	When("input has hyphen", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("foo-bar")
		})

		It("should preserve hyphen", func() {
			Expect(result).To(Equal("foo-bar"))
		})
	})

	When("input has Unicode letters", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("日本語")
		})

		It("should preserve Unicode letters", func() {
			Expect(result).To(Equal("日本語"))
		})
	})

	When("input has digits", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("123")
		})

		It("should preserve digits", func() {
			Expect(result).To(Equal("123"))
		})
	})

	When("input has combining marks", func() {
		var result string

		BeforeEach(func() {
			// Thai vowel mark
			result = replaceProblematicChars("ก\u0E38")
		})

		It("should preserve combining marks", func() {
			Expect(result).To(Equal("ก\u0E38"))
		})
	})

	When("input has space", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("foo bar")
		})

		It("should replace space with underscore", func() {
			Expect(result).To(Equal("foo_bar"))
		})
	})

	When("input has slash", func() {
		var result string

		BeforeEach(func() {
			result = replaceProblematicChars("foo/bar")
		})

		It("should replace slash with underscore", func() {
			Expect(result).To(Equal("foo_bar"))
		})
	})
})

var _ = Describe("keepAllowedChars", func() {
	When("input has letters", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("abc")
		})

		It("should keep letters", func() {
			Expect(result).To(Equal("abc"))
		})
	})

	When("input has digits", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("123")
		})

		It("should keep digits", func() {
			Expect(result).To(Equal("123"))
		})
	})

	When("input has underscore", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("a_b")
		})

		It("should keep underscore", func() {
			Expect(result).To(Equal("a_b"))
		})
	})

	When("input has hyphen", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("a-b")
		})

		It("should keep hyphen", func() {
			Expect(result).To(Equal("a-b"))
		})
	})

	When("input has combining marks", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("ก\u0E38")
		})

		It("should keep combining marks", func() {
			Expect(result).To(Equal("ก\u0E38"))
		})
	})

	When("input has spaces", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("a b")
		})

		It("should remove spaces", func() {
			Expect(result).To(Equal("ab"))
		})
	})

	When("input has punctuation", func() {
		var result string

		BeforeEach(func() {
			result = keepAllowedChars("a.b")
		})

		It("should remove punctuation", func() {
			Expect(result).To(Equal("ab"))
		})
	})
})

var _ = Describe("removeAdversarialUnicode", func() {
	When("input has zero-width space", func() {
		var result string

		BeforeEach(func() {
			result = removeAdversarialUnicode("foo\u200Bbar")
		})

		It("should remove zero-width space", func() {
			Expect(result).To(Equal("foobar"))
		})
	})

	When("input has bidi override", func() {
		var result string

		BeforeEach(func() {
			result = removeAdversarialUnicode("\u202Etest")
		})

		It("should remove bidi override", func() {
			Expect(result).To(Equal("test"))
		})
	})

	When("input has NUL character", func() {
		var result string

		BeforeEach(func() {
			result = removeAdversarialUnicode("foo\x00bar")
		})

		It("should remove NUL character", func() {
			Expect(result).To(Equal("foobar"))
		})
	})

	When("input has normal characters", func() {
		var result string

		BeforeEach(func() {
			result = removeAdversarialUnicode("hello world")
		})

		It("should preserve normal characters", func() {
			Expect(result).To(Equal("hello world"))
		})
	})

	When("input has Unicode letters", func() {
		var result string

		BeforeEach(func() {
			result = removeAdversarialUnicode("日本語")
		})

		It("should preserve Unicode letters", func() {
			Expect(result).To(Equal("日本語"))
		})
	})
})
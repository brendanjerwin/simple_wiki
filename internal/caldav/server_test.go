//revive:disable:dot-imports
package caldav_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
)

// The package-level RunSpecs lives in backend_test.go (TestBackend);
// adding a second TestServer would trip Ginkgo's "RunSpecs more than
// once" guard. Specs in this file are attached to that runner via
// var _ = Describe(...).
//
// SanitizePathComponent / ValidateUID / ParsePath are unexported, so
// tests exercise them through their re-exports in export_test.go (a
// tiny test-only helper file).

var _ = Describe("sanitizePathComponent", func() {
	When("input is empty", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("")
		})

		It("should return ErrEmptyPathComponent", func() {
			Expect(errors.Is(err, caldav.ErrEmptyPathComponent)).To(BeTrue())
		})
	})

	When("input is whitespace only", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("   ")
		})

		It("should return ErrEmptyPathComponent", func() {
			Expect(errors.Is(err, caldav.ErrEmptyPathComponent)).To(BeTrue())
		})
	})

	When(`input is ".."`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("..")
		})

		It("should return ErrPathTraversal", func() {
			Expect(errors.Is(err, caldav.ErrPathTraversal)).To(BeTrue())
		})
	})

	When("input contains a NUL byte", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("foo\x00bar")
		})

		It("should return ErrPathContainsNUL", func() {
			Expect(errors.Is(err, caldav.ErrPathContainsNUL)).To(BeTrue())
		})
	})

	When(`input starts with "/"`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("/foo")
		})

		It("should return ErrPathLeadingSeparator", func() {
			Expect(errors.Is(err, caldav.ErrPathLeadingSeparator)).To(BeTrue())
		})
	})

	When(`input starts with "\\"`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest(`\foo`)
		})

		It("should return ErrPathLeadingSeparator", func() {
			Expect(errors.Is(err, caldav.ErrPathLeadingSeparator)).To(BeTrue())
		})
	})

	When("input is a normal ASCII identifier", func() {
		var got string
		var err error

		BeforeEach(func() {
			got, err = caldav.SanitizePathComponentForTest("shopping")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the identifier unchanged", func() {
			Expect(got).To(Equal("shopping"))
		})
	})

	When("input has surrounding whitespace", func() {
		var got string
		var err error

		BeforeEach(func() {
			got, err = caldav.SanitizePathComponentForTest("  shopping  ")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the trimmed identifier", func() {
			Expect(got).To(Equal("shopping"))
		})
	})
})

var _ = Describe("validateUID", func() {
	When("uid is a 26-char Crockford-base32 ULID", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8C")).NotTo(HaveOccurred())
		})
	})

	When("uid is a lowercase RFC 4122 UUID with dashes", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("c5b7e0a4-3d2e-4f1a-9b8c-1234567890ab")).NotTo(HaveOccurred())
		})
	})

	When("uid is an uppercase RFC 4122 UUID with dashes", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("C5B7E0A4-3D2E-4F1A-9B8C-1234567890AB")).NotTo(HaveOccurred())
		})
	})

	When("uid is empty", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is 25 characters", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is 27 characters", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8CC")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid contains a non-base32 character", func() {
		It("should return ErrInvalidUID", func() {
			// 'I' is excluded from Crockford base32.
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8I")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is a UUID with the wrong dash placement", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("c5b7e0a43d2e-4f1a-9b8c-1234567890ab")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})
})

var _ = Describe("parsePath", func() {
	When("path is /<page>", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should leave list empty", func() {
			Expect(list).To(Equal(""))
		})

		It("should leave uid empty", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("path is /<page>/<list>", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should return the list component", func() {
			Expect(list).To(Equal("this-week"))
		})

		It("should leave uid empty", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("path is /<page>/<list>/<uid>.ics", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should return the list component", func() {
			Expect(list).To(Equal("this-week"))
		})

		It("should return the uid without the .ics suffix", func() {
			Expect(uid).To(Equal("01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"))
		})
	})

	When("path has trailing slash on /<page>/<list>/", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week/")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should treat the path as the collection", func() {
			Expect(page).To(Equal("shopping"))
			Expect(list).To(Equal("this-week"))
			Expect(uid).To(Equal(""))
		})
	})

	When("path is empty", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("path is just /", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("path has too many segments", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/a/b/c.ics/d")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("third segment is not a .ics file", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("uid in third segment is invalid", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/this-week/not-a-ulid.ics")
		})

		It("should return ErrInvalidUID", func() {
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("page contains ..", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/../this-week")
		})

		It("should return ErrPathTraversal", func() {
			Expect(errors.Is(err, caldav.ErrPathTraversal)).To(BeTrue())
		})
	})

	When("list contains a NUL byte", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/list\x00bad")
		})

		It("should return ErrPathContainsNUL", func() {
			Expect(errors.Is(err, caldav.ErrPathContainsNUL)).To(BeTrue())
		})
	})
})

var _ = Describe("Server.serveOPTIONS", func() {
	var server *caldav.Server
	var rec *httptest.ResponseRecorder
	var req *http.Request

	BeforeEach(func() {
		server = &caldav.Server{}
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodOptions, "/shopping", nil)
		server.ServeOPTIONSForTest(rec, req)
	})

	It("should return 200 OK", func() {
		Expect(rec.Code).To(Equal(http.StatusOK))
	})

	It("should set the DAV header to advertise CalDAV capabilities", func() {
		Expect(rec.Header().Get("DAV")).To(Equal("1, 3, calendar-access"))
	})

	It("should set the Allow header listing every supported verb", func() {
		Expect(rec.Header().Get("Allow")).To(Equal("OPTIONS, GET, HEAD, PROPFIND, REPORT, PUT, DELETE"))
	})

	It("should write an empty body", func() {
		Expect(rec.Body.Len()).To(Equal(0))
	})
})

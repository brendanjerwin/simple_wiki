//revive:disable:dot-imports
package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

var _ = Describe("reserved namespace registry — wiki namespace", func() {
	Describe("isReservedTopLevel", func() {
		When("the key is 'wiki'", func() {
			It("should return true", func() {
				Expect(isReservedTopLevel("wiki")).To(BeTrue())
			})
		})
	})

	Describe("reservedKeyInMap", func() {
		When("fm contains the 'wiki' key", func() {
			var found string

			BeforeEach(func() {
				fm := map[string]any{
					"title": "T",
					"wiki":  map[string]any{"checklists": map[string]any{}},
				}
				found = reservedKeyInMap(fm)
			})

			It("should return 'wiki'", func() {
				Expect(found).To(Equal("wiki"))
			})
		})
	})

	Describe("reservedKeyOnPath", func() {
		When("the first path component is 'wiki'", func() {
			var found string

			BeforeEach(func() {
				path := []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "wiki"}},
					{Component: &apiv1.PathComponent_Key{Key: "checklists"}},
				}
				found = reservedKeyOnPath(path)
			})

			It("should return 'wiki'", func() {
				Expect(found).To(Equal("wiki"))
			})
		})
	})

	Describe("reservedNamespaceError", func() {
		When("called with the registered 'wiki' key", func() {
			var err error

			BeforeEach(func() {
				err = reservedNamespaceError("wiki")
			})

			It("should return InvalidArgument", func() {
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})

			It("should describe the rejection as a reserved-namespace violation", func() {
				Expect(err.Error()).To(ContainSubstring("reserved"))
				Expect(err.Error()).To(ContainSubstring("wiki"))
			})
		})
	})

	Describe("preserveReservedSubtrees", func() {
		When("existing has the 'wiki' key and incoming does not", func() {
			var existing, incoming map[string]any

			BeforeEach(func() {
				existing = map[string]any{
					"title": "Old",
					"wiki":  map[string]any{"checklists": map[string]any{"list": map[string]any{}}},
				}
				incoming = map[string]any{"title": "New"}
				preserveReservedSubtrees(existing, incoming)
			})

			It("should copy the wiki subtree into incoming", func() {
				Expect(incoming).To(HaveKey("wiki"))
			})

			It("should not modify non-reserved incoming keys", func() {
				Expect(incoming["title"]).To(Equal("New"))
			})
		})
	})

	Describe("stripReservedKeys", func() {
		When("fm contains the 'wiki' key", func() {
			var stripped map[string]any

			BeforeEach(func() {
				fm := map[string]any{
					"title": "T",
					"wiki":  map[string]any{"checklists": map[string]any{}},
				}
				stripped = stripReservedKeys(fm)
			})

			It("should remove the 'wiki' key", func() {
				Expect(stripped).NotTo(HaveKey("wiki"))
			})

			It("should preserve non-reserved keys", func() {
				Expect(stripped["title"]).To(Equal("T"))
			})
		})
	})
})

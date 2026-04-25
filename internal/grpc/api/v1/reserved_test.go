package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

var _ = Describe("reserved namespace registry", func() {
	Describe("isReservedTopLevel", func() {
		When("the key is registered", func() {
			It("should return true for 'agent'", func() {
				Expect(isReservedTopLevel("agent")).To(BeTrue())
			})
		})

		When("the key is not registered", func() {
			It("should return false", func() {
				Expect(isReservedTopLevel("title")).To(BeFalse())
			})
		})
	})

	Describe("reservedKeyInMap", func() {
		When("fm contains a reserved key", func() {
			var found string

			BeforeEach(func() {
				fm := map[string]any{
					"title": "T",
					"agent": map[string]any{"schedules": []any{}},
				}
				found = reservedKeyInMap(fm)
			})

			It("should return that key", func() {
				Expect(found).To(Equal("agent"))
			})
		})

		When("fm contains no reserved keys", func() {
			It("should return an empty string", func() {
				Expect(reservedKeyInMap(map[string]any{"title": "T"})).To(BeEmpty())
			})
		})

		When("fm is nil", func() {
			It("should return an empty string", func() {
				Expect(reservedKeyInMap(nil)).To(BeEmpty())
			})
		})
	})

	Describe("reservedKeyOnPath", func() {
		When("the first path component is a reserved key", func() {
			var found string

			BeforeEach(func() {
				path := []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "agent"}},
					{Component: &apiv1.PathComponent_Key{Key: "schedules"}},
				}
				found = reservedKeyOnPath(path)
			})

			It("should return the reserved key", func() {
				Expect(found).To(Equal("agent"))
			})
		})

		When("the first path component is not a reserved key", func() {
			It("should return an empty string", func() {
				path := []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "title"}},
				}
				Expect(reservedKeyOnPath(path)).To(BeEmpty())
			})
		})

		When("the path is empty", func() {
			It("should return an empty string", func() {
				Expect(reservedKeyOnPath(nil)).To(BeEmpty())
			})
		})

		When("the first path component is an index, not a key", func() {
			It("should return an empty string", func() {
				path := []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Index{Index: 0}},
				}
				Expect(reservedKeyOnPath(path)).To(BeEmpty())
			})
		})
	})

	Describe("reservedNamespaceError", func() {
		When("called with the registered 'agent' key", func() {
			var err error

			BeforeEach(func() {
				err = reservedNamespaceError("agent")
			})

			It("should return InvalidArgument", func() {
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})

			It("should name AgentMetadataService in the message", func() {
				Expect(err.Error()).To(ContainSubstring("AgentMetadataService"))
			})
		})

		When("called with an unregistered key", func() {
			var err error

			BeforeEach(func() {
				err = reservedNamespaceError("zzz_unknown")
			})

			It("should still return InvalidArgument", func() {
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("preserveReservedSubtrees", func() {
		When("existing has every reserved key and incoming has none", func() {
			var existing, incoming map[string]any

			BeforeEach(func() {
				existing = map[string]any{
					"title": "Old",
					"agent": map[string]any{"schedules": []any{"s1"}},
				}
				incoming = map[string]any{"title": "New"}
				preserveReservedSubtrees(existing, incoming)
			})

			It("should copy every reserved subtree into incoming", func() {
				Expect(incoming).To(HaveKey("agent"))
			})

			It("should not modify non-reserved incoming keys", func() {
				Expect(incoming["title"]).To(Equal("New"))
			})
		})

		When("existing or incoming is nil", func() {
			It("should not panic when existing is nil", func() {
				incoming := map[string]any{"title": "T"}
				Expect(func() { preserveReservedSubtrees(nil, incoming) }).NotTo(Panic())
			})

			It("should not panic when incoming is nil", func() {
				existing := map[string]any{"agent": map[string]any{}}
				Expect(func() { preserveReservedSubtrees(existing, nil) }).NotTo(Panic())
			})

			It("should not panic when both are nil", func() {
				Expect(func() { preserveReservedSubtrees(nil, nil) }).NotTo(Panic())
			})
		})
	})

	Describe("stripReservedKeys", func() {
		When("fm contains both reserved and non-reserved keys", func() {
			var stripped map[string]any

			BeforeEach(func() {
				fm := map[string]any{
					"title":     "T",
					"agent":     map[string]any{"schedules": []any{}},
					"checklist": map[string]any{},
				}
				stripped = stripReservedKeys(fm)
			})

			It("should remove the reserved keys", func() {
				Expect(stripped).NotTo(HaveKey("agent"))
			})

			It("should preserve non-reserved keys", func() {
				Expect(stripped["title"]).To(Equal("T"))
				Expect(stripped).To(HaveKey("checklist"))
			})
		})

		When("fm is nil", func() {
			It("should return nil", func() {
				Expect(stripReservedKeys(nil)).To(BeNil())
			})
		})
	})
})

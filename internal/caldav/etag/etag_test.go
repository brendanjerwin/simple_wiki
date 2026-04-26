//revive:disable:dot-imports
package etag_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/caldav/etag"
)

func TestEtag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "etag")
}

var _ = Describe("ItemETag", func() {
	When("item is nil", func() {
		It("should return the empty string", func() {
			Expect(etag.ItemETag(nil)).To(Equal(""))
		})
	})

	When("item.UpdatedAt is nil", func() {
		It("should return the empty string", func() {
			item := &apiv1.ChecklistItem{Uid: "u"}
			Expect(etag.ItemETag(item)).To(Equal(""))
		})
	})

	When("item has an UpdatedAt", func() {
		It("should return a weak ETag containing the rfc3339nano timestamp", func() {
			t1 := time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
			item := &apiv1.ChecklistItem{Uid: "u", UpdatedAt: timestamppb.New(t1)}
			Expect(etag.ItemETag(item)).To(Equal(`W/"2026-04-25T13:00:00Z"`))
		})

		It("should produce stable values across calls", func() {
			t1 := time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
			item := &apiv1.ChecklistItem{Uid: "u", UpdatedAt: timestamppb.New(t1)}
			Expect(etag.ItemETag(item)).To(Equal(etag.ItemETag(item)))
		})
	})
})

var _ = Describe("CollectionCTag", func() {
	When("checklist is nil", func() {
		It("should return the empty string", func() {
			Expect(etag.CollectionCTag(nil)).To(Equal(""))
		})
	})

	When("checklist.UpdatedAt is nil", func() {
		It("should return the empty string", func() {
			Expect(etag.CollectionCTag(&apiv1.Checklist{Name: "list"})).To(Equal(""))
		})
	})

	When("checklist has UpdatedAt", func() {
		It("should return a quoted rfc3339nano value", func() {
			t1 := time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
			c := &apiv1.Checklist{Name: "list", UpdatedAt: timestamppb.New(t1)}
			Expect(etag.CollectionCTag(c)).To(Equal(`"2026-04-25T13:00:00Z"`))
		})
	})
})

var _ = Describe("CollectionSyncToken", func() {
	When("checklist is nil", func() {
		It("should return the empty string", func() {
			Expect(etag.CollectionSyncToken(nil)).To(Equal(""))
		})
	})

	When("checklist has SyncToken=0", func() {
		It("should return the prefix with a 0 suffix", func() {
			Expect(etag.CollectionSyncToken(&apiv1.Checklist{})).To(Equal("http://simple-wiki.local/ns/sync/0"))
		})
	})

	When("checklist has SyncToken=42", func() {
		It("should return the prefix with the integer suffix", func() {
			Expect(etag.CollectionSyncToken(&apiv1.Checklist{SyncToken: 42})).To(Equal("http://simple-wiki.local/ns/sync/42"))
		})
	})
})

var _ = Describe("ParseSyncToken", func() {
	When("token is empty", func() {
		It("should return (0, nil) — initial sync", func() {
			n, err := etag.ParseSyncToken("")
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(0)))
		})
	})

	When("token is round-tripped from CollectionSyncToken", func() {
		It("should parse back to the original int", func() {
			c := &apiv1.Checklist{SyncToken: 17}
			emitted := etag.CollectionSyncToken(c)
			n, err := etag.ParseSyncToken(emitted)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(17)))
		})
	})

	When("token has the wrong prefix", func() {
		It("should return an error", func() {
			_, err := etag.ParseSyncToken("urn:other:42")
			Expect(err).To(HaveOccurred())
		})
	})

	When("token suffix is not an integer", func() {
		It("should return an error", func() {
			_, err := etag.ParseSyncToken("http://simple-wiki.local/ns/sync/notanumber")
			Expect(err).To(HaveOccurred())
		})
	})
})

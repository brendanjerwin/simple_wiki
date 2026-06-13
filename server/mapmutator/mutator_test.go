package mapmutator

import (
	"context"
	"errors"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type fakeStore struct {
	mu     sync.Mutex
	pages  map[wikipage.PageIdentifier]wikipage.FrontMatter
	writes int
}

func newFakeStore() *fakeStore {
	return &fakeStore{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (s *fakeStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, ok := s.pages[id]
	if !ok {
		return "", nil, errors.New("missing")
	}
	return id, cloneFrontMatter(fm), nil
}

func (s *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[id] = cloneFrontMatter(fm)
	s.writes++
	return nil
}

func (*fakeStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (*fakeStore) WriteMarkdown(wikipage.PageIdentifier, wikipage.Markdown) error {
	return nil
}

func (*fakeStore) DeletePage(wikipage.PageIdentifier) error {
	return nil
}

func (*fakeStore) ModifyMarkdown(_ wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	_, err := modifier("")
	return err
}

func cloneFrontMatter(fm wikipage.FrontMatter) wikipage.FrontMatter {
	out := wikipage.FrontMatter{}
	for k, v := range fm {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, nested := range typed {
			out[k] = cloneAny(nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, cloneAny(nested))
		}
		return out
	default:
		return typed
	}
}

var _ = Describe("Mutator", func() {
	var (
		ctx      context.Context
		store    *fakeStore
		mutator  *Mutator
		now      time.Time
		human    tailscale.IdentityValue
		marker   *apiv1.MapMarker
		mapState *apiv1.Map
		err      error
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 6, 12, 20, 30, 0, 0, time.UTC)
		store = newFakeStore()
		store.pages["garden_plan"] = wikipage.FrontMatter{"title": "Garden Plan"}
		human = tailscale.NewIdentity("alice@example.com", "Alice", "alice-node")
		mutator = New(store, fixedClock{now: now}, ulid.FixedGenerator("01JMAPMARKER0000000000001"))
	})

	Describe("AddMarker", func() {
		BeforeEach(func() {
			marker, mapState, err = mutator.AddMarker(ctx, "garden_plan", "yard", &apiv1.MapMarker{
				Label: "Shed",
				Position: &apiv1.GeoPoint{
					Lat: 41.1,
					Lon: -72.2,
				},
				PopupMarkdown: "[Shed](shed)",
				Color:         "green",
			}, nil, human)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the marker with server-derived metadata", func() {
			Expect(marker.GetMetadata().GetUid()).To(Equal("01JMAPMARKER0000000000001"))
			Expect(marker.GetMetadata().GetCreatedAt().AsTime()).To(Equal(now))
			Expect(marker.GetMetadata().GetUpdatedAt().AsTime()).To(Equal(now))
			Expect(marker.GetMetadata().GetCreatedBy()).To(Equal("alice@example.com"))
			Expect(marker.GetMetadata().GetAutomated()).To(BeFalse())
			Expect(marker.GetMetadata().GetSortOrder()).To(Equal(int64(1000)))
		})

		It("should return the updated map", func() {
			Expect(mapState.GetName()).To(Equal("yard"))
			Expect(mapState.GetSyncToken()).To(Equal(int64(1)))
			Expect(mapState.GetUpdatedAt().AsTime()).To(Equal(now))
			Expect(mapState.GetMarkers()).To(HaveLen(1))
		})

		It("should write once", func() {
			Expect(store.writes).To(Equal(1))
		})

		It("should write user-mutable marker data under maps", func() {
			written := store.pages["garden_plan"]
			maps, ok := written["maps"].(map[string]any)
			Expect(ok).To(BeTrue())
			yard, ok := maps["yard"].(map[string]any)
			Expect(ok).To(BeTrue())
			markers, ok := yard["markers"].([]any)
			Expect(ok).To(BeTrue())
			Expect(markers).To(HaveLen(1))
			first, ok := markers[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(first["label"]).To(Equal("Shed"))
			Expect(first["lat"]).To(Equal(41.1))
			Expect(first["lon"]).To(Equal(-72.2))
			Expect(first["popup_markdown"]).To(Equal("[Shed](shed)"))
			Expect(first["color"]).To(Equal("green"))
		})

		It("should write wiki-managed metadata under agent.maps", func() {
			written := store.pages["garden_plan"]
			agent, ok := written["agent"].(map[string]any)
			Expect(ok).To(BeTrue())
			agentMaps, ok := agent["maps"].(map[string]any)
			Expect(ok).To(BeTrue())
			yard, ok := agentMaps["yard"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(yard["updated_at"]).To(Equal(now.Format(time.RFC3339Nano)))
			Expect(yard["sync_token"]).To(Equal(int64(1)))
			markers, ok := yard["markers"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(markers).To(HaveKey("01JMAPMARKER0000000000001"))
		})
	})

	Describe("when adding a marker with invalid latitude", func() {
		BeforeEach(func() {
			marker, mapState, err = mutator.AddMarker(ctx, "garden_plan", "yard", &apiv1.MapMarker{
				Label:    "Bad",
				Position: &apiv1.GeoPoint{Lat: 91, Lon: -72.2},
			}, nil, human)
		})

		It("should return InvalidArgument", func() {
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			Expect(err.Error()).To(ContainSubstring("latitude"))
		})

		It("should not return a marker", func() {
			Expect(marker).To(BeNil())
		})

		It("should not return a map", func() {
			Expect(mapState).To(BeNil())
		})

		It("should not write", func() {
			Expect(store.writes).To(Equal(0))
		})
	})

	Describe("SetMapStyle", func() {
		BeforeEach(func() {
			mapState, err = mutator.SetMapStyle(ctx, "garden_plan", "yard", &apiv1.MapStyle{
				TileLayerId: apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
				AspectRatio: "3:2",
			}, nil)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the configured aspect ratio", func() {
			Expect(mapState.GetStyle().GetAspectRatio()).To(Equal("3:2"))
		})

		It("should write the aspect ratio to frontmatter", func() {
			written := store.pages["garden_plan"]
			maps, ok := written["maps"].(map[string]any)
			Expect(ok).To(BeTrue())
			yard, ok := maps["yard"].(map[string]any)
			Expect(ok).To(BeTrue())
			style, ok := yard["style"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(style["aspect_ratio"]).To(Equal("3:2"))
		})
	})

	Describe("when setting an invalid aspect ratio", func() {
		BeforeEach(func() {
			mapState, err = mutator.SetMapStyle(ctx, "garden_plan", "yard", &apiv1.MapStyle{
				TileLayerId: apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
				AspectRatio: "wide",
			}, nil)
		})

		It("should return InvalidArgument", func() {
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			Expect(err.Error()).To(ContainSubstring("aspect_ratio"))
		})

		It("should not return a map", func() {
			Expect(mapState).To(BeNil())
		})

		It("should not write", func() {
			Expect(store.writes).To(Equal(0))
		})
	})
})

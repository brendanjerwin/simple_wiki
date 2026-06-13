package mapmutator

import (
	"context"
	"errors"
	"math"
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

	Describe("map element lifecycle", func() {
		var (
			fetched           *apiv1.Map
			outlines          []*apiv1.MapOutline
			deletedLookupErr  error
			createdMarkerUID  string
			createdPolygonUID string
			createdCircleUID  string
		)

		BeforeEach(func() {
			mutator = New(store, fixedClock{now: now}, ulid.NewSequenceGenerator(
				"01JMAPMARKER0000000000001",
				"01JMAPMARKER0000000000002",
				"01JMAPMARKER0000000000003",
			))

			mapState, err = mutator.SetMapView(ctx, "garden_plan", "yard", &apiv1.MapView{
				Center: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				Zoom:   12,
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			mapState, err = mutator.SetMapStyle(ctx, "garden_plan", "yard", &apiv1.MapStyle{
				TileLayerId: apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
				AspectRatio: "4:3",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			marker, mapState, err = mutator.AddMarker(ctx, "garden_plan", "yard", &apiv1.MapMarker{
				Label:         "Shed",
				Position:      &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				PopupMarkdown: "Old shed",
				Color:         "red",
			}, nil, human)
			Expect(err).NotTo(HaveOccurred())
			createdMarkerUID = marker.GetMetadata().GetUid()

			marker, mapState, err = mutator.UpdateMarker(ctx, "garden_plan", "yard", createdMarkerUID, &apiv1.MapMarker{
				Label:         "Greenhouse",
				Position:      &apiv1.GeoPoint{Lat: 41.2, Lon: -72.3},
				PopupMarkdown: "Updated shed",
				Color:         "green",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			marker, mapState, err = mutator.MoveMarker(ctx, "garden_plan", "yard", createdMarkerUID, &apiv1.GeoPoint{Lat: 41.3, Lon: -72.4}, nil)
			Expect(err).NotTo(HaveOccurred())

			polygon, _, err := mutator.AddPolygon(ctx, "garden_plan", "yard", &apiv1.MapPolygon{
				Label: "Fence",
				Points: []*apiv1.GeoPoint{
					{Lat: 41.0, Lon: -72.0},
					{Lat: 41.0, Lon: -72.1},
					{Lat: 41.1, Lon: -72.1},
				},
				PopupMarkdown: "Fence line",
				StrokeColor:   "#2563eb",
				FillColor:     "#60a5fa",
			}, nil, human)
			Expect(err).NotTo(HaveOccurred())
			createdPolygonUID = polygon.GetMetadata().GetUid()

			_, _, err = mutator.UpdatePolygon(ctx, "garden_plan", "yard", createdPolygonUID, &apiv1.MapPolygon{
				Label: "Garden",
				Points: []*apiv1.GeoPoint{
					{Lat: 41.2, Lon: -72.2},
					{Lat: 41.2, Lon: -72.3},
					{Lat: 41.3, Lon: -72.3},
				},
				PopupMarkdown: "Garden bed",
				StrokeColor:   "#166534",
				FillColor:     "#86efac",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			circle, _, err := mutator.AddCircle(ctx, "garden_plan", "yard", &apiv1.MapCircle{
				Label:         "Sprinkler",
				Center:        &apiv1.GeoPoint{Lat: 41.4, Lon: -72.5},
				RadiusMeters:  25,
				PopupMarkdown: "Watering zone",
				StrokeColor:   "#0369a1",
				FillColor:     "#7dd3fc",
			}, nil, human)
			Expect(err).NotTo(HaveOccurred())
			createdCircleUID = circle.GetMetadata().GetUid()

			_, _, err = mutator.UpdateCircle(ctx, "garden_plan", "yard", createdCircleUID, &apiv1.MapCircle{
				Label:         "Pump range",
				Center:        &apiv1.GeoPoint{Lat: 41.5, Lon: -72.6},
				RadiusMeters:  40,
				PopupMarkdown: "Updated range",
				StrokeColor:   "#7c2d12",
				FillColor:     "#fdba74",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			mapState, err = mutator.ReorderElement(ctx, "garden_plan", "yard", apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER, createdMarkerUID, 5000, nil)
			Expect(err).NotTo(HaveOccurred())

			fetched, err = mutator.GetMap(ctx, "garden_plan", "yard")
			Expect(err).NotTo(HaveOccurred())

			outlines, err = mutator.ListMaps(ctx, "garden_plan")
			Expect(err).NotTo(HaveOccurred())

			mapState, err = mutator.DeleteMarker(ctx, "garden_plan", "yard", createdMarkerUID, nil)
			Expect(err).NotTo(HaveOccurred())
			mapState, err = mutator.DeletePolygon(ctx, "garden_plan", "yard", createdPolygonUID, nil)
			Expect(err).NotTo(HaveOccurred())
			mapState, err = mutator.DeleteCircle(ctx, "garden_plan", "yard", createdCircleUID, nil)
			Expect(err).NotTo(HaveOccurred())
			err = mutator.DeleteMap(ctx, "garden_plan", "yard", nil)
			Expect(err).NotTo(HaveOccurred())
			_, deletedLookupErr = mutator.GetMap(ctx, "garden_plan", "yard")
		})

		It("should persist and decode the map view", func() {
			Expect(fetched.GetView().GetCenter().GetLat()).To(Equal(41.1))
			Expect(fetched.GetView().GetCenter().GetLon()).To(Equal(-72.2))
			Expect(fetched.GetView().GetZoom()).To(Equal(float64(12)))
		})

		It("should persist and decode the map style", func() {
			Expect(fetched.GetStyle().GetTileLayerId()).To(Equal(apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP))
			Expect(fetched.GetStyle().GetAspectRatio()).To(Equal("4:3"))
			Expect(fetched.GetStyle().GetAvailableTileLayers()).NotTo(BeEmpty())
		})

		It("should persist and decode updated marker data", func() {
			Expect(fetched.GetMarkers()).To(HaveLen(1))
			Expect(fetched.GetMarkers()[0].GetLabel()).To(Equal("Greenhouse"))
			Expect(fetched.GetMarkers()[0].GetPosition().GetLat()).To(Equal(41.3))
			Expect(fetched.GetMarkers()[0].GetMetadata().GetSortOrder()).To(Equal(int64(5000)))
		})

		It("should persist and decode updated polygon data", func() {
			Expect(fetched.GetPolygons()).To(HaveLen(1))
			Expect(fetched.GetPolygons()[0].GetLabel()).To(Equal("Garden"))
			Expect(fetched.GetPolygons()[0].GetPoints()).To(HaveLen(3))
			Expect(fetched.GetPolygons()[0].GetStrokeColor()).To(Equal("#166534"))
		})

		It("should persist and decode updated circle data", func() {
			Expect(fetched.GetCircles()).To(HaveLen(1))
			Expect(fetched.GetCircles()[0].GetLabel()).To(Equal("Pump range"))
			Expect(fetched.GetCircles()[0].GetRadiusMeters()).To(Equal(float64(40)))
			Expect(fetched.GetCircles()[0].GetFillColor()).To(Equal("#fdba74"))
		})

		It("should list the map outline counts", func() {
			Expect(outlines).To(HaveLen(1))
			Expect(outlines[0].GetName()).To(Equal("yard"))
			Expect(outlines[0].GetMarkerCount()).To(Equal(int32(1)))
			Expect(outlines[0].GetPolygonCount()).To(Equal(int32(1)))
			Expect(outlines[0].GetCircleCount()).To(Equal(int32(1)))
		})

		It("should delete the map", func() {
			Expect(deletedLookupErr).To(MatchError(ErrMapNotFound))
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

	Describe("when adding a marker with NaN latitude", func() {
		BeforeEach(func() {
			marker, mapState, err = mutator.AddMarker(ctx, "garden_plan", "yard", &apiv1.MapMarker{
				Label:    "Bad",
				Position: &apiv1.GeoPoint{Lat: math.NaN(), Lon: -72.2},
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

	Describe("when setting a view with NaN zoom", func() {
		BeforeEach(func() {
			mapState, err = mutator.SetMapView(ctx, "garden_plan", "yard", &apiv1.MapView{
				Center: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				Zoom:   math.NaN(),
			}, nil)
		})

		It("should return InvalidArgument", func() {
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			Expect(err.Error()).To(ContainSubstring("zoom"))
		})

		It("should not return a map", func() {
			Expect(mapState).To(BeNil())
		})

		It("should not write", func() {
			Expect(store.writes).To(Equal(0))
		})
	})

	Describe("ReplaceMarkers", func() {
		BeforeEach(func() {
			mutator = New(store, fixedClock{now: now}, ulid.NewSequenceGenerator(
				"01JMAPMARKER0000000000001",
				"01JMAPMARKER0000000000002",
			))
			marker, mapState, err = mutator.AddMarker(ctx, "garden_plan", "yard", &apiv1.MapMarker{
				Label:    "Shed",
				Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
			}, nil, human)
			Expect(err).NotTo(HaveOccurred())

			mapState, err = mutator.ReplaceMarkers(ctx, "garden_plan", "yard", []*apiv1.MapMarker{
				{Label: "Shed", Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2}},
				{Label: "Shed", Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2}},
			}, nil, human)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the existing uid for only one duplicate marker", func() {
			Expect(mapState.GetMarkers()).To(HaveLen(2))
			Expect(mapState.GetMarkers()[0].GetMetadata().GetUid()).To(Equal(marker.GetMetadata().GetUid()))
			Expect(mapState.GetMarkers()[1].GetMetadata().GetUid()).NotTo(Equal(marker.GetMetadata().GetUid()))
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

//revive:disable:dot-imports
package v1_test

import (
	"context"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/mapmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
)

const (
	mapClockYear   = 2026
	mapClockMonth  = time.June
	mapClockDay    = 12
	mapClockHour   = 20
	mapClockMinute = 45
)

type mapSteadyClock struct{}

func (mapSteadyClock) Now() time.Time {
	return time.Date(mapClockYear, mapClockMonth, mapClockDay, mapClockHour, mapClockMinute, 0, 0, time.UTC)
}

func newMapTestServer(store *MockPageReaderMutator) *v1.Server {
	return mustNewServer(store, nil, nil).
		WithMapMutator(mapmutator.New(store, mapSteadyClock{}, ulid.FixedGenerator("01JMAPMARKER0000000000001")))
}

var _ = Describe("MapService handlers", func() {
	var (
		ctx    context.Context
		store  *MockPageReaderMutator
		server *v1.Server
		err    error
	)

	BeforeEach(func() {
		ctx = tailscale.ContextWithIdentity(context.Background(), tailscale.NewIdentity("bob@example.com", "Bob", "bob-node"))
		store = &MockPageReaderMutator{Frontmatter: wikipage.FrontMatter{"title": "Garden Plan"}}
		server = newMapTestServer(store)
	})

	Describe("AddMarker", func() {
		var resp *apiv1.AddMarkerResponse

		BeforeEach(func() {
			resp, err = server.AddMarker(ctx, &apiv1.AddMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Marker: &apiv1.MapMarker{
					Label:    "Shed",
					Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				},
			})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the created marker", func() {
			Expect(resp.GetMarker().GetMetadata().GetUid()).To(Equal("01JMAPMARKER0000000000001"))
			Expect(resp.GetMarker().GetMetadata().GetCreatedBy()).To(Equal("bob@example.com"))
		})

		It("should return the updated map", func() {
			Expect(resp.GetMap().GetName()).To(Equal("yard"))
			Expect(resp.GetMap().GetMarkers()).To(HaveLen(1))
		})
	})

	Describe("GetMap", func() {
		var resp *apiv1.GetMapResponse

		BeforeEach(func() {
			_, err = server.AddMarker(ctx, &apiv1.AddMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Marker: &apiv1.MapMarker{
					Label:    "Shed",
					Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err = server.GetMap(ctx, &apiv1.GetMapRequest{Page: "garden_plan", MapName: "yard"})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the map", func() {
			Expect(resp.GetMap().GetMarkers()).To(HaveLen(1))
		})
	})

	Describe("map element lifecycle", func() {
		var (
			setViewResp          *apiv1.SetMapViewResponse
			setStyleResp         *apiv1.SetMapStyleResponse
			addMarkerResp        *apiv1.AddMarkerResponse
			updateMarkerResp     *apiv1.UpdateMarkerResponse
			moveMarkerResp       *apiv1.MoveMarkerResponse
			addPolygonResp       *apiv1.AddPolygonResponse
			updatePolygonResp    *apiv1.UpdatePolygonResponse
			addCircleResp        *apiv1.AddCircleResponse
			updateCircleResp     *apiv1.UpdateCircleResponse
			reorderResp          *apiv1.ReorderElementResponse
			replaceMarkersResp   *apiv1.ReplaceMarkersResponse
			listMapsResp         *apiv1.ListMapsResponse
			listElementsResp     *apiv1.ListMapElementsResponse
			pagedElementsResp    *apiv1.ListMapElementsResponse
			getMarkerResp        *apiv1.GetElementResponse
			getPolygonResp       *apiv1.GetElementResponse
			getCircleResp        *apiv1.GetElementResponse
			findMarkersResp      *apiv1.FindMarkersResponse
			filteredMapResp      *apiv1.GetMapResponse
			deleteMarkerResp     *apiv1.DeleteMarkerResponse
			deletePolygonResp    *apiv1.DeletePolygonResponse
			deleteCircleResp     *apiv1.DeleteCircleResponse
			deleteMapResp        *apiv1.DeleteMapResponse
			missingElementErr    error
			invalidPageTokenErr  error
			createdMarkerUID     string
			createdPolygonUID    string
			createdCircleUID     string
			replacementMarkerUID string
		)

		BeforeEach(func() {
			server = mustNewServer(store, nil, nil).
				WithMapMutator(mapmutator.New(store, mapSteadyClock{}, ulid.NewSequenceGenerator(
					"01JMAPMARKER0000000000001",
					"01JMAPMARKER0000000000002",
					"01JMAPMARKER0000000000003",
					"01JMAPMARKER0000000000004",
				)))

			setViewResp, err = server.SetMapView(ctx, &apiv1.SetMapViewRequest{
				Page:    "garden_plan",
				MapName: "yard",
				View: &apiv1.MapView{
					Center: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
					Zoom:   12,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			setStyleResp, err = server.SetMapStyle(ctx, &apiv1.SetMapStyleRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Style: &apiv1.MapStyle{
					TileLayerId: apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
					AspectRatio: "16:9",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			addMarkerResp, err = server.AddMarker(ctx, &apiv1.AddMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Marker: &apiv1.MapMarker{
					Label:         "Shed",
					Position:      &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
					PopupMarkdown: "Tool shed",
					Color:         "green",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			createdMarkerUID = addMarkerResp.GetMarker().GetMetadata().GetUid()

			updateMarkerResp, err = server.UpdateMarker(ctx, &apiv1.UpdateMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     createdMarkerUID,
				Marker: &apiv1.MapMarker{
					Label:         "Greenhouse",
					Position:      &apiv1.GeoPoint{Lat: 41.2, Lon: -72.3},
					PopupMarkdown: "Seed starts",
					Color:         "red",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			moveMarkerResp, err = server.MoveMarker(ctx, &apiv1.MoveMarkerRequest{
				Page:     "garden_plan",
				MapName:  "yard",
				Uid:      createdMarkerUID,
				Position: &apiv1.GeoPoint{Lat: 41.25, Lon: -72.35},
			})
			Expect(err).NotTo(HaveOccurred())

			addPolygonResp, err = server.AddPolygon(ctx, &apiv1.AddPolygonRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Polygon: &apiv1.MapPolygon{
					Label: "Fence",
					Points: []*apiv1.GeoPoint{
						{Lat: 41.0, Lon: -72.0},
						{Lat: 41.0, Lon: -72.1},
						{Lat: 41.1, Lon: -72.1},
					},
					PopupMarkdown: "Old fence",
					StrokeColor:   "#2563eb",
					FillColor:     "#60a5fa",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			createdPolygonUID = addPolygonResp.GetPolygon().GetMetadata().GetUid()

			updatePolygonResp, err = server.UpdatePolygon(ctx, &apiv1.UpdatePolygonRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     createdPolygonUID,
				Polygon: &apiv1.MapPolygon{
					Label: "Garden",
					Points: []*apiv1.GeoPoint{
						{Lat: 41.2, Lon: -72.2},
						{Lat: 41.2, Lon: -72.3},
						{Lat: 41.3, Lon: -72.3},
					},
					PopupMarkdown: "Garden bed",
					StrokeColor:   "#166534",
					FillColor:     "#86efac",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			addCircleResp, err = server.AddCircle(ctx, &apiv1.AddCircleRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Circle: &apiv1.MapCircle{
					Label:         "Sprinkler",
					Center:        &apiv1.GeoPoint{Lat: 41.4, Lon: -72.5},
					RadiusMeters:  25,
					PopupMarkdown: "Watering zone",
					StrokeColor:   "#0369a1",
					FillColor:     "#7dd3fc",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			createdCircleUID = addCircleResp.GetCircle().GetMetadata().GetUid()

			updateCircleResp, err = server.UpdateCircle(ctx, &apiv1.UpdateCircleRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     createdCircleUID,
				Circle: &apiv1.MapCircle{
					Label:         "Pump range",
					Center:        &apiv1.GeoPoint{Lat: 41.5, Lon: -72.6},
					RadiusMeters:  40,
					PopupMarkdown: "Updated range",
					StrokeColor:   "#7c2d12",
					FillColor:     "#fdba74",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			reorderResp, err = server.ReorderElement(ctx, &apiv1.ReorderElementRequest{
				Page:      "garden_plan",
				MapName:   "yard",
				Type:      apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER,
				Uid:       createdMarkerUID,
				SortOrder: 5000,
			})
			Expect(err).NotTo(HaveOccurred())

			replaceMarkersResp, err = server.ReplaceMarkers(ctx, &apiv1.ReplaceMarkersRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Markers: []*apiv1.MapMarker{
					{
						Metadata: &apiv1.MapElementMetadata{Uid: createdMarkerUID},
						Label:    "Greenhouse",
						Position: &apiv1.GeoPoint{Lat: 41.25, Lon: -72.35},
						Color:    "red",
					},
					{
						Label:    "Gate",
						Position: &apiv1.GeoPoint{Lat: 41.6, Lon: -72.7},
						Color:    "blue",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			replacementMarkerUID = replaceMarkersResp.GetMap().GetMarkers()[1].GetMetadata().GetUid()

			listMapsResp, err = server.ListMaps(ctx, &apiv1.ListMapsRequest{Page: "garden_plan"})
			Expect(err).NotTo(HaveOccurred())

			listElementsResp, err = server.ListMapElements(ctx, &apiv1.ListMapElementsRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Limit:   2,
			})
			Expect(err).NotTo(HaveOccurred())

			pagedElementsResp, err = server.ListMapElements(ctx, &apiv1.ListMapElementsRequest{
				Page:      "garden_plan",
				MapName:   "yard",
				PageToken: listElementsResp.GetNextPageToken(),
			})
			Expect(err).NotTo(HaveOccurred())

			getMarkerResp, err = server.GetElement(ctx, &apiv1.GetElementRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Type:    apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER,
				Uid:     replacementMarkerUID,
			})
			Expect(err).NotTo(HaveOccurred())

			getPolygonResp, err = server.GetElement(ctx, &apiv1.GetElementRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Type:    apiv1.MapElementType_MAP_ELEMENT_TYPE_POLYGON,
				Uid:     createdPolygonUID,
			})
			Expect(err).NotTo(HaveOccurred())

			getCircleResp, err = server.GetElement(ctx, &apiv1.GetElementRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Type:    apiv1.MapElementType_MAP_ELEMENT_TYPE_CIRCLE,
				Uid:     createdCircleUID,
			})
			Expect(err).NotTo(HaveOccurred())

			findMarkersResp, err = server.FindMarkers(ctx, &apiv1.FindMarkersRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Query:   "gate",
				Bbox: &apiv1.BoundingBox{
					SouthWest: &apiv1.GeoPoint{Lat: 41.55, Lon: -72.75},
					NorthEast: &apiv1.GeoPoint{Lat: 41.65, Lon: -72.65},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			filteredMapResp, err = server.GetMap(ctx, &apiv1.GetMapRequest{
				Page:            "garden_plan",
				MapName:         "yard",
				IncludePolygons: true,
				Bbox: &apiv1.BoundingBox{
					SouthWest: &apiv1.GeoPoint{Lat: 41.15, Lon: -72.35},
					NorthEast: &apiv1.GeoPoint{Lat: 41.35, Lon: -72.15},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, invalidPageTokenErr = server.ListMapElements(ctx, &apiv1.ListMapElementsRequest{
				Page:      "garden_plan",
				MapName:   "yard",
				PageToken: "bad",
			})

			_, missingElementErr = server.GetElement(ctx, &apiv1.GetElementRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Type:    apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER,
				Uid:     "missing",
			})

			deleteMarkerResp, err = server.DeleteMarker(ctx, &apiv1.DeleteMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     replacementMarkerUID,
			})
			Expect(err).NotTo(HaveOccurred())

			deletePolygonResp, err = server.DeletePolygon(ctx, &apiv1.DeletePolygonRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     createdPolygonUID,
			})
			Expect(err).NotTo(HaveOccurred())

			deleteCircleResp, err = server.DeleteCircle(ctx, &apiv1.DeleteCircleRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Uid:     createdCircleUID,
			})
			Expect(err).NotTo(HaveOccurred())

			deleteMapResp, err = server.DeleteMap(ctx, &apiv1.DeleteMapRequest{Page: "garden_plan", MapName: "yard"})
		})

		It("should update the map view", func() {
			Expect(setViewResp.GetMap().GetView().GetZoom()).To(Equal(float64(12)))
		})

		It("should update the map style", func() {
			Expect(setStyleResp.GetMap().GetStyle().GetTileLayerId()).To(Equal(apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP))
			Expect(setStyleResp.GetMap().GetStyle().GetAspectRatio()).To(Equal("16:9"))
		})

		It("should update markers", func() {
			Expect(updateMarkerResp.GetMarker().GetLabel()).To(Equal("Greenhouse"))
			Expect(moveMarkerResp.GetMarker().GetPosition().GetLat()).To(Equal(41.25))
			Expect(reorderResp.GetMap().GetMarkers()[0].GetMetadata().GetSortOrder()).To(Equal(int64(5000)))
		})

		It("should replace markers", func() {
			Expect(replaceMarkersResp.GetMap().GetMarkers()).To(HaveLen(2))
			Expect(replaceMarkersResp.GetMap().GetMarkers()[0].GetMetadata().GetUid()).To(Equal(createdMarkerUID))
			Expect(replaceMarkersResp.GetMap().GetMarkers()[1].GetMetadata().GetUid()).NotTo(BeEmpty())
		})

		It("should update polygons", func() {
			Expect(updatePolygonResp.GetPolygon().GetLabel()).To(Equal("Garden"))
			Expect(updatePolygonResp.GetPolygon().GetStrokeColor()).To(Equal("#166534"))
		})

		It("should update circles", func() {
			Expect(updateCircleResp.GetCircle().GetLabel()).To(Equal("Pump range"))
			Expect(updateCircleResp.GetCircle().GetRadiusMeters()).To(Equal(float64(40)))
		})

		It("should list map outlines", func() {
			Expect(listMapsResp.GetMaps()).To(HaveLen(1))
			Expect(listMapsResp.GetMaps()[0].GetName()).To(Equal("yard"))
			Expect(listMapsResp.GetMaps()[0].GetMarkerCount()).To(Equal(int32(2)))
		})

		It("should page element outlines", func() {
			Expect(listElementsResp.GetElements()).To(HaveLen(2))
			Expect(listElementsResp.GetNextPageToken()).To(Equal("2"))
			Expect(pagedElementsResp.GetElements()).NotTo(BeEmpty())
		})

		It("should get elements by type", func() {
			Expect(getMarkerResp.GetMarker().GetLabel()).To(Equal("Gate"))
			Expect(getPolygonResp.GetPolygon().GetLabel()).To(Equal("Garden"))
			Expect(getCircleResp.GetCircle().GetLabel()).To(Equal("Pump range"))
		})

		It("should find markers by query and bounds", func() {
			Expect(findMarkersResp.GetMarkers()).To(HaveLen(1))
			Expect(findMarkersResp.GetMarkers()[0].GetLabel()).To(Equal("Gate"))
		})

		It("should filter maps by requested element types and bounds", func() {
			Expect(filteredMapResp.GetMap().GetMarkers()).To(BeEmpty())
			Expect(filteredMapResp.GetMap().GetPolygons()).To(HaveLen(1))
			Expect(filteredMapResp.GetMap().GetCircles()).To(BeEmpty())
		})

		It("should validate page tokens", func() {
			Expect(invalidPageTokenErr).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page_token"))
		})

		It("should return not found for missing elements", func() {
			Expect(missingElementErr).To(HaveGrpcStatusWithSubstr(codes.NotFound, "map element not found"))
		})

		It("should delete elements", func() {
			Expect(deleteMarkerResp.GetMap().GetMarkers()).To(HaveLen(1))
			Expect(deletePolygonResp.GetMap().GetPolygons()).To(BeEmpty())
			Expect(deleteCircleResp.GetMap().GetCircles()).To(BeEmpty())
		})

		It("should delete the map", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteMapResp).NotTo(BeNil())
		})
	})

	Describe("when a request is nil", func() {
		var errs []error

		BeforeEach(func() {
			errs = nil
			_, err = server.SetMapView(ctx, nil)
			errs = append(errs, err)
			_, err = server.SetMapStyle(ctx, nil)
			errs = append(errs, err)
			_, err = server.AddMarker(ctx, nil)
			errs = append(errs, err)
			_, err = server.UpdateMarker(ctx, nil)
			errs = append(errs, err)
			_, err = server.MoveMarker(ctx, nil)
			errs = append(errs, err)
			_, err = server.DeleteMarker(ctx, nil)
			errs = append(errs, err)
			_, err = server.AddPolygon(ctx, nil)
			errs = append(errs, err)
			_, err = server.UpdatePolygon(ctx, nil)
			errs = append(errs, err)
			_, err = server.DeletePolygon(ctx, nil)
			errs = append(errs, err)
			_, err = server.AddCircle(ctx, nil)
			errs = append(errs, err)
			_, err = server.UpdateCircle(ctx, nil)
			errs = append(errs, err)
			_, err = server.DeleteCircle(ctx, nil)
			errs = append(errs, err)
			_, err = server.ReorderElement(ctx, nil)
			errs = append(errs, err)
			_, err = server.ReplaceMarkers(ctx, nil)
			errs = append(errs, err)
			_, err = server.DeleteMap(ctx, nil)
			errs = append(errs, err)
			_, err = server.GetMap(ctx, nil)
			errs = append(errs, err)
			_, err = server.ListMaps(ctx, nil)
			errs = append(errs, err)
			_, err = server.ListMapElements(ctx, nil)
			errs = append(errs, err)
			_, err = server.GetElement(ctx, nil)
			errs = append(errs, err)
			_, err = server.FindMarkers(ctx, nil)
			errs = append(errs, err)
		})

		It("should return InvalidArgument for each handler", func() {
			Expect(errs).To(HaveLen(20))
			for _, handlerErr := range errs {
				Expect(handlerErr).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "request is required"))
			}
		})
	})

	Describe("when the request context is canceled before mutation", func() {
		BeforeEach(func() {
			_, err = server.AddMarker(ctx, &apiv1.AddMarkerRequest{
				Page:    "garden_plan",
				MapName: "yard",
				Marker: &apiv1.MapMarker{
					Label:    "Shed",
					Position: &apiv1.GeoPoint{Lat: 41.1, Lon: -72.2},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			canceledCtx, cancel := context.WithCancel(ctx)
			cancel()
			_, err = server.DeleteMap(canceledCtx, &apiv1.DeleteMapRequest{Page: "garden_plan", MapName: "yard"})
		})

		It("should preserve the canceled status code", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.Canceled, "context canceled"))
		})
	})
})

var _ = Describe("MapService handlers - map mutator not configured", func() {
	Describe("AddMarker", func() {
		var err error

		BeforeEach(func() {
			server := mustNewServer(&MockPageReaderMutator{}, nil, nil)
			_, err = server.AddMarker(context.Background(), &apiv1.AddMarkerRequest{})
		})

		It("should return FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "map mutator not configured"))
		})
	})
})

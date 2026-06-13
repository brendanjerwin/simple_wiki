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

package v1

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/trackgeom"
	"github.com/brendanjerwin/simple_wiki/server/mapmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errMapMutatorNotConfigured = status.Error(codes.FailedPrecondition, "map mutator not configured on server")

const defaultMapPageSize = 100

func requireMapRequest[T any](req *T) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return nil
}

type mapElementIncludes struct {
	markers  bool
	polygons bool
	circles  bool
}

// SetMapView implements the SetMapView RPC.
func (s *Server) SetMapView(ctx context.Context, req *apiv1.SetMapViewRequest) (*apiv1.SetMapViewResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.SetMapView(ctx, req.GetPage(), req.GetMapName(), req.GetView(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.SetMapViewResponse{Map: mapState}, nil
}

// SetMapStyle implements the SetMapStyle RPC.
func (s *Server) SetMapStyle(ctx context.Context, req *apiv1.SetMapStyleRequest) (*apiv1.SetMapStyleResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.SetMapStyle(ctx, req.GetPage(), req.GetMapName(), req.GetStyle(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.SetMapStyleResponse{Map: mapState}, nil
}

// AddMarker implements the AddMarker RPC.
func (s *Server) AddMarker(ctx context.Context, req *apiv1.AddMarkerRequest) (*apiv1.AddMarkerResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	identity := tailscale.IdentityFromContext(ctx)
	marker, mapState, err := s.mapMutator.AddMarker(ctx, req.GetPage(), req.GetMapName(), req.GetMarker(), timestampPtr(req.ExpectedUpdatedAt), identity)
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.AddMarkerResponse{Marker: marker, Map: mapState}, nil
}

// UpdateMarker implements the UpdateMarker RPC.
func (s *Server) UpdateMarker(ctx context.Context, req *apiv1.UpdateMarkerRequest) (*apiv1.UpdateMarkerResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	marker, mapState, err := s.mapMutator.UpdateMarker(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), req.GetMarker(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.UpdateMarkerResponse{Marker: marker, Map: mapState}, nil
}

// MoveMarker implements the MoveMarker RPC.
func (s *Server) MoveMarker(ctx context.Context, req *apiv1.MoveMarkerRequest) (*apiv1.MoveMarkerResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	marker, mapState, err := s.mapMutator.MoveMarker(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), req.GetPosition(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.MoveMarkerResponse{Marker: marker, Map: mapState}, nil
}

// DeleteMarker implements the DeleteMarker RPC.
func (s *Server) DeleteMarker(ctx context.Context, req *apiv1.DeleteMarkerRequest) (*apiv1.DeleteMarkerResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.DeleteMarker(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.DeleteMarkerResponse{Map: mapState}, nil
}

// AddPolygon implements the AddPolygon RPC.
func (s *Server) AddPolygon(ctx context.Context, req *apiv1.AddPolygonRequest) (*apiv1.AddPolygonResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	polygon, mapState, err := s.mapMutator.AddPolygon(ctx, req.GetPage(), req.GetMapName(), req.GetPolygon(), timestampPtr(req.ExpectedUpdatedAt), tailscale.IdentityFromContext(ctx))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.AddPolygonResponse{Polygon: polygon, Map: mapState}, nil
}

// UpdatePolygon implements the UpdatePolygon RPC.
func (s *Server) UpdatePolygon(ctx context.Context, req *apiv1.UpdatePolygonRequest) (*apiv1.UpdatePolygonResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	polygon, mapState, err := s.mapMutator.UpdatePolygon(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), req.GetPolygon(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.UpdatePolygonResponse{Polygon: polygon, Map: mapState}, nil
}

// DeletePolygon implements the DeletePolygon RPC.
func (s *Server) DeletePolygon(ctx context.Context, req *apiv1.DeletePolygonRequest) (*apiv1.DeletePolygonResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.DeletePolygon(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.DeletePolygonResponse{Map: mapState}, nil
}

// AddCircle implements the AddCircle RPC.
func (s *Server) AddCircle(ctx context.Context, req *apiv1.AddCircleRequest) (*apiv1.AddCircleResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	circle, mapState, err := s.mapMutator.AddCircle(ctx, req.GetPage(), req.GetMapName(), req.GetCircle(), timestampPtr(req.ExpectedUpdatedAt), tailscale.IdentityFromContext(ctx))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.AddCircleResponse{Circle: circle, Map: mapState}, nil
}

// UpdateCircle implements the UpdateCircle RPC.
func (s *Server) UpdateCircle(ctx context.Context, req *apiv1.UpdateCircleRequest) (*apiv1.UpdateCircleResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	circle, mapState, err := s.mapMutator.UpdateCircle(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), req.GetCircle(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.UpdateCircleResponse{Circle: circle, Map: mapState}, nil
}

// DeleteCircle implements the DeleteCircle RPC.
func (s *Server) DeleteCircle(ctx context.Context, req *apiv1.DeleteCircleRequest) (*apiv1.DeleteCircleResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.DeleteCircle(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.DeleteCircleResponse{Map: mapState}, nil
}

// AddTrack implements the AddTrack RPC.
func (s *Server) AddTrack(ctx context.Context, req *apiv1.AddTrackRequest) (*apiv1.AddTrackResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	if err := s.validateTrackFile(req.GetTrack()); err != nil {
		return nil, err
	}
	identity := tailscale.IdentityFromContext(ctx)
	track, mapState, err := s.mapMutator.AddTrack(ctx, req.GetPage(), req.GetMapName(), req.GetTrack(), timestampPtr(req.ExpectedUpdatedAt), identity)
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.AddTrackResponse{Track: track, Map: mapState}, nil
}

// UpdateTrack implements the UpdateTrack RPC.
func (s *Server) UpdateTrack(ctx context.Context, req *apiv1.UpdateTrackRequest) (*apiv1.UpdateTrackResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	if err := s.validateTrackFile(req.GetTrack()); err != nil {
		return nil, err
	}
	track, mapState, err := s.mapMutator.UpdateTrack(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), req.GetTrack(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.UpdateTrackResponse{Track: track, Map: mapState}, nil
}

func (s *Server) validateTrackFile(track *apiv1.MapTrack) error {
	if track == nil {
		return status.Error(codes.InvalidArgument, "track is required")
	}
	if s.fileStorer == nil {
		return status.Error(codes.FailedPrecondition, "file storage is not configured")
	}
	fileHash := track.GetFileHash()
	format := track.GetFormat()

	if format == "" {
		return status.Error(codes.InvalidArgument, "format is required")
	}
	if fileHash == "" {
		return status.Error(codes.InvalidArgument, "file_hash is required")
	}
	normalizedFormat := trackgeom.TrackFormat(format)
	switch normalizedFormat {
	case trackgeom.TrackFormatGPX, trackgeom.TrackFormatGeoJSON:
	default:
		return status.Errorf(codes.InvalidArgument, "format must be GPX or GeoJSON, got: %s", format)
	}

	reader, err := s.fileStorer.Open(fileHash)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return status.Error(codes.NotFound, "track file not found")
		}
		return status.Errorf(codes.InvalidArgument, "failed to open track file: %v", err)
	}
	defer reader.Close()

	if _, err := trackgeom.Parse(normalizedFormat, reader); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid or corrupt track file format: %v", err)
	}
	return nil
}

// DeleteTrack implements the DeleteTrack RPC.
func (s *Server) DeleteTrack(ctx context.Context, req *apiv1.DeleteTrackRequest) (*apiv1.DeleteTrackResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.DeleteTrack(ctx, req.GetPage(), req.GetMapName(), req.GetUid(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.DeleteTrackResponse{Map: mapState}, nil
}

// GetTrackGeometry implements the GetTrackGeometry RPC.
func (s *Server) GetTrackGeometry(ctx context.Context, req *apiv1.GetTrackGeometryRequest) (*apiv1.GetTrackGeometryResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	mapState, err := s.readAuthorizedMap(ctx, req.GetPage(), req.GetMapName())
	if err != nil {
		return nil, err
	}
	var targetTrack *apiv1.MapTrack
	for _, track := range mapState.GetTracks() {
		if track.GetMetadata().GetUid() == req.GetUid() {
			targetTrack = track
			break
		}
	}
	if targetTrack == nil {
		return nil, status.Error(codes.NotFound, "track element not found")
	}
	if s.fileStorer == nil {
		return nil, status.Error(codes.FailedPrecondition, "file storage is not configured")
	}
	reader, err := s.fileStorer.Open(targetTrack.GetFileHash())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, status.Error(codes.NotFound, "track file not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to open track file: %v", err)
	}
	defer reader.Close()

	parsedSegments, err := trackgeom.Parse(trackgeom.TrackFormat(targetTrack.GetFormat()), reader)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse track geometry: %v", err)
	}

	simplified := (&trackgeom.Track{Segments: parsedSegments}).Simplify(1000)

	segments := make([]*apiv1.TrackSegment, 0, len(simplified.Segments))
	for _, seg := range simplified.Segments {
		points := make([]*apiv1.GeoPoint, 0, len(seg))
		for _, pt := range seg {
			points = append(points, &apiv1.GeoPoint{
				Lat: pt.Lat,
				Lon: pt.Lon,
			})
		}
		segments = append(segments, &apiv1.TrackSegment{
			Points: points,
		})
	}

	return &apiv1.GetTrackGeometryResponse{Segments: segments}, nil
}

// ReorderElement implements the ReorderElement RPC.
func (s *Server) ReorderElement(ctx context.Context, req *apiv1.ReorderElementRequest) (*apiv1.ReorderElementResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.ReorderElement(ctx, req.GetPage(), req.GetMapName(), req.GetType(), req.GetUid(), req.GetSortOrder(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.ReorderElementResponse{Map: mapState}, nil
}

// ReplaceMarkers implements the ReplaceMarkers RPC.
func (s *Server) ReplaceMarkers(ctx context.Context, req *apiv1.ReplaceMarkersRequest) (*apiv1.ReplaceMarkersResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	mapState, err := s.mapMutator.ReplaceMarkers(ctx, req.GetPage(), req.GetMapName(), req.GetMarkers(), timestampPtr(req.ExpectedUpdatedAt), tailscale.IdentityFromContext(ctx))
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.ReplaceMarkersResponse{Map: mapState}, nil
}

// DeleteMap implements the DeleteMap RPC.
func (s *Server) DeleteMap(ctx context.Context, req *apiv1.DeleteMapRequest) (*apiv1.DeleteMapResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if err := s.requireMapMutation(ctx, req.GetPage(), req.GetMapName()); err != nil {
		return nil, err
	}
	if err := s.mapMutator.DeleteMap(ctx, req.GetPage(), req.GetMapName(), timestampPtr(req.ExpectedUpdatedAt)); err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.DeleteMapResponse{}, nil
}

// GetMap implements the GetMap RPC.
func (s *Server) GetMap(ctx context.Context, req *apiv1.GetMapRequest) (*apiv1.GetMapResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	mapState, err := s.readAuthorizedMap(ctx, req.GetPage(), req.GetMapName())
	if err != nil {
		return nil, err
	}
	filterMapResponse(mapState, requestedMapElementIncludes(req), req.GetBbox())
	return &apiv1.GetMapResponse{Map: mapState}, nil
}

// ListMaps implements the ListMaps RPC.
func (s *Server) ListMaps(ctx context.Context, req *apiv1.ListMapsRequest) (*apiv1.ListMapsResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); authErr != nil {
		return nil, authErr
	}
	maps, err := s.mapMutator.ListMaps(ctx, req.GetPage())
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return &apiv1.ListMapsResponse{Maps: maps}, nil
}

// ListMapElements implements the ListMapElements RPC.
func (s *Server) ListMapElements(ctx context.Context, req *apiv1.ListMapElementsRequest) (*apiv1.ListMapElementsResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	mapState, err := s.readAuthorizedMap(ctx, req.GetPage(), req.GetMapName())
	if err != nil {
		return nil, err
	}
	outlines := elementOutlines(mapState, req.GetTypes(), req.GetBbox())
	start, err := parseMapPageToken(req.GetPageToken())
	if err != nil {
		return nil, err
	}
	limit := int(req.GetLimit())
	if limit <= 0 || limit > defaultMapPageSize {
		limit = defaultMapPageSize
	}
	if start > len(outlines) {
		start = len(outlines)
	}
	end := start + limit
	nextToken := ""
	if end < len(outlines) {
		nextToken = strconv.Itoa(end)
	} else {
		end = len(outlines)
	}
	return &apiv1.ListMapElementsResponse{Elements: outlines[start:end], NextPageToken: nextToken}, nil
}

// GetElement implements the GetElement RPC.
func (s *Server) GetElement(ctx context.Context, req *apiv1.GetElementRequest) (*apiv1.GetElementResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	if req.GetUid() == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	mapState, err := s.readAuthorizedMap(ctx, req.GetPage(), req.GetMapName())
	if err != nil {
		return nil, err
	}
	switch req.GetType() {
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER:
		for _, marker := range mapState.GetMarkers() {
			if marker.GetMetadata().GetUid() == req.GetUid() {
				return &apiv1.GetElementResponse{Marker: marker}, nil
			}
		}
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_POLYGON:
		for _, polygon := range mapState.GetPolygons() {
			if polygon.GetMetadata().GetUid() == req.GetUid() {
				return &apiv1.GetElementResponse{Polygon: polygon}, nil
			}
		}
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_CIRCLE:
		for _, circle := range mapState.GetCircles() {
			if circle.GetMetadata().GetUid() == req.GetUid() {
				return &apiv1.GetElementResponse{Circle: circle}, nil
			}
		}
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_TRACK:
		for _, track := range mapState.GetTracks() {
			if track.GetMetadata().GetUid() == req.GetUid() {
				return &apiv1.GetElementResponse{Track: track}, nil
			}
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	return nil, status.Error(codes.NotFound, "map element not found")
}

// FindMarkers implements the FindMarkers RPC.
func (s *Server) FindMarkers(ctx context.Context, req *apiv1.FindMarkersRequest) (*apiv1.FindMarkersResponse, error) {
	if err := requireMapRequest(req); err != nil {
		return nil, err
	}
	mapState, err := s.readAuthorizedMap(ctx, req.GetPage(), req.GetMapName())
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(req.GetQuery()))
	limit := int(req.GetLimit())
	if limit <= 0 || limit > defaultMapPageSize {
		limit = defaultMapPageSize
	}
	markers := make([]*apiv1.MapMarker, 0)
	for _, marker := range mapState.GetMarkers() {
		if !pointInBBox(marker.GetPosition(), req.GetBbox()) {
			continue
		}
		haystack := strings.ToLower(marker.GetLabel() + "\n" + marker.GetPopupMarkdown())
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		markers = append(markers, marker)
		if len(markers) == limit {
			break
		}
	}
	return &apiv1.FindMarkersResponse{Markers: markers}, nil
}

func (s *Server) readAuthorizedMap(ctx context.Context, page, mapName string) (*apiv1.Map, error) {
	if s.mapMutator == nil {
		return nil, errMapMutatorNotConfigured
	}
	if page == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if mapName == "" {
		return nil, status.Error(codes.InvalidArgument, "map_name is required")
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(page)); authErr != nil {
		return nil, authErr
	}
	mapState, err := s.mapMutator.GetMap(ctx, page, mapName)
	if err != nil {
		return nil, mapMapMutatorErr(err)
	}
	return mapState, nil
}

func (s *Server) requireMapMutation(ctx context.Context, page, mapName string) error {
	if page == "" {
		return status.Error(codes.InvalidArgument, errPageRequired)
	}
	if mapName == "" {
		return status.Error(codes.InvalidArgument, "map_name is required")
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(page)); guardErr != nil {
		return guardErr
	}
	return requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(page))
}

func requestedMapElementIncludes(req *apiv1.GetMapRequest) mapElementIncludes {
	includeAll := !req.GetIncludeMarkers() && !req.GetIncludePolygons() && !req.GetIncludeCircles()
	return mapElementIncludes{
		markers:  includeAll || req.GetIncludeMarkers(),
		polygons: includeAll || req.GetIncludePolygons(),
		circles:  includeAll || req.GetIncludeCircles(),
	}
}

func filterMapResponse(mapState *apiv1.Map, includes mapElementIncludes, bbox *apiv1.BoundingBox) {
	if includes.markers {
		mapState.Markers = filterMarkersByBBox(mapState.GetMarkers(), bbox)
	} else {
		mapState.Markers = nil
	}
	if includes.polygons {
		mapState.Polygons = filterPolygonsByBBox(mapState.GetPolygons(), bbox)
	} else {
		mapState.Polygons = nil
	}
	if includes.circles {
		mapState.Circles = filterCirclesByBBox(mapState.GetCircles(), bbox)
	} else {
		mapState.Circles = nil
	}
}

func elementOutlines(mapState *apiv1.Map, types []apiv1.MapElementType, bbox *apiv1.BoundingBox) []*apiv1.MapElementOutline {
	typeSet := map[apiv1.MapElementType]bool{}
	for _, elementType := range types {
		typeSet[elementType] = true
	}
	includeType := func(elementType apiv1.MapElementType) bool {
		return len(typeSet) == 0 || typeSet[elementType]
	}
	out := []*apiv1.MapElementOutline{}
	if includeType(apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER) {
		for _, marker := range filterMarkersByBBox(mapState.GetMarkers(), bbox) {
			out = append(out, &apiv1.MapElementOutline{
				Uid:                 marker.GetMetadata().GetUid(),
				Type:                apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER,
				Label:               marker.GetLabel(),
				RepresentativePoint: marker.GetPosition(),
				UpdatedAt:           marker.GetMetadata().GetUpdatedAt(),
				SortOrder:           marker.GetMetadata().GetSortOrder(),
			})
		}
	}
	if includeType(apiv1.MapElementType_MAP_ELEMENT_TYPE_POLYGON) {
		for _, polygon := range filterPolygonsByBBox(mapState.GetPolygons(), bbox) {
			out = append(out, &apiv1.MapElementOutline{
				Uid:                 polygon.GetMetadata().GetUid(),
				Type:                apiv1.MapElementType_MAP_ELEMENT_TYPE_POLYGON,
				Label:               polygon.GetLabel(),
				RepresentativePoint: representativePolygonPoint(polygon),
				UpdatedAt:           polygon.GetMetadata().GetUpdatedAt(),
				SortOrder:           polygon.GetMetadata().GetSortOrder(),
			})
		}
	}
	if includeType(apiv1.MapElementType_MAP_ELEMENT_TYPE_CIRCLE) {
		for _, circle := range filterCirclesByBBox(mapState.GetCircles(), bbox) {
			out = append(out, &apiv1.MapElementOutline{
				Uid:                 circle.GetMetadata().GetUid(),
				Type:                apiv1.MapElementType_MAP_ELEMENT_TYPE_CIRCLE,
				Label:               circle.GetLabel(),
				RepresentativePoint: circle.GetCenter(),
				UpdatedAt:           circle.GetMetadata().GetUpdatedAt(),
				SortOrder:           circle.GetMetadata().GetSortOrder(),
			})
		}
	}
	if includeType(apiv1.MapElementType_MAP_ELEMENT_TYPE_TRACK) {
		for _, track := range mapState.GetTracks() {
			out = append(out, &apiv1.MapElementOutline{
				Uid:                 track.GetMetadata().GetUid(),
				Type:                apiv1.MapElementType_MAP_ELEMENT_TYPE_TRACK,
				Label:               track.GetLabel(),
				RepresentativePoint: nil,
				UpdatedAt:           track.GetMetadata().GetUpdatedAt(),
				SortOrder:           track.GetMetadata().GetSortOrder(),
			})
		}
	}
	slices.SortStableFunc(out, func(a, b *apiv1.MapElementOutline) int {
		return compareMapSortOrder(a.GetSortOrder(), b.GetSortOrder())
	})
	return out
}

func compareMapSortOrder(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func parseMapPageToken(pageToken string) (int, error) {
	if pageToken == "" {
		return 0, nil
	}
	start, err := strconv.Atoi(pageToken)
	if err != nil || start < 0 {
		return 0, status.Error(codes.InvalidArgument, "page_token must be a non-negative integer offset")
	}
	return start, nil
}

func filterMarkersByBBox(markers []*apiv1.MapMarker, bbox *apiv1.BoundingBox) []*apiv1.MapMarker {
	out := make([]*apiv1.MapMarker, 0, len(markers))
	for _, marker := range markers {
		if pointInBBox(marker.GetPosition(), bbox) {
			out = append(out, marker)
		}
	}
	return out
}

func filterPolygonsByBBox(polygons []*apiv1.MapPolygon, bbox *apiv1.BoundingBox) []*apiv1.MapPolygon {
	out := make([]*apiv1.MapPolygon, 0, len(polygons))
	for _, polygon := range polygons {
		if polygonTouchesBBox(polygon, bbox) {
			out = append(out, polygon)
		}
	}
	return out
}

func filterCirclesByBBox(circles []*apiv1.MapCircle, bbox *apiv1.BoundingBox) []*apiv1.MapCircle {
	out := make([]*apiv1.MapCircle, 0, len(circles))
	for _, circle := range circles {
		if pointInBBox(circle.GetCenter(), bbox) {
			out = append(out, circle)
		}
	}
	return out
}

func polygonTouchesBBox(polygon *apiv1.MapPolygon, bbox *apiv1.BoundingBox) bool {
	if bbox == nil {
		return true
	}
	for _, point := range polygon.GetPoints() {
		if pointInBBox(point, bbox) {
			return true
		}
	}
	return false
}

func pointInBBox(point *apiv1.GeoPoint, bbox *apiv1.BoundingBox) bool {
	if bbox == nil {
		return true
	}
	if point == nil || bbox.GetSouthWest() == nil || bbox.GetNorthEast() == nil {
		return false
	}
	return point.GetLat() >= bbox.GetSouthWest().GetLat() &&
		point.GetLat() <= bbox.GetNorthEast().GetLat() &&
		point.GetLon() >= bbox.GetSouthWest().GetLon() &&
		point.GetLon() <= bbox.GetNorthEast().GetLon()
}

func representativePolygonPoint(polygon *apiv1.MapPolygon) *apiv1.GeoPoint {
	if len(polygon.GetPoints()) == 0 {
		return nil
	}
	return polygon.GetPoints()[0]
}

func mapMapMutatorErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, mapmutator.ErrElementNotFound):
		return status.Error(codes.NotFound, "map element not found")
	case errors.Is(err, mapmutator.ErrMapNotFound):
		return status.Error(codes.NotFound, "map not found")
	case errors.Is(err, mapmutator.ErrPageNotFound):
		return status.Error(codes.NotFound, "page not found")
	default:
		return status.Errorf(codes.Internal, "map mutation failed: %v", err)
	}
}

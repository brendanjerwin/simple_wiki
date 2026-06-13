// Package mapmutator owns every mutation to first-class wiki map data.
package mapmutator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Clock returns the current time. Production uses SystemClock; tests inject a
// deterministic clock.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the current wall-clock time.
type SystemClock struct{}

// Now returns time.Now.
func (SystemClock) Now() time.Time { return time.Now() }

// SortOrderStep is the conventional spacing between adjacent map elements.
const SortOrderStep int64 = 1000

// Errors returned by the map mutator.
var (
	ErrElementNotFound = errors.New("map element not found")
	ErrMapNotFound     = errors.New("map not found")
	ErrPageNotFound    = errors.New("page not found")
)

// Mutator is the single funnel for map mutations.
type Mutator struct {
	pages wikipage.PageReaderMutator
	clock Clock
	ulids ulid.Generator

	pageMu sync.Map
}

// New constructs a map mutator.
func New(pages wikipage.PageReaderMutator, clock Clock, ulids ulid.Generator) *Mutator {
	return &Mutator{pages: pages, clock: clock, ulids: ulids}
}

// SetMapView creates or updates a map's initial viewport.
func (m *Mutator) SetMapView(ctx context.Context, page, mapName string, view *apiv1.MapView, expected *time.Time) (*apiv1.Map, error) {
	if err := validateView(view); err != nil {
		return nil, err
	}
	return m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		mapState.View = cloneView(view)
		return nil
	})
}

// SetMapStyle creates or updates a map's style.
func (m *Mutator) SetMapStyle(ctx context.Context, page, mapName string, style *apiv1.MapStyle, expected *time.Time) (*apiv1.Map, error) {
	if err := validateStyle(style); err != nil {
		return nil, err
	}
	return m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		mapState.Style = styleWithTileLayer(style.GetTileLayerId())
		return nil
	})
}

// AddMarker adds a marker to a named map, creating the map shell when needed.
func (m *Mutator) AddMarker(ctx context.Context, page, mapName string, marker *apiv1.MapMarker, expected *time.Time, identity tailscale.IdentityValue) (*apiv1.MapMarker, *apiv1.Map, error) {
	if marker == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "marker is required")
	}
	if err := validatePoint(marker.GetPosition(), "position"); err != nil {
		return nil, nil, err
	}
	var created *apiv1.MapMarker
	mapState, err := m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		created = cloneMarker(marker)
		created.Metadata = m.newMetadata(identity, nextSortOrder(totalElementCount(mapState)))
		mapState.Markers = append(mapState.Markers, created)
		return nil
	})
	return created, mapState, err
}

// UpdateMarker updates user-mutable fields on an existing marker.
func (m *Mutator) UpdateMarker(ctx context.Context, page, mapName, uid string, marker *apiv1.MapMarker, expected *time.Time) (*apiv1.MapMarker, *apiv1.Map, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if marker == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "marker is required")
	}
	if err := validatePoint(marker.GetPosition(), "position"); err != nil {
		return nil, nil, err
	}
	var updated *apiv1.MapMarker
	mapState, err := m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		existing := findMarker(mapState.Markers, uid)
		if existing == nil {
			return ErrElementNotFound
		}
		metadata := cloneMetadata(existing.GetMetadata())
		metadata.UpdatedAt = timestamppb.New(m.clock.Now())
		updated = cloneMarker(marker)
		updated.Metadata = metadata
		*existing = *updated
		return nil
	})
	return updated, mapState, err
}

// MoveMarker updates only a marker position.
func (m *Mutator) MoveMarker(ctx context.Context, page, mapName, uid string, position *apiv1.GeoPoint, expected *time.Time) (*apiv1.MapMarker, *apiv1.Map, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if err := validatePoint(position, "position"); err != nil {
		return nil, nil, err
	}
	var moved *apiv1.MapMarker
	mapState, err := m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		existing := findMarker(mapState.Markers, uid)
		if existing == nil {
			return ErrElementNotFound
		}
		existing.Position = clonePoint(position)
		existing.Metadata = cloneMetadata(existing.GetMetadata())
		existing.Metadata.UpdatedAt = timestamppb.New(m.clock.Now())
		moved = cloneMarkerWithMetadata(existing)
		return nil
	})
	return moved, mapState, err
}

// DeleteMarker removes one marker.
func (m *Mutator) DeleteMarker(ctx context.Context, page, mapName, uid string, expected *time.Time) (*apiv1.Map, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	return m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		markers, removed := removeMarker(mapState.Markers, uid)
		if !removed {
			return ErrElementNotFound
		}
		mapState.Markers = markers
		return nil
	})
}

// AddPolygon adds a polygon to a named map.
func (m *Mutator) AddPolygon(ctx context.Context, page, mapName string, polygon *apiv1.MapPolygon, expected *time.Time, identity tailscale.IdentityValue) (*apiv1.MapPolygon, *apiv1.Map, error) {
	if polygon == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "polygon is required")
	}
	if err := validatePolygonPoints(polygon.GetPoints()); err != nil {
		return nil, nil, err
	}
	var created *apiv1.MapPolygon
	mapState, err := m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		created = clonePolygon(polygon)
		created.Metadata = m.newMetadata(identity, nextSortOrder(totalElementCount(mapState)))
		mapState.Polygons = append(mapState.Polygons, created)
		return nil
	})
	return created, mapState, err
}

// UpdatePolygon updates an existing polygon.
func (m *Mutator) UpdatePolygon(ctx context.Context, page, mapName, uid string, polygon *apiv1.MapPolygon, expected *time.Time) (*apiv1.MapPolygon, *apiv1.Map, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if polygon == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "polygon is required")
	}
	if err := validatePolygonPoints(polygon.GetPoints()); err != nil {
		return nil, nil, err
	}
	var updated *apiv1.MapPolygon
	mapState, err := m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		existing := findPolygon(mapState.Polygons, uid)
		if existing == nil {
			return ErrElementNotFound
		}
		metadata := cloneMetadata(existing.GetMetadata())
		metadata.UpdatedAt = timestamppb.New(m.clock.Now())
		updated = clonePolygon(polygon)
		updated.Metadata = metadata
		*existing = *updated
		return nil
	})
	return updated, mapState, err
}

// DeletePolygon removes one polygon.
func (m *Mutator) DeletePolygon(ctx context.Context, page, mapName, uid string, expected *time.Time) (*apiv1.Map, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	return m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		polygons, removed := removePolygon(mapState.Polygons, uid)
		if !removed {
			return ErrElementNotFound
		}
		mapState.Polygons = polygons
		return nil
	})
}

// AddCircle adds a circle to a named map.
func (m *Mutator) AddCircle(ctx context.Context, page, mapName string, circle *apiv1.MapCircle, expected *time.Time, identity tailscale.IdentityValue) (*apiv1.MapCircle, *apiv1.Map, error) {
	if circle == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "circle is required")
	}
	if err := validateCircle(circle); err != nil {
		return nil, nil, err
	}
	var created *apiv1.MapCircle
	mapState, err := m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		created = cloneCircle(circle)
		created.Metadata = m.newMetadata(identity, nextSortOrder(totalElementCount(mapState)))
		mapState.Circles = append(mapState.Circles, created)
		return nil
	})
	return created, mapState, err
}

// UpdateCircle updates an existing circle.
func (m *Mutator) UpdateCircle(ctx context.Context, page, mapName, uid string, circle *apiv1.MapCircle, expected *time.Time) (*apiv1.MapCircle, *apiv1.Map, error) {
	if uid == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if circle == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "circle is required")
	}
	if err := validateCircle(circle); err != nil {
		return nil, nil, err
	}
	var updated *apiv1.MapCircle
	mapState, err := m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		existing := findCircle(mapState.Circles, uid)
		if existing == nil {
			return ErrElementNotFound
		}
		metadata := cloneMetadata(existing.GetMetadata())
		metadata.UpdatedAt = timestamppb.New(m.clock.Now())
		updated = cloneCircle(circle)
		updated.Metadata = metadata
		*existing = *updated
		return nil
	})
	return updated, mapState, err
}

// DeleteCircle removes one circle.
func (m *Mutator) DeleteCircle(ctx context.Context, page, mapName, uid string, expected *time.Time) (*apiv1.Map, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	return m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		circles, removed := removeCircle(mapState.Circles, uid)
		if !removed {
			return ErrElementNotFound
		}
		mapState.Circles = circles
		return nil
	})
}

// ReorderElement updates sparse sort order for one element.
func (m *Mutator) ReorderElement(ctx context.Context, page, mapName string, elementType apiv1.MapElementType, uid string, sortOrder int64, expected *time.Time) (*apiv1.Map, error) {
	if uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if elementType == apiv1.MapElementType_MAP_ELEMENT_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	return m.mutateMap(ctx, page, mapName, expected, requireExistingMap, func(mapState *apiv1.Map) error {
		metadata := metadataForElement(mapState, elementType, uid)
		if metadata == nil {
			return ErrElementNotFound
		}
		metadata.SortOrder = sortOrder
		metadata.UpdatedAt = timestamppb.New(m.clock.Now())
		sortMapElements(mapState)
		return nil
	})
}

// ReplaceMarkers reconciles all markers, preserving uids by natural marker key.
func (m *Mutator) ReplaceMarkers(ctx context.Context, page, mapName string, markers []*apiv1.MapMarker, expected *time.Time, identity tailscale.IdentityValue) (*apiv1.Map, error) {
	for _, marker := range markers {
		if marker == nil {
			return nil, status.Error(codes.InvalidArgument, "markers must not contain nil entries")
		}
		if err := validatePoint(marker.GetPosition(), "position"); err != nil {
			return nil, err
		}
	}
	return m.mutateMap(ctx, page, mapName, expected, allowMissingMap, func(mapState *apiv1.Map) error {
		existingByKey := map[string]*apiv1.MapMarker{}
		for _, marker := range mapState.Markers {
			existingByKey[markerKey(marker)] = marker
		}
		reconciled := make([]*apiv1.MapMarker, 0, len(markers))
		for i, marker := range markers {
			next := cloneMarker(marker)
			if existing := existingByKey[markerKey(marker)]; existing != nil {
				next.Metadata = cloneMetadata(existing.GetMetadata())
				next.Metadata.UpdatedAt = timestamppb.New(m.clock.Now())
			} else {
				next.Metadata = m.newMetadata(identity, nextSortOrder(i))
			}
			reconciled = append(reconciled, next)
		}
		mapState.Markers = reconciled
		return nil
	})
}

// DeleteMap deletes a named map's user and metadata trees.
func (m *Mutator) DeleteMap(ctx context.Context, page, mapName string, expected *time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePageAndMapName(page, mapName); err != nil {
		return err
	}
	release := m.lockPage(page)
	defer release()
	fm, err := m.readFrontMatter(page)
	if err != nil {
		return err
	}
	if !mapExists(fm, mapName) {
		return ErrMapNotFound
	}
	if err := checkExpectedUpdatedAt(decodeMap(fm, page, mapName), expected); err != nil {
		return err
	}
	deleteMapData(fm, mapName)
	if err := m.pages.WriteFrontMatter(wikipage.PageIdentifier(page), fm); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	return nil
}

// GetMap reads a named map with all element types.
func (m *Mutator) GetMap(_ context.Context, page, mapName string) (*apiv1.Map, error) {
	if err := validatePageAndMapName(page, mapName); err != nil {
		return nil, err
	}
	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}
	if !mapExists(fm, mapName) {
		return nil, ErrMapNotFound
	}
	return decodeMap(fm, page, mapName), nil
}

// ListMaps returns compact outlines for every map on a page.
func (m *Mutator) ListMaps(_ context.Context, page string) ([]*apiv1.MapOutline, error) {
	if page == "" {
		return nil, status.Error(codes.InvalidArgument, "page is required")
	}
	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}
	out := []*apiv1.MapOutline{}
	for _, name := range mapNames(fm) {
		mapState := decodeMap(fm, page, name)
		out = append(out, &apiv1.MapOutline{
			Page:         page,
			Name:         name,
			UpdatedAt:    mapState.GetUpdatedAt(),
			SyncToken:    mapState.GetSyncToken(),
			MarkerCount:  int32(len(mapState.GetMarkers())),
			PolygonCount: int32(len(mapState.GetPolygons())),
			CircleCount:  int32(len(mapState.GetCircles())),
		})
	}
	return out, nil
}

type missingMapPolicy int

const (
	requireExistingMap missingMapPolicy = iota
	allowMissingMap
)

func (m *Mutator) mutateMap(ctx context.Context, page, mapName string, expected *time.Time, missingPolicy missingMapPolicy, mutate func(*apiv1.Map) error) (*apiv1.Map, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validatePageAndMapName(page, mapName); err != nil {
		return nil, err
	}
	release := m.lockPage(page)
	defer release()

	fm, err := m.readFrontMatter(page)
	if err != nil {
		return nil, err
	}
	if !mapExists(fm, mapName) && missingPolicy == requireExistingMap {
		return nil, ErrMapNotFound
	}
	mapState := decodeMap(fm, page, mapName)
	if err := checkExpectedUpdatedAt(mapState, expected); err != nil {
		return nil, err
	}
	if err := mutate(mapState); err != nil {
		return nil, err
	}
	mapState.UpdatedAt = timestamppb.New(m.clock.Now())
	mapState.SyncToken++
	encodeMap(fm, mapState)
	if err := m.pages.WriteFrontMatter(wikipage.PageIdentifier(page), fm); err != nil {
		return nil, fmt.Errorf("write frontmatter: %w", err)
	}
	return mapState, nil
}

func (m *Mutator) readFrontMatter(page string) (wikipage.FrontMatter, error) {
	_, fm, err := m.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("read frontmatter: %w", err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	return fm, nil
}

func (m *Mutator) lockPage(page string) func() {
	v, _ := m.pageMu.LoadOrStore(page, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
	}
	mu.Lock()
	return mu.Unlock
}

func (m *Mutator) newMetadata(identity tailscale.IdentityValue, sortOrder int64) *apiv1.MapElementMetadata {
	now := m.clock.Now()
	return &apiv1.MapElementMetadata{
		Uid:       m.ulids.NewULID(),
		CreatedAt: timestamppb.New(now),
		UpdatedAt: timestamppb.New(now),
		CreatedBy: identity.Name(),
		Automated: identity.IsAgent(),
		SortOrder: sortOrder,
	}
}

func validatePageAndMapName(page, mapName string) error {
	if page == "" {
		return status.Error(codes.InvalidArgument, "page is required")
	}
	if mapName == "" {
		return status.Error(codes.InvalidArgument, "map_name is required")
	}
	return nil
}

func validateView(view *apiv1.MapView) error {
	if view == nil {
		return status.Error(codes.InvalidArgument, "view is required")
	}
	if err := validatePoint(view.GetCenter(), "center"); err != nil {
		return err
	}
	if view.GetZoom() < 0 {
		return status.Error(codes.InvalidArgument, "zoom must be non-negative")
	}
	return nil
}

func validateStyle(style *apiv1.MapStyle) error {
	if style == nil {
		return status.Error(codes.InvalidArgument, "style is required")
	}
	if !isKnownTileLayer(style.GetTileLayerId()) {
		return status.Error(codes.InvalidArgument, "tile_layer_id is not supported")
	}
	return nil
}

func validatePoint(point *apiv1.GeoPoint, fieldName string) error {
	if point == nil {
		return status.Errorf(codes.InvalidArgument, "%s is required", fieldName)
	}
	if point.GetLat() < -90 || point.GetLat() > 90 {
		return status.Error(codes.InvalidArgument, "latitude must be between -90 and 90")
	}
	if point.GetLon() < -180 || point.GetLon() > 180 {
		return status.Error(codes.InvalidArgument, "longitude must be between -180 and 180")
	}
	return nil
}

func validatePolygonPoints(points []*apiv1.GeoPoint) error {
	if len(points) < 3 {
		return status.Error(codes.InvalidArgument, "polygon requires at least three points")
	}
	for _, point := range points {
		if err := validatePoint(point, "polygon point"); err != nil {
			return err
		}
	}
	return nil
}

func validateCircle(circle *apiv1.MapCircle) error {
	if err := validatePoint(circle.GetCenter(), "center"); err != nil {
		return err
	}
	if circle.GetRadiusMeters() <= 0 {
		return status.Error(codes.InvalidArgument, "radius_meters must be positive")
	}
	return nil
}

func checkExpectedUpdatedAt(mapState *apiv1.Map, expected *time.Time) error {
	if expected == nil {
		return nil
	}
	if mapState.GetUpdatedAt() == nil {
		return status.Error(codes.FailedPrecondition, "expected_updated_at mismatch: map has no recorded updated_at")
	}
	if !mapState.GetUpdatedAt().AsTime().Equal(*expected) {
		return status.Error(codes.FailedPrecondition, "expected_updated_at mismatch")
	}
	return nil
}

func nextSortOrder(existingCount int) int64 {
	return int64(existingCount+1) * SortOrderStep
}

func totalElementCount(mapState *apiv1.Map) int {
	return len(mapState.GetMarkers()) + len(mapState.GetPolygons()) + len(mapState.GetCircles())
}

func isKnownTileLayer(id apiv1.TileLayerId) bool {
	switch id {
	case apiv1.TileLayerId_TILE_LAYER_ID_OPENSTREETMAP,
		apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
		apiv1.TileLayerId_TILE_LAYER_ID_ESRI_WORLD_IMAGERY:
		return true
	default:
		return false
	}
}

func defaultStyle() *apiv1.MapStyle {
	return styleWithTileLayer(apiv1.TileLayerId_TILE_LAYER_ID_OPENSTREETMAP)
}

func styleWithTileLayer(id apiv1.TileLayerId) *apiv1.MapStyle {
	if !isKnownTileLayer(id) {
		id = apiv1.TileLayerId_TILE_LAYER_ID_OPENSTREETMAP
	}
	return &apiv1.MapStyle{TileLayerId: id, AvailableTileLayers: supportedTileLayers()}
}

func supportedTileLayers() []*apiv1.TileLayer {
	return []*apiv1.TileLayer{
		{
			Id:              apiv1.TileLayerId_TILE_LAYER_ID_OPENSTREETMAP,
			Label:           "OpenStreetMap",
			UrlTemplate:     "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
			AttributionHtml: `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`,
		},
		{
			Id:              apiv1.TileLayerId_TILE_LAYER_ID_OPENTOPOMAP,
			Label:           "OpenTopoMap",
			UrlTemplate:     "https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png",
			AttributionHtml: `Map data: &copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors, SRTM | Map style: &copy; <a href="https://opentopomap.org">OpenTopoMap</a>`,
		},
		{
			Id:              apiv1.TileLayerId_TILE_LAYER_ID_ESRI_WORLD_IMAGERY,
			Label:           "Esri World Imagery",
			UrlTemplate:     "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
			AttributionHtml: "Tiles &copy; Esri",
		},
	}
}

func sortMapElements(mapState *apiv1.Map) {
	sort.SliceStable(mapState.Markers, func(i, j int) bool {
		return mapState.Markers[i].GetMetadata().GetSortOrder() < mapState.Markers[j].GetMetadata().GetSortOrder()
	})
	sort.SliceStable(mapState.Polygons, func(i, j int) bool {
		return mapState.Polygons[i].GetMetadata().GetSortOrder() < mapState.Polygons[j].GetMetadata().GetSortOrder()
	})
	sort.SliceStable(mapState.Circles, func(i, j int) bool {
		return mapState.Circles[i].GetMetadata().GetSortOrder() < mapState.Circles[j].GetMetadata().GetSortOrder()
	})
}

func markerKey(marker *apiv1.MapMarker) string {
	return fmt.Sprintf("%s|%.6f|%.6f", strings.TrimSpace(marker.GetLabel()), round6(marker.GetPosition().GetLat()), round6(marker.GetPosition().GetLon()))
}

func round6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func findMarker(markers []*apiv1.MapMarker, uid string) *apiv1.MapMarker {
	for _, marker := range markers {
		if marker.GetMetadata().GetUid() == uid {
			return marker
		}
	}
	return nil
}

func findPolygon(polygons []*apiv1.MapPolygon, uid string) *apiv1.MapPolygon {
	for _, polygon := range polygons {
		if polygon.GetMetadata().GetUid() == uid {
			return polygon
		}
	}
	return nil
}

func findCircle(circles []*apiv1.MapCircle, uid string) *apiv1.MapCircle {
	for _, circle := range circles {
		if circle.GetMetadata().GetUid() == uid {
			return circle
		}
	}
	return nil
}

func metadataForElement(mapState *apiv1.Map, elementType apiv1.MapElementType, uid string) *apiv1.MapElementMetadata {
	switch elementType {
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_MARKER:
		if marker := findMarker(mapState.Markers, uid); marker != nil {
			return marker.Metadata
		}
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_POLYGON:
		if polygon := findPolygon(mapState.Polygons, uid); polygon != nil {
			return polygon.Metadata
		}
	case apiv1.MapElementType_MAP_ELEMENT_TYPE_CIRCLE:
		if circle := findCircle(mapState.Circles, uid); circle != nil {
			return circle.Metadata
		}
	}
	return nil
}

func removeMarker(markers []*apiv1.MapMarker, uid string) ([]*apiv1.MapMarker, bool) {
	out := make([]*apiv1.MapMarker, 0, len(markers))
	removed := false
	for _, marker := range markers {
		if marker.GetMetadata().GetUid() == uid {
			removed = true
			continue
		}
		out = append(out, marker)
	}
	return out, removed
}

func removePolygon(polygons []*apiv1.MapPolygon, uid string) ([]*apiv1.MapPolygon, bool) {
	out := make([]*apiv1.MapPolygon, 0, len(polygons))
	removed := false
	for _, polygon := range polygons {
		if polygon.GetMetadata().GetUid() == uid {
			removed = true
			continue
		}
		out = append(out, polygon)
	}
	return out, removed
}

func removeCircle(circles []*apiv1.MapCircle, uid string) ([]*apiv1.MapCircle, bool) {
	out := make([]*apiv1.MapCircle, 0, len(circles))
	removed := false
	for _, circle := range circles {
		if circle.GetMetadata().GetUid() == uid {
			removed = true
			continue
		}
		out = append(out, circle)
	}
	return out, removed
}

func cloneMarker(marker *apiv1.MapMarker) *apiv1.MapMarker {
	if marker == nil {
		return nil
	}
	return &apiv1.MapMarker{
		Label:         marker.GetLabel(),
		Position:      clonePoint(marker.GetPosition()),
		PopupMarkdown: marker.GetPopupMarkdown(),
		Color:         marker.GetColor(),
	}
}

func cloneMarkerWithMetadata(marker *apiv1.MapMarker) *apiv1.MapMarker {
	cloned := cloneMarker(marker)
	cloned.Metadata = cloneMetadata(marker.GetMetadata())
	return cloned
}

func clonePolygon(polygon *apiv1.MapPolygon) *apiv1.MapPolygon {
	if polygon == nil {
		return nil
	}
	return &apiv1.MapPolygon{
		Label:         polygon.GetLabel(),
		Points:        clonePoints(polygon.GetPoints()),
		PopupMarkdown: polygon.GetPopupMarkdown(),
		StrokeColor:   polygon.GetStrokeColor(),
		FillColor:     polygon.GetFillColor(),
	}
}

func cloneCircle(circle *apiv1.MapCircle) *apiv1.MapCircle {
	if circle == nil {
		return nil
	}
	return &apiv1.MapCircle{
		Label:         circle.GetLabel(),
		Center:        clonePoint(circle.GetCenter()),
		RadiusMeters:  circle.GetRadiusMeters(),
		PopupMarkdown: circle.GetPopupMarkdown(),
		StrokeColor:   circle.GetStrokeColor(),
		FillColor:     circle.GetFillColor(),
	}
}

func cloneView(view *apiv1.MapView) *apiv1.MapView {
	if view == nil {
		return nil
	}
	return &apiv1.MapView{Center: clonePoint(view.GetCenter()), Zoom: view.GetZoom()}
}

func clonePoint(point *apiv1.GeoPoint) *apiv1.GeoPoint {
	if point == nil {
		return nil
	}
	return &apiv1.GeoPoint{Lat: point.GetLat(), Lon: point.GetLon()}
}

func clonePoints(points []*apiv1.GeoPoint) []*apiv1.GeoPoint {
	out := make([]*apiv1.GeoPoint, 0, len(points))
	for _, point := range points {
		out = append(out, clonePoint(point))
	}
	return out
}

func cloneMetadata(metadata *apiv1.MapElementMetadata) *apiv1.MapElementMetadata {
	if metadata == nil {
		return &apiv1.MapElementMetadata{}
	}
	return &apiv1.MapElementMetadata{
		Uid:       metadata.GetUid(),
		CreatedAt: metadata.GetCreatedAt(),
		UpdatedAt: metadata.GetUpdatedAt(),
		CreatedBy: metadata.GetCreatedBy(),
		Automated: metadata.GetAutomated(),
		SortOrder: metadata.GetSortOrder(),
	}
}

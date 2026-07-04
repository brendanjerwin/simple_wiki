package mapmutator

import (
	"slices"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	mapsKey        = "maps"
	agentKey       = "agent"
	viewKey        = "view"
	styleKey       = "style"
	updatedAtKey   = "updated_at"
	syncTokenKey   = "sync_token"
	markersKey     = "markers"
	polygonsKey    = "polygons"
	circlesKey     = "circles"
	uidKey         = "uid"
	labelKey       = "label"
	latKey         = "lat"
	lonKey         = "lon"
	zoomKey        = "zoom"
	pointsKey      = "points"
	radiusKey      = "radius_meters"
	popupKey       = "popup_markdown"
	colorKey       = "color"
	strokeKey      = "stroke_color"
	fillKey        = "fill_color"
	tileLayerKey   = "tile_layer_id"
	aspectRatioKey = "aspect_ratio"
	createdAtKey   = "created_at"
	createdByKey   = "created_by"
	automatedKey   = "automated"
	sortOrderKey   = "sort_order"
	tracksKey      = "tracks"
	fileHashKey    = "file_hash"
	formatKey      = "format"
	tagsKey        = "tags"
	filenameKey    = "filename"
)

func mapExists(fm wikipage.FrontMatter, mapName string) bool {
	maps, ok := fm[mapsKey].(map[string]any)
	if !ok {
		return false
	}
	_, ok = maps[mapName]
	return ok
}

func mapNames(fm wikipage.FrontMatter) []string {
	maps, ok := fm[mapsKey].(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(maps))
	for name := range maps {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func decodeMap(fm wikipage.FrontMatter, page, mapName string) *apiv1.Map {
	out := &apiv1.Map{Page: page, Name: mapName}
	userMap := nestedMap(nestedMap(fm, mapsKey), mapName)
	agentMap := nestedMap(nestedMap(nestedMap(fm, agentKey), mapsKey), mapName)
	out.UpdatedAt = decodeTimestamp(agentMap[updatedAtKey])
	out.SyncToken = decodeInt64(agentMap[syncTokenKey])
	out.View = decodeView(userMap[viewKey])
	out.Style = decodeStyle(userMap[styleKey])

	markerMetadataByUID := nestedMap(agentMap, markersKey)
	for _, raw := range decodeSlice(userMap[markersKey]) {
		rawMarker, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := decodeString(rawMarker[uidKey])
		out.Markers = append(out.Markers, &apiv1.MapMarker{
			Metadata:      decodeMetadata(nestedMap(markerMetadataByUID, uid), uid),
			Label:         decodeString(rawMarker[labelKey]),
			Position:      decodePoint(rawMarker),
			PopupMarkdown: decodeString(rawMarker[popupKey]),
			Color:         decodeString(rawMarker[colorKey]),
			Tags:          decodeStringSlice(rawMarker[tagsKey]),
		})
	}

	polygonMetadataByUID := nestedMap(agentMap, polygonsKey)
	for _, raw := range decodeSlice(userMap[polygonsKey]) {
		rawPolygon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := decodeString(rawPolygon[uidKey])
		out.Polygons = append(out.Polygons, &apiv1.MapPolygon{
			Metadata:      decodeMetadata(nestedMap(polygonMetadataByUID, uid), uid),
			Label:         decodeString(rawPolygon[labelKey]),
			Points:        decodePoints(rawPolygon[pointsKey]),
			PopupMarkdown: decodeString(rawPolygon[popupKey]),
			StrokeColor:   decodeString(rawPolygon[strokeKey]),
			FillColor:     decodeString(rawPolygon[fillKey]),
			Tags:          decodeStringSlice(rawPolygon[tagsKey]),
		})
	}

	circleMetadataByUID := nestedMap(agentMap, circlesKey)
	for _, raw := range decodeSlice(userMap[circlesKey]) {
		rawCircle, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := decodeString(rawCircle[uidKey])
		out.Circles = append(out.Circles, &apiv1.MapCircle{
			Metadata:      decodeMetadata(nestedMap(circleMetadataByUID, uid), uid),
			Label:         decodeString(rawCircle[labelKey]),
			Center:        decodePoint(rawCircle),
			RadiusMeters:  decodeFloat64(rawCircle[radiusKey]),
			PopupMarkdown: decodeString(rawCircle[popupKey]),
			StrokeColor:   decodeString(rawCircle[strokeKey]),
			FillColor:     decodeString(rawCircle[fillKey]),
			Tags:          decodeStringSlice(rawCircle[tagsKey]),
		})
	}

	trackMetadataByUID := nestedMap(agentMap, tracksKey)
	for _, raw := range decodeSlice(userMap[tracksKey]) {
		rawTrack, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := decodeString(rawTrack[uidKey])
		out.Tracks = append(out.Tracks, &apiv1.MapTrack{
			Metadata: decodeMetadata(nestedMap(trackMetadataByUID, uid), uid),
			Label:    decodeString(rawTrack[labelKey]),
			FileHash: decodeString(rawTrack[fileHashKey]),
			Format:   decodeString(rawTrack[formatKey]),
			Color:    decodeString(rawTrack[colorKey]),
			Tags:     decodeStringSlice(rawTrack[tagsKey]),
			Filename: decodeString(rawTrack[filenameKey]),
		})
	}

	sortMapElements(out)
	return out
}

func encodeMap(fm wikipage.FrontMatter, mapState *apiv1.Map) {
	maps := ensureNestedMap(fm, mapsKey)
	userMap := ensureNestedMap(maps, mapState.GetName())
	if mapState.GetView() != nil {
		userMap[viewKey] = encodeView(mapState.GetView())
	}
	if mapState.GetStyle() != nil {
		userMap[styleKey] = encodeStyle(mapState.GetStyle())
	}
	userMap[markersKey] = encodeMarkersUserData(mapState.GetMarkers())
	userMap[polygonsKey] = encodePolygonsUserData(mapState.GetPolygons())
	userMap[circlesKey] = encodeCirclesUserData(mapState.GetCircles())
	userMap[tracksKey] = encodeTracksUserData(mapState.GetTracks())

	agent := ensureNestedMap(fm, agentKey)
	agentMaps := ensureNestedMap(agent, mapsKey)
	agentMap := ensureNestedMap(agentMaps, mapState.GetName())
	if mapState.GetUpdatedAt() != nil {
		agentMap[updatedAtKey] = mapState.GetUpdatedAt().AsTime().Format(time.RFC3339Nano)
	}
	agentMap[syncTokenKey] = mapState.GetSyncToken()
	agentMap[markersKey] = encodeMarkerMetadata(mapState.GetMarkers())
	agentMap[polygonsKey] = encodePolygonMetadata(mapState.GetPolygons())
	agentMap[circlesKey] = encodeCircleMetadata(mapState.GetCircles())
	agentMap[tracksKey] = encodeTrackMetadata(mapState.GetTracks())
}

func deleteMapData(fm wikipage.FrontMatter, mapName string) {
	if maps, ok := fm[mapsKey].(map[string]any); ok {
		delete(maps, mapName)
	}
	if agent, ok := fm[agentKey].(map[string]any); ok {
		if agentMaps, ok := agent[mapsKey].(map[string]any); ok {
			delete(agentMaps, mapName)
		}
	}
}

func encodeView(view *apiv1.MapView) map[string]any {
	return map[string]any{
		latKey:  view.GetCenter().GetLat(),
		lonKey:  view.GetCenter().GetLon(),
		zoomKey: view.GetZoom(),
	}
}

func decodeView(raw any) *apiv1.MapView {
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return &apiv1.MapView{Center: decodePoint(rawMap), Zoom: decodeFloat64(rawMap[zoomKey])}
}

func encodeStyle(style *apiv1.MapStyle) map[string]any {
	return map[string]any{
		tileLayerKey:   int32(style.GetTileLayerId()),
		aspectRatioKey: style.GetAspectRatio(),
	}
}

func decodeStyle(raw any) *apiv1.MapStyle {
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return defaultStyle()
	}
	return styleWithTileLayerAndAspectRatio(
		apiv1.TileLayerId(decodeInt32(rawMap[tileLayerKey])),
		decodeString(rawMap[aspectRatioKey]),
	)
}

func encodeMarkersUserData(markers []*apiv1.MapMarker) []any {
	out := make([]any, 0, len(markers))
	for _, marker := range markers {
		if marker == nil {
			continue
		}
		row := map[string]any{
			uidKey:   marker.GetMetadata().GetUid(),
			labelKey: marker.GetLabel(),
			latKey:   marker.GetPosition().GetLat(),
			lonKey:   marker.GetPosition().GetLon(),
		}
		addOptionalCommonFields(row, marker.GetPopupMarkdown(), marker.GetColor(), "", "")
		if len(marker.GetTags()) > 0 {
			row[tagsKey] = encodeStringSlice(marker.GetTags())
		}
		out = append(out, row)
	}
	return out
}

func encodePolygonsUserData(polygons []*apiv1.MapPolygon) []any {
	out := make([]any, 0, len(polygons))
	for _, polygon := range polygons {
		if polygon == nil {
			continue
		}
		row := map[string]any{
			uidKey:    polygon.GetMetadata().GetUid(),
			labelKey:  polygon.GetLabel(),
			pointsKey: encodePoints(polygon.GetPoints()),
		}
		addOptionalCommonFields(row, polygon.GetPopupMarkdown(), "", polygon.GetStrokeColor(), polygon.GetFillColor())
		if len(polygon.GetTags()) > 0 {
			row[tagsKey] = encodeStringSlice(polygon.GetTags())
		}
		out = append(out, row)
	}
	return out
}

func encodeCirclesUserData(circles []*apiv1.MapCircle) []any {
	out := make([]any, 0, len(circles))
	for _, circle := range circles {
		if circle == nil {
			continue
		}
		row := map[string]any{
			uidKey:    circle.GetMetadata().GetUid(),
			labelKey:  circle.GetLabel(),
			latKey:    circle.GetCenter().GetLat(),
			lonKey:    circle.GetCenter().GetLon(),
			radiusKey: circle.GetRadiusMeters(),
		}
		addOptionalCommonFields(row, circle.GetPopupMarkdown(), "", circle.GetStrokeColor(), circle.GetFillColor())
		if len(circle.GetTags()) > 0 {
			row[tagsKey] = encodeStringSlice(circle.GetTags())
		}
		out = append(out, row)
	}
	return out
}

func encodeTracksUserData(tracks []*apiv1.MapTrack) []any {
	out := make([]any, 0, len(tracks))
	for _, track := range tracks {
		if track == nil {
			continue
		}
		row := map[string]any{
			uidKey:      track.GetMetadata().GetUid(),
			labelKey:    track.GetLabel(),
			fileHashKey: track.GetFileHash(),
			formatKey:   track.GetFormat(),
		}
		if track.GetFilename() != "" {
			row[filenameKey] = track.GetFilename()
		}
		if track.GetColor() != "" {
			row[colorKey] = track.GetColor()
		}
		if len(track.GetTags()) > 0 {
			row[tagsKey] = encodeStringSlice(track.GetTags())
		}
		out = append(out, row)
	}
	return out
}

func encodeStringSlice(slice []string) []any {
	out := make([]any, 0, len(slice))
	for _, s := range slice {
		out = append(out, s)
	}
	return out
}

func addOptionalCommonFields(row map[string]any, popupMarkdown, color, strokeColor, fillColor string) {
	if popupMarkdown != "" {
		row[popupKey] = popupMarkdown
	}
	if color != "" {
		row[colorKey] = color
	}
	if strokeColor != "" {
		row[strokeKey] = strokeColor
	}
	if fillColor != "" {
		row[fillKey] = fillColor
	}
}

func encodeMarkerMetadata(markers []*apiv1.MapMarker) map[string]any {
	out := map[string]any{}
	for _, marker := range markers {
		if marker != nil {
			encodeMetadata(out, marker.GetMetadata())
		}
	}
	return out
}

func encodePolygonMetadata(polygons []*apiv1.MapPolygon) map[string]any {
	out := map[string]any{}
	for _, polygon := range polygons {
		if polygon != nil {
			encodeMetadata(out, polygon.GetMetadata())
		}
	}
	return out
}

func encodeCircleMetadata(circles []*apiv1.MapCircle) map[string]any {
	out := map[string]any{}
	for _, circle := range circles {
		if circle != nil {
			encodeMetadata(out, circle.GetMetadata())
		}
	}
	return out
}

func encodeTrackMetadata(tracks []*apiv1.MapTrack) map[string]any {
	out := map[string]any{}
	for _, track := range tracks {
		if track != nil {
			encodeMetadata(out, track.GetMetadata())
		}
	}
	return out
}

func encodeMetadata(out map[string]any, metadata *apiv1.MapElementMetadata) {
	if metadata == nil || metadata.GetUid() == "" {
		return
	}
	row := map[string]any{
		createdByKey: metadata.GetCreatedBy(),
		automatedKey: metadata.GetAutomated(),
		sortOrderKey: metadata.GetSortOrder(),
	}
	if metadata.GetCreatedAt() != nil {
		row[createdAtKey] = metadata.GetCreatedAt().AsTime().Format(time.RFC3339Nano)
	}
	if metadata.GetUpdatedAt() != nil {
		row[updatedAtKey] = metadata.GetUpdatedAt().AsTime().Format(time.RFC3339Nano)
	}
	out[metadata.GetUid()] = row
}

func decodeMetadata(raw map[string]any, uid string) *apiv1.MapElementMetadata {
	return &apiv1.MapElementMetadata{
		Uid:       uid,
		CreatedAt: decodeTimestamp(raw[createdAtKey]),
		UpdatedAt: decodeTimestamp(raw[updatedAtKey]),
		CreatedBy: decodeString(raw[createdByKey]),
		Automated: decodeBool(raw[automatedKey]),
		SortOrder: decodeInt64(raw[sortOrderKey]),
	}
}

func encodePoints(points []*apiv1.GeoPoint) []any {
	out := make([]any, 0, len(points))
	for _, point := range points {
		out = append(out, map[string]any{latKey: point.GetLat(), lonKey: point.GetLon()})
	}
	return out
}

func decodePoints(raw any) []*apiv1.GeoPoint {
	rawPoints := decodeSlice(raw)
	out := make([]*apiv1.GeoPoint, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		rawMap, ok := rawPoint.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, decodePoint(rawMap))
	}
	return out
}

func decodePoint(raw map[string]any) *apiv1.GeoPoint {
	return &apiv1.GeoPoint{Lat: decodeFloat64(raw[latKey]), Lon: decodeFloat64(raw[lonKey])}
}

func ensureNestedMap(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	created := map[string]any{}
	parent[key] = created
	return created
}

func nestedMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return map[string]any{}
	}
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	return map[string]any{}
}

func decodeSlice(raw any) []any {
	if typed, ok := raw.([]any); ok {
		return typed
	}
	return nil
}

func decodeString(raw any) string {
	if typed, ok := raw.(string); ok {
		return typed
	}
	return ""
}

func decodeStringSlice(raw any) []string {
	slice := decodeSlice(raw)
	if slice == nil {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func decodeBool(raw any) bool {
	if typed, ok := raw.(bool); ok {
		return typed
	}
	return false
}

func decodeFloat64(raw any) float64 {
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func decodeInt32(raw any) int32 {
	return int32(decodeInt64(raw))
}

func decodeInt64(raw any) int64 {
	switch typed := raw.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func decodeTimestamp(raw any) *timestamppb.Timestamp {
	rawString, ok := raw.(string)
	if !ok || rawString == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, rawString)
	if err != nil {
		return nil
	}
	return timestamppb.New(parsed)
}

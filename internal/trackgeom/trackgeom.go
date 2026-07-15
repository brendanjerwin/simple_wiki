package trackgeom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/tkrajina/gpxgo/gpx"
)

// Point represents a single GPS coordinate with optional elevation and timestamp.
type Point struct {
	Lat       float64
	Lon       float64
	Elevation *float64   // in meters, optional
	Time      *time.Time // optional
}

// Segment represents a continuous list of points.
type Segment []Point

// Track represents a named GPS track with one or more segments.
type Track struct {
	Name     string
	Segments []Segment
}

// TrackFormat represents the file format of the track data.
type TrackFormat string

const (
	TrackFormatGPX     TrackFormat = "GPX"
	TrackFormatGeoJSON TrackFormat = "GeoJSON"

	GPX     TrackFormat = "GPX"
	GeoJSON TrackFormat = "GeoJSON"
)

// Numeric constants used during parsing, coordinate normalization, and simplification.
const (
	maxTrackPoints                    = 50000 // hard safety cap on total parsed points
	longitudeWrapThreshold            = 180.0 // degrees; beyond this a longitude delta wraps
	longitudeWrapOffset               = 360.0 // degrees; added/subtracted to normalize wrapped longitudes
	degreesToRadians                  = math.Pi / 180.0
	minSegmentLengthForSimplification = 3     // Douglas–Peucker needs at least three points
	simplificationSearchIterations    = 40    // geometric binary-search iterations for point ceiling
	decimalGroupingSize               = 3     // digits per comma-separated thousands group
)

// Parse routes the parsing to the appropriate format parser based on the given TrackFormat,
// checks point safety limit, and returns the list of parsed Segments.
func Parse(format TrackFormat, reader io.Reader) ([]Segment, error) {
	var track *Track
	var err error

	normalized := TrackFormat(strings.ToUpper(strings.TrimSpace(string(format))))
	switch normalized {
	case "GPX":
		track, err = ParseGPX(reader)
	case "GEOJSON", "JSON":
		track, err = ParseGeoJSON(reader)
	default:
		return nil, fmt.Errorf("unsupported track format: %s", format)
	}

	if err != nil {
		return nil, err
	}

	totalPoints := totalSegmentPoints(track.Segments)

	// Point Safety Limit (reject if total points exceeds the safety cap).
	if totalPoints > maxTrackPoints {
		return nil, fmt.Errorf("track exceeds maximum safety limit of %s points (got %s)", formatIntWithCommas(maxTrackPoints), formatIntWithCommas(totalPoints))
	}

	return track.Segments, nil
}

// totalSegmentPoints returns the total number of points across all segments.
func totalSegmentPoints(segments []Segment) int {
	total := 0
	for _, seg := range segments {
		total += len(seg)
	}
	return total
}

// formatIntWithCommas returns n formatted with comma thousands separators.
func formatIntWithCommas(n int) string {
	str := strconv.Itoa(n)
	if len(str) <= decimalGroupingSize {
		return str
	}
	var out strings.Builder
	for i, digit := range str {
		if i > 0 && (len(str)-i)%decimalGroupingSize == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}

// ParseGPX parses GPX XML data from r.
func ParseGPX(r io.Reader) (*Track, error) {
	gpxData, err := gpx.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPX: %w", err)
	}

	track := &Track{}
	if len(gpxData.Tracks) > 0 {
		extractTracks(track, gpxData.Tracks)
	} else if len(gpxData.Routes) > 0 {
		extractRoutes(track, gpxData.Routes)
	}

	if len(track.Segments) == 0 {
		return nil, errors.New("no tracks or segments found in GPX")
	}

	return track, nil
}

// extractTracks converts gpxgo track segments into the shared Segment format.
func extractTracks(track *Track, gpxTracks []gpx.GPXTrack) {
	for _, gpxTrack := range gpxTracks {
		track.Name = firstNonEmpty(track.Name, gpxTrack.Name)
		for _, gpxSeg := range gpxTrack.Segments {
			if seg := segmentFromGPXPoints(gpxSeg.Points); len(seg) > 0 {
				track.Segments = append(track.Segments, seg)
			}
		}
	}
}

// extractRoutes converts gpxgo route waypoints into a single Segment.
func extractRoutes(track *Track, gpxRoutes []gpx.GPXRoute) {
	for _, gpxRoute := range gpxRoutes {
		track.Name = firstNonEmpty(track.Name, gpxRoute.Name)
		if seg := segmentFromGPXPoints(gpxRoute.Points); len(seg) > 0 {
			track.Segments = append(track.Segments, seg)
		}
	}
}

// firstNonEmpty returns a if it is non-empty, otherwise b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// segmentFromGPXPoints builds a Segment from a slice of gpxgo points.
func segmentFromGPXPoints(points []gpx.GPXPoint) Segment {
	seg := make(Segment, 0, len(points))
	for _, pt := range points {
		seg = append(seg, pointFromGPXPoint(pt))
	}
	return seg
}

// pointFromGPXPoint converts a single gpxgo point into the local Point type.
func pointFromGPXPoint(pt gpx.GPXPoint) Point {
	var elevation *float64
	if pt.Elevation.NotNull() {
		val := pt.Elevation.Value()
		elevation = &val
	}
	var t *time.Time
	if !pt.Timestamp.IsZero() {
		val := pt.Timestamp
		t = &val
	}
	return Point{
		Lat:       pt.Latitude,
		Lon:       pt.Longitude,
		Elevation: elevation,
		Time:      t,
	}
}

// ParseGeoJSON parses GeoJSON data from r.
func ParseGeoJSON(r io.Reader) (*Track, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read GeoJSON: %w", err)
	}

	// Unmarshal as FeatureCollection
	if fc, err := geojson.UnmarshalFeatureCollection(data); err == nil {
		return trackFromFeatureCollection(fc)
	}

	// Unmarshal as Feature
	if f, err := geojson.UnmarshalFeature(data); err == nil {
		return trackFromFeature(f)
	}

	// Unmarshal as Geometry
	if geom, err := geojson.UnmarshalGeometry(data); err == nil {
		return trackFromGeometry(geom.Geometry())
	}

	// Dynamic json fallback just in case GeoJSON is wrapped or has different keys
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		if val, ok := raw["type"].(string); ok {
			return nil, fmt.Errorf("unsupported GeoJSON type: %s", val)
		}
	}

	return nil, errors.New("invalid GeoJSON format")
}

func trackFromFeatureCollection(fc *geojson.FeatureCollection) (*Track, error) {
	track := &Track{}
	for _, f := range fc.Features {
		t, err := trackFromFeature(f)
		if err == nil {
			track.Segments = append(track.Segments, t.Segments...)
			if track.Name == "" && t.Name != "" {
				track.Name = t.Name
			}
		}
	}
	if len(track.Segments) == 0 {
		return nil, errors.New("no tracks found in feature collection")
	}
	return track, nil
}

func trackFromFeature(f *geojson.Feature) (*Track, error) {
	name, _ := f.Properties["name"].(string)
	track, err := trackFromGeometry(f.Geometry)
	if err != nil {
		return nil, err
	}
	track.Name = name

	applyCoordTimes(track.Segments, f.Properties["coordTimes"])
	applyCoordTimes(track.Segments, f.Properties["times"])

	return track, nil
}

// applyCoordTimes parses RFC3339 timestamps from a property value and writes
// them into the matching segment points in order.
func applyCoordTimes(segments []Segment, raw any) {
	coordTimes, ok := raw.([]any)
	if !ok {
		return
	}
	idx := 0
	for sIdx := range segments {
		for pIdx := range segments[sIdx] {
			if idx >= len(coordTimes) {
				return
			}
			if tStr, ok := coordTimes[idx].(string); ok {
				if t, err := time.Parse(time.RFC3339, tStr); err == nil {
					segments[sIdx][pIdx].Time = &t
				}
			}
			idx++
		}
	}
}

func trackFromGeometry(geom orb.Geometry) (*Track, error) {
	if geom == nil {
		return nil, errors.New("nil geometry")
	}

	track := &Track{}
	switch g := geom.(type) {
	case orb.LineString:
		var seg Segment
		for _, pt := range g {
			seg = append(seg, Point{
				Lat: pt[1],
				Lon: pt[0],
			})
		}
		track.Segments = append(track.Segments, seg)
	case orb.MultiLineString:
		for _, ls := range g {
			var seg Segment
			for _, pt := range ls {
				seg = append(seg, Point{
					Lat: pt[1],
					Lon: pt[0],
				})
			}
			track.Segments = append(track.Segments, seg)
		}
	default:
		return nil, fmt.Errorf("unsupported geometry type: %T", geom)
	}
	return track, nil
}

// perpendicularDistance calculates the Cartesian distance (in degrees) from p 
// to the line segment start-end, accounting for longitude convergence.
func perpendicularDistance(p, start, end Point) float64 {
	avgLat := (start.Lat + end.Lat) / 2.0
	rad := avgLat * degreesToRadians
	cosLat := math.Cos(rad)

	// Normalize longitude differences relative to start.Lon to handle antimeridian crossing
	dx := normalizeLongitudeDelta(end.Lon - start.Lon) * cosLat
	pDx := normalizeLongitudeDelta(p.Lon - start.Lon) * cosLat

	dy := end.Lat - start.Lat

	if dx == 0 && dy == 0 {
		return math.Hypot(pDx, p.Lat-start.Lat)
	}

	t := (pDx*dx + (p.Lat-start.Lat)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	nearestLonDiff := t * dx
	nearestLat := start.Lat + t*dy

	return math.Hypot(pDx-nearestLonDiff, p.Lat-nearestLat)
}

// normalizeLongitudeDelta wraps a longitude difference to [-180, 180] degrees.
func normalizeLongitudeDelta(delta float64) float64 {
	if delta > longitudeWrapThreshold {
		return delta - longitudeWrapOffset
	}
	if delta < -longitudeWrapThreshold {
		return delta + longitudeWrapOffset
	}
	return delta
}

func simplifySegment(segment Segment, epsilon float64) Segment {
	if len(segment) < minSegmentLengthForSimplification {
		return segment
	}

	maxDist := 0.0
	index := -1
	end := len(segment) - 1

	for i := 1; i < end; i++ {
		d := perpendicularDistance(segment[i], segment[0], segment[end])
		if d > maxDist {
			maxDist = d
			index = i
		}
	}

	if maxDist > epsilon {
		results1 := simplifySegment(segment[:index+1], epsilon)
		results2 := simplifySegment(segment[index:], epsilon)

		// Combine results without duplicating the midpoint, allocating a new slice
		out := make(Segment, 0, len(results1)-1+len(results2))
		out = append(out, results1[:len(results1)-1]...)
		out = append(out, results2...)
		return out
	}

	return Segment{segment[0], segment[end]}
}

// Simplify simplifies the track's segments so that the total number of points
// across all segments is <= ceiling. It uses a geometric (log-scale) binary
// search over epsilon, which is essential because perpendicular distances
// for GPS tracks span many orders of magnitude (sub-meter to hundreds of km)
// while a linear search across [0, 360] degrees would skip the narrow window
// where the point count transitions between "all" and "two".
func (t *Track) Simplify(ceiling int) *Track {
	totalPoints := 0
	for _, seg := range t.Segments {
		totalPoints += len(seg)
	}

	// No simplification needed if we're already under the ceiling.
	if totalPoints <= ceiling {
		return t
	}

	// Geometric binary search over epsilon in degrees. GPS perpendicular
	// distances range from ~1e-8 (sub-meter) to ~180 (antimeridian span), so
	// search in log space: 1e-12 to 1e3 covers the full realistic range with
	// margin. 60 iterations of log2(1e15) ≈ 50 bits of precision.
	const (
		logLow  = -12.0 // ln(1e-12) ≈ -27.6, use log2 for clarity
		logHigh = 3.0   // ln(1e3)  ≈ 6.9
	)
	low := logLow
	high := logHigh
	bestEpsilon := math.Pow(2, high)

	for range simplificationSearchIterations {
		mid := (low + high) / 2.0
		eps := math.Pow(2, mid)

		count := 0
		for _, seg := range t.Segments {
			simplified := simplifySegment(seg, eps)
			count += len(simplified)
		}

		if count <= ceiling {
			// Threshold is valid; record it and search for a smaller epsilon
			// (more detail) by lowering the upper bound.
			bestEpsilon = eps
			high = mid
		} else {
			// Too many points; increase epsilon.
			low = mid
		}
	}

	simplifiedTrack := &Track{
		Name: t.Name,
	}
	for _, seg := range t.Segments {
		simplifiedTrack.Segments = append(simplifiedTrack.Segments, simplifySegment(seg, bestEpsilon))
	}

	return simplifiedTrack
}

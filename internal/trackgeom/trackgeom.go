package trackgeom

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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

	// Calculate total points
	totalPoints := 0
	for _, seg := range track.Segments {
		totalPoints += len(seg)
	}

	// Point Safety Limit (reject if total points > 50,000)
	if totalPoints > 50000 {
		return nil, fmt.Errorf("track exceeds maximum safety limit of 50,000 points (got %d)", totalPoints)
	}

	return track.Segments, nil
}

// ParseGPX parses GPX XML data from r.
func ParseGPX(r io.Reader) (*Track, error) {
	gpxData, err := gpx.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPX: %w", err)
	}

	track := &Track{}
	if len(gpxData.Tracks) > 0 {
		for _, gpxTrack := range gpxData.Tracks {
			if track.Name == "" && gpxTrack.Name != "" {
				track.Name = gpxTrack.Name
			}

			for _, gpxSeg := range gpxTrack.Segments {
				var seg Segment
				for _, pt := range gpxSeg.Points {
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
					seg = append(seg, Point{
						Lat:       pt.Latitude,
						Lon:       pt.Longitude,
						Elevation: elevation,
						Time:      t,
					})
				}
				if len(seg) > 0 {
					track.Segments = append(track.Segments, seg)
				}
			}
		}
	} else if len(gpxData.Routes) > 0 {
		for _, gpxRoute := range gpxData.Routes {
			if track.Name == "" && gpxRoute.Name != "" {
				track.Name = gpxRoute.Name
			}
			var seg Segment
			for _, pt := range gpxRoute.Points {
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
				seg = append(seg, Point{
					Lat:       pt.Latitude,
					Lon:       pt.Longitude,
					Elevation: elevation,
					Time:      t,
				})
			}
			if len(seg) > 0 {
				track.Segments = append(track.Segments, seg)
			}
		}
	}

	if len(track.Segments) == 0 {
		return nil, fmt.Errorf("no tracks or segments found in GPX")
	}

	return track, nil
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

	return nil, fmt.Errorf("invalid GeoJSON format")
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
		return nil, fmt.Errorf("no tracks found in feature collection")
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

	// Retrieve times if present in properties
	if coordTimes, ok := f.Properties["coordTimes"].([]any); ok {
		idx := 0
		for sIdx := range track.Segments {
			for pIdx := range track.Segments[sIdx] {
				if idx < len(coordTimes) {
					if tStr, ok := coordTimes[idx].(string); ok {
						if t, err := time.Parse(time.RFC3339, tStr); err == nil {
							track.Segments[sIdx][pIdx].Time = &t
						}
					}
				}
				idx++
			}
		}
	} else if coordTimes, ok := f.Properties["times"].([]any); ok {
		idx := 0
		for sIdx := range track.Segments {
			for pIdx := range track.Segments[sIdx] {
				if idx < len(coordTimes) {
					if tStr, ok := coordTimes[idx].(string); ok {
						if t, err := time.Parse(time.RFC3339, tStr); err == nil {
							track.Segments[sIdx][pIdx].Time = &t
						}
					}
				}
				idx++
			}
		}
	}

	return track, nil
}

func trackFromGeometry(geom orb.Geometry) (*Track, error) {
	if geom == nil {
		return nil, fmt.Errorf("nil geometry")
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
	rad := avgLat * math.Pi / 180.0
	cosLat := math.Cos(rad)

	// Normalize longitude differences relative to start.Lon to handle antimeridian crossing
	dx := end.Lon - start.Lon
	if dx > 180 {
		dx -= 360
	} else if dx < -180 {
		dx += 360
	}
	dx = dx * cosLat

	pDx := p.Lon - start.Lon
	if pDx > 180 {
		pDx -= 360
	} else if pDx < -180 {
		pDx += 360
	}
	pDx = pDx * cosLat

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

func simplifySegment(segment Segment, epsilon float64) Segment {
	if len(segment) < 3 {
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

	for range 40 {
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

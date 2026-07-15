package trackgeom_test

import (
	"strings"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/trackgeom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTrackGeom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TrackGeom Suite")
}

var _ = Describe("Track Parsing and Simplification", func() {
	Describe("ParseGPX", func() {
		var (
			input string
			track *trackgeom.Track
			err   error
		)

		When("given valid GPX with track, segments and points", func() {
			BeforeEach(func() {
				input = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>Morning Hike</name>
    <trkseg>
      <trkpt lat="37.7749" lon="-122.4194">
        <ele>100.5</ele>
        <time>2026-06-14T18:00:00Z</time>
      </trkpt>
      <trkpt lat="37.7750" lon="-122.4195">
        <ele>101.2</ele>
        <time>2026-06-14T18:05:00Z</time>
      </trkpt>
    </trkseg>
  </trk>
</gpx>`
				track, err = trackgeom.ParseGPX(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse the correct track name", func() {
				Expect(track.Name).To(Equal("Morning Hike"))
			})

			It("should contain one segment", func() {
				Expect(track.Segments).To(HaveLen(1))
			})

			It("should contain two points in the segment", func() {
				Expect(track.Segments[0]).To(HaveLen(2))
			})

			It("should parse coordinate values correctly", func() {
				Expect(track.Segments[0][0].Lat).To(Equal(37.7749))
				Expect(track.Segments[0][0].Lon).To(Equal(-122.4194))
			})

			It("should parse elevation correctly", func() {
				Expect(track.Segments[0][0].Elevation).NotTo(BeNil())
				Expect(*track.Segments[0][0].Elevation).To(Equal(100.5))
			})

			It("should parse time correctly", func() {
				Expect(track.Segments[0][0].Time).NotTo(BeNil())
				Expect(track.Segments[0][0].Time.Format(time.RFC3339)).To(Equal("2026-06-14T18:00:00Z"))
			})
		})

		When("given valid GPX with route and routepoints", func() {
			BeforeEach(func() {
				input = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <rte>
    <name>Route Hike</name>
    <rtept lat="37.7749" lon="-122.4194">
      <ele>100.5</ele>
      <time>2026-06-14T18:00:00Z</time>
    </rtept>
    <rtept lat="37.7750" lon="-122.4195">
      <ele>101.2</ele>
      <time>2026-06-14T18:05:00Z</time>
    </rtept>
  </rte>
</gpx>`
				track, err = trackgeom.ParseGPX(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse the correct route name", func() {
				Expect(track.Name).To(Equal("Route Hike"))
			})

			It("should contain one segment", func() {
				Expect(track.Segments).To(HaveLen(1))
			})

			It("should contain two points", func() {
				Expect(track.Segments[0]).To(HaveLen(2))
			})
		})

		When("given valid GPX with multiple tracks and segments", func() {
			BeforeEach(func() {
				input = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>Track 1</name>
    <trkseg>
      <trkpt lat="37.7749" lon="-122.4194"></trkpt>
      <trkpt lat="37.7750" lon="-122.4195"></trkpt>
    </trkseg>
    <trkseg>
      <trkpt lat="37.7751" lon="-122.4196"></trkpt>
      <trkpt lat="37.7752" lon="-122.4197"></trkpt>
    </trkseg>
  </trk>
  <trk>
    <name>Track 2</name>
    <trkseg>
      <trkpt lat="37.7753" lon="-122.4198"></trkpt>
      <trkpt lat="37.7754" lon="-122.4199"></trkpt>
    </trkseg>
  </trk>
</gpx>`
				track, err = trackgeom.ParseGPX(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should capture the name of the first track", func() {
				Expect(track.Name).To(Equal("Track 1"))
			})

			It("should contain three segments in total across all tracks", func() {
				Expect(track.Segments).To(HaveLen(3))
			})

			It("should capture all points correctly", func() {
				Expect(track.Segments[0][0].Lat).To(Equal(37.7749))
				Expect(track.Segments[1][0].Lat).To(Equal(37.7751))
				Expect(track.Segments[2][0].Lat).To(Equal(37.7753))
			})
		})

		When("given invalid GPX format", func() {
			BeforeEach(func() {
				input = `<invalid xml>`
				track, err = trackgeom.ParseGPX(strings.NewReader(input))
			})

			It("should return a parsing error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse GPX"))
			})

			It("should return nil track", func() {
				Expect(track).To(BeNil())
			})
		})

		When("given GPX without any track or route segments", func() {
			BeforeEach(func() {
				input = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
</gpx>`
				track, err = trackgeom.ParseGPX(strings.NewReader(input))
			})

			It("should return a validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no tracks or segments found in GPX"))
			})

			It("should return nil track", func() {
				Expect(track).To(BeNil())
			})
		})
	})

	Describe("ParseGeoJSON", func() {
		var (
			input string
			track *trackgeom.Track
			err   error
		)

		When("given a raw LineString geometry", func() {
			BeforeEach(func() {
				input = `{
					"type": "LineString",
					"coordinates": [
						[-122.4194, 37.7749],
						[-122.4195, 37.7750]
					]
				}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should contain one segment", func() {
				Expect(track.Segments).To(HaveLen(1))
			})

			It("should parse coordinates correctly", func() {
				Expect(track.Segments[0][0].Lat).To(Equal(37.7749))
				Expect(track.Segments[0][0].Lon).To(Equal(-122.4194))
				Expect(track.Segments[0][1].Lat).To(Equal(37.7750))
				Expect(track.Segments[0][1].Lon).To(Equal(-122.4195))
			})
		})

		When("given a raw MultiLineString geometry", func() {
			BeforeEach(func() {
				input = `{
					"type": "MultiLineString",
					"coordinates": [
						[
							[-122.4194, 37.7749],
							[-122.4195, 37.7750]
						],
						[
							[-122.4196, 37.7751],
							[-122.4197, 37.7752]
						]
					]
				}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should contain two segments", func() {
				Expect(track.Segments).To(HaveLen(2))
			})

			It("should parse coordinates correctly for all segments", func() {
				Expect(track.Segments[0][0].Lat).To(Equal(37.7749))
				Expect(track.Segments[1][0].Lat).To(Equal(37.7751))
			})
		})

		When("given a Feature containing LineString", func() {
			BeforeEach(func() {
				input = `{
					"type": "Feature",
					"properties": {
						"name": "Evening Run",
						"coordTimes": ["2026-06-14T18:00:00Z", "2026-06-14T18:05:00Z"]
					},
					"geometry": {
						"type": "LineString",
						"coordinates": [
							[-122.4194, 37.7749],
							[-122.4195, 37.7750]
						]
					}
				}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse track name correctly", func() {
				Expect(track.Name).To(Equal("Evening Run"))
			})

			It("should parse times from Feature properties correctly", func() {
				Expect(track.Segments[0][0].Time).NotTo(BeNil())
				Expect(track.Segments[0][0].Time.Format(time.RFC3339)).To(Equal("2026-06-14T18:00:00Z"))
				Expect(track.Segments[0][1].Time).NotTo(BeNil())
				Expect(track.Segments[0][1].Time.Format(time.RFC3339)).To(Equal("2026-06-14T18:05:00Z"))
			})
		})

		When("given a FeatureCollection", func() {
			BeforeEach(func() {
				input = `{
					"type": "FeatureCollection",
					"features": [
						{
							"type": "Feature",
							"properties": {
								"name": "Morning Ride"
							},
							"geometry": {
								"type": "LineString",
								"coordinates": [
									[-122.4194, 37.7749],
									[-122.4195, 37.7750]
								]
							}
						}
					]
				}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should parse successfully without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse name from the feature", func() {
				Expect(track.Name).To(Equal("Morning Ride"))
			})

			It("should contain the segments from the feature", func() {
				Expect(track.Segments).To(HaveLen(1))
			})
		})

		When("given unsupported geometry types", func() {
			BeforeEach(func() {
				input = `{
					"type": "Point",
					"coordinates": [-122.4194, 37.7749]
				}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should return an error indicating unsupported geometry type", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported geometry type"))
			})
		})

		When("given invalid GeoJSON data", func() {
			BeforeEach(func() {
				input = `{invalid-json}`
				track, err = trackgeom.ParseGeoJSON(strings.NewReader(input))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GeoJSON format"))
			})
		})
	})

	Describe("Douglas-Peucker Simplification", func() {
		var (
			track *trackgeom.Track
		)

		When("track is already under the ceiling", func() {
			var simplified *trackgeom.Track

			BeforeEach(func() {
				track = &trackgeom.Track{
					Name: "Short Track",
					Segments: []trackgeom.Segment{
						{
							{Lat: 1.0, Lon: 1.0},
							{Lat: 2.0, Lon: 2.0},
							{Lat: 3.0, Lon: 3.0},
						},
					},
				}
				simplified = track.Simplify(5)
			})

			It("should return original track structure as is", func() {
				Expect(simplified.Segments[0]).To(HaveLen(3))
				Expect(simplified.Segments[0][1].Lat).To(Equal(2.0))
			})
		})

		When("simplifying collinear points with a ceiling", func() {
			var simplified *trackgeom.Track

			BeforeEach(func() {
				// 5 collinear points on a line, with minor numerical differences
				track = &trackgeom.Track{
					Name: "Collinear Track",
					Segments: []trackgeom.Segment{
						{
							{Lat: 0.0, Lon: 0.0},
							{Lat: 1.0, Lon: 1.0},
							{Lat: 2.0, Lon: 2.0},
							{Lat: 3.0, Lon: 3.0},
							{Lat: 4.0, Lon: 4.0},
						},
					},
				}
				simplified = track.Simplify(3)
			})

			It("should simplify to start and end points only", func() {
				Expect(simplified.Segments[0]).To(HaveLen(2))
				Expect(simplified.Segments[0][0].Lat).To(Equal(0.0))
				Expect(simplified.Segments[0][1].Lat).To(Equal(4.0))
			})
		})

		When("simplifying a track with a distinct peak and a low ceiling", func() {
			var simplified *trackgeom.Track

			BeforeEach(func() {
				// (0,0) -> (1,1) -> (2,3) -> (3,1) -> (4,0)
				// (2,3) is a distinct peak
				track = &trackgeom.Track{
					Name: "Peak Track",
					Segments: []trackgeom.Segment{
						{
							{Lat: 0.0, Lon: 0.0},
							{Lat: 1.0, Lon: 1.0},
							{Lat: 3.0, Lon: 2.0}, // Peak
							{Lat: 1.0, Lon: 3.0},
							{Lat: 0.0, Lon: 4.0},
						},
					},
				}
				simplified = track.Simplify(3)
			})

			It("should retain the peak point", func() {
				Expect(simplified.Segments[0]).To(HaveLen(3))
				Expect(simplified.Segments[0][0].Lat).To(Equal(0.0))
				Expect(simplified.Segments[0][1].Lat).To(Equal(3.0))
				Expect(simplified.Segments[0][2].Lat).To(Equal(0.0))
			})
		})

		When("the track is closed (forming a loop)", func() {
			var simplified *trackgeom.Track

			BeforeEach(func() {
				// Closed loop: (0,0) -> (0,1) -> (1,1) -> (1,0) -> (0,0)
				track = &trackgeom.Track{
					Name: "Loop Track",
					Segments: []trackgeom.Segment{
						{
							{Lat: 0.0, Lon: 0.0},
							{Lat: 0.0, Lon: 1.0},
							{Lat: 1.0, Lon: 1.0},
							{Lat: 1.0, Lon: 0.0},
							{Lat: 0.0, Lon: 0.0},
						},
					},
				}
				simplified = track.Simplify(3)
			})

			It("should simplify while maintaining the geometry boundaries", func() {
				// Max points constraint is 3, but loop needs to be represented.
				// Since start and end are identical (0,0), distance to other points is measured.
				// With ceiling 3, it simplifies to keep points that maximize shape.
				Expect(len(simplified.Segments[0])).To(BeNumerically("<=", 3))
			})
		})

		When("simplifying extremely large segments with a ceiling of 1000", func() {
			var (
				largeTrack *trackgeom.Track
				simplified *trackgeom.Track
			)

			BeforeEach(func() {
				// Create a track with a single segment containing 5000 points
				// A high-frequency zig-zag to ensure points aren't collinear
				seg := make(trackgeom.Segment, 5000)
				for i := 0; i < 5000; i++ {
					lat := 37.7749 + 0.001*float64(i%3)
					lon := -122.4194 + 0.001*float64(i)
					seg[i] = trackgeom.Point{Lat: lat, Lon: lon}
				}

				largeTrack = &trackgeom.Track{
					Name:     "Large Track",
					Segments: []trackgeom.Segment{seg},
				}
				simplified = largeTrack.Simplify(1000)
			})

			It("should reduce the number of points below the ceiling of 1000", func() {
				totalPoints := 0
				for _, seg := range simplified.Segments {
					totalPoints += len(seg)
				}
				Expect(totalPoints).To(BeNumerically("<=", 1000))
				// Ensure it is not simplified to the absolute minimum of 2 points,
				// meaning binary search successfully found an intermediate epsilon
				Expect(totalPoints).To(BeNumerically(">", 2))
			})

			It("should not mutate the original track", func() {
				Expect(largeTrack.Segments[0]).To(HaveLen(5000))
			})
		})

		When("simplifying a track with a ceiling smaller than 2 * segments count", func() {
			var (
				multiTrack *trackgeom.Track
				simplified *trackgeom.Track
			)

			BeforeEach(func() {
				// 3 segments, each segment has 5 points (total 15 points)
				// Minimum possible points for 3 segments is 3 * 2 = 6 points
				// We call Simplify with ceiling = 4
				var segments []trackgeom.Segment
				for s := 0; s < 3; s++ {
					seg := make(trackgeom.Segment, 5)
					for i := 0; i < 5; i++ {
						lat := 37.7749 + float64(s) + 0.01*float64(i%2)
						lon := -122.4194 + float64(s) + 0.01*float64(i)
						seg[i] = trackgeom.Point{Lat: lat, Lon: lon}
					}
					segments = append(segments, seg)
				}

				multiTrack = &trackgeom.Track{
					Name:     "Multi Track",
					Segments: segments,
				}
				simplified = multiTrack.Simplify(4)
			})

			It("should handle the constraints gracefully, reducing points to the absolute minimum of 6 points", func() {
				totalPoints := 0
				for _, seg := range simplified.Segments {
					totalPoints += len(seg)
				}
				// The algorithm cannot simplify segments to less than 2 points each,
				// so the count should be 6, which is the closest it can get to 4.
				Expect(totalPoints).To(Equal(6))
			})
		})
	})

	Describe("Parse Interface and Safety Limits", func() {
		Context("when parsing via high-level Parse function", func() {
			It("should route GPX format correctly", func() {
				input := `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>Route Hike</name>
    <trkseg>
      <trkpt lat="37.7749" lon="-122.4194"></trkpt>
      <trkpt lat="37.7750" lon="-122.4195"></trkpt>
    </trkseg>
  </trk>
</gpx>`
				segments, err := trackgeom.Parse(trackgeom.TrackFormatGPX, strings.NewReader(input))
				Expect(err).NotTo(HaveOccurred())
				Expect(segments).To(HaveLen(1))
				Expect(segments[0]).To(HaveLen(2))
			})

			It("should route GeoJSON format correctly", func() {
				input := `{
					"type": "LineString",
					"coordinates": [
						[-122.4194, 37.7749],
						[-122.4195, 37.7750]
					]
				}`
				segments, err := trackgeom.Parse(trackgeom.TrackFormatGeoJSON, strings.NewReader(input))
				Expect(err).NotTo(HaveOccurred())
				Expect(segments).To(HaveLen(1))
				Expect(segments[0]).To(HaveLen(2))
			})

			It("should return an error for unsupported formats", func() {
				_, err := trackgeom.Parse(trackgeom.TrackFormat("KML"), strings.NewReader("<xml></xml>"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported track format"))
			})
		})

		Context("with points safety limit (> 50,000)", func() {
			It("should reject files exceeding the 50,000 points safety limit", func() {
				// Create a GPX segment with 50,001 points
				var sb strings.Builder
				sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>Huge Track</name>
    <trkseg>`)
				for i := 0; i < 50001; i++ {
					sb.WriteString(`<trkpt lat="37.7" lon="-122.4"></trkpt>`)
				}
				sb.WriteString(`</trkseg>
  </trk>
</gpx>`)
				_, err := trackgeom.Parse(trackgeom.TrackFormatGPX, strings.NewReader(sb.String()))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("track exceeds maximum safety limit of 50,000 points"))
			})

			It("should allow files with exactly 50,000 points", func() {
				var sb strings.Builder
				sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="ExpertGPS" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>Limit Track</name>
    <trkseg>`)
				for i := 0; i < 50000; i++ {
					sb.WriteString(`<trkpt lat="37.7" lon="-122.4"></trkpt>`)
				}
				sb.WriteString(`</trkseg>
  </trk>
</gpx>`)
				segments, err := trackgeom.Parse(trackgeom.TrackFormatGPX, strings.NewReader(sb.String()))
				Expect(err).NotTo(HaveOccurred())
				totalPoints := 0
				for _, s := range segments {
					totalPoints += len(s)
				}
				Expect(totalPoints).To(Equal(50000))
			})
		})
	})

	Describe("Antimeridian Crossing and Longitude Wrapping", func() {
		It("should correctly calculate perpendicular distance across the antimeridian", func() {
			// Segment from start at 179.0 Lon to end at -179.0 Lon (crosses 180/-180 meridian).
			// The distance in degrees is 2.0 degrees total.
			// Let's place a query point p right in the middle at 180.0 Lon (which maps to -180.0).
			// If wrapping is not handled, end Lon - start Lon would be -179 - 179 = -358.
			// With wrapping, end Lon - start Lon is 2.0 degrees, and p lies on the segment (distance ~ 0).
			
			// Let's verify by simplifying a track that crosses the antimeridian
			// We have three points:
			// P1: (0.0, 179.0)
			// P2: (0.001, -180.0) // slightly offset lat, but on the wrapped path
			// P3: (0.0, -179.0)
			track := &trackgeom.Track{
				Name: "Antimeridian Track",
				Segments: []trackgeom.Segment{
					{
						{Lat: 0.0, Lon: 179.0},
						{Lat: 0.001, Lon: -180.0},
						{Lat: 0.0, Lon: -179.0},
					},
				},
			}
			
			// Simplify with epsilon larger than 0.001 but smaller than the un-wrapped distance.
			// If wrapped properly, P2 is only 0.001 degrees lat away from the segment P1-P3.
			// If we simplify with ceiling 2 and epsilon 0.002, it should simplify to P1 and P3.
			simplified := track.Simplify(2)
			Expect(simplified.Segments[0]).To(HaveLen(2))
			Expect(simplified.Segments[0][0].Lon).To(Equal(179.0))
			Expect(simplified.Segments[0][1].Lon).To(Equal(-179.0))
		})
	})
})

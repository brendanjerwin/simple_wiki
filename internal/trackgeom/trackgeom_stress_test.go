package trackgeom_test

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/trackgeom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Douglas-Peucker Stress and Oracle Tests", func() {
	Context("with extremely large GPS track inputs", func() {
		It("should reduce 50,000 points below a ceiling of 1000 within reasonable time", func() {
			// Generate 50,000 points along a winding route with real directional
			// changes so Douglas-Peucker has structure to preserve. A near-straight
			// line with only noise collapses to 2 points (correct behavior), so the
			// data must contain actual turns for the > 2 assertion to be meaningful.
			r := rand.New(rand.NewPCG(42, 42))
			seg := make(trackgeom.Segment, 50000)
			lat := 45.0
			lon := -75.0
			heading := 0.0
			for i := 0; i < 50000; i++ {
				// Periodic turns create real structure for DP to preserve.
				if i%5000 == 0 {
					heading += (r.Float64() - 0.5) * 2.0
				}
				lat += 0.0001 * math.Cos(heading)
				lon += 0.0001 * math.Sin(heading)
				// Small noise on top of the structured path.
				seg[i] = trackgeom.Point{
					Lat: lat + (r.Float64()-0.5)*0.0005,
					Lon: lon + (r.Float64()-0.5)*0.0005,
				}
			}

			track := &trackgeom.Track{
				Name:     "Giant Track",
				Segments: []trackgeom.Segment{seg},
			}

			startTime := time.Now()
			simplified := track.Simplify(1000)
			duration := time.Since(startTime)

			// Verify counts
			totalPoints := 0
			for _, s := range simplified.Segments {
				totalPoints += len(s)
			}

			// Report time
			GinkgoWriter.Printf("Simplifying 50k points took %v and resulted in %d points\n", duration, totalPoints)

			Expect(totalPoints).To(BeNumerically("<=", 1000))
			Expect(totalPoints).To(BeNumerically(">", 2)) // must not over-simplify to just start and end
			Expect(duration).To(BeNumerically("<", 2*time.Second), "Should complete within 2 seconds")
		})

		It("should handle worst-case collinear inputs with tiny perturbation safely", func() {
			// Collinear line of 10,000 points with one small peak in the middle
			// This represents the classic DP worst case where recursion can go deep
			seg := make(trackgeom.Segment, 10000)
			for i := 0; i < 10000; i++ {
				lat := 40.0 + 0.0001*float64(i)
				lon := -70.0 + 0.0001*float64(i)
				if i == 5000 {
					lat += 0.001 // tiny peak
				}
				seg[i] = trackgeom.Point{Lat: lat, Lon: lon}
			}

			track := &trackgeom.Track{
				Name:     "Worst Case Track",
				Segments: []trackgeom.Segment{seg},
			}

			startTime := time.Now()
			simplified := track.Simplify(500)
			duration := time.Since(startTime)

			totalPoints := 0
			for _, s := range simplified.Segments {
				totalPoints += len(s)
			}

			GinkgoWriter.Printf("Worst-case 10k points took %v and resulted in %d points\n", duration, totalPoints)

			Expect(totalPoints).To(BeNumerically("<=", 500))
			Expect(totalPoints).To(BeNumerically(">=", 3)) // should keep start, peak, and end
		})
	})
})

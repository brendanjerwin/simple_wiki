//revive:disable:dot-imports
package design_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDesign(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Design Token Suite")
}

// relativeTokenCSS resolves static/css/default.css from this package's
// directory regardless of the test's working directory.
func relativeTokenCSS() string {
	css, err := os.ReadFile(filepath.Join("..", "..", "static", "css", "default.css"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return string(css)
}

// tokenValues extracts the raw hex value of a custom property. It requires
// exactly one definition per mode block (light :root, dark media block).
func tokenValues(css, prop string) []string {
	var values []string
	for _, line := range strings.Split(css, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prop+":") {
			continue
		}
		value := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, prop+":")), ";")
		ExpectWithOffset(1, value).To(HavePrefix("#"), "token %s must be a hex color", prop)
		values = append(values, strings.ToLower(value))
	}
	return values
}

// relativeLuminance converts a color to its WCAG relative luminance.
// https://www.w3.org/TR/WCAG22/#dfn-relative-luminance
func relativeLuminance(hex string) float64 {
	Expect(hex).To(MatchRegexp(`^#[0-9a-f]{6}$`), "expected a 6-digit hex color")
	channel := func(offset int) float64 {
		channelValue, err := strconv.ParseInt(hex[1+offset:3+offset], 16, 64)
		Expect(err).NotTo(HaveOccurred())
		normalized := float64(channelValue) / 255
		if normalized <= 0.04045 {
			return normalized / 12.92
		}
		return math.Pow((normalized+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(0) + 0.7152*channel(2) + 0.0722*channel(4)
}

// contrastRatio computes the WCAG 2.2 contrast ratio between two colors.
// WCAG AA requires ≥ 4.5:1 for normal text.
func contrastRatio(foreground, background string) float64 {
	lightFg := relativeLuminance(foreground)
	lightBg := relativeLuminance(background)
	if lightFg < lightBg {
		lightFg, lightBg = lightBg, lightFg
	}
	return (lightFg + 0.05) / (lightBg + 0.05)
}

// lightMutedOnSurfaces asserts the light-mode --color-text-muted token clears
// WCAG AA (≥4.5:1) on every light surface it renders against. The sunken check
// is the trap: it is the harder background (lighter than pure text backgrounds
// elsewhere) and silently dropped below 4.5 when the 2026-10 muted token was
// chosen by eye.
var _ = Describe("muted text token AA contrast", func() {
	It("should have exactly one light-mode and one dark-mode definition", func() {
		values := tokenValues(relativeTokenCSS(), "--color-text-muted")
		Expect(values).To(HaveLen(2))
	})

	Describe("when the light-mode muted token renders on light surfaces", func() {
		type surface struct {
			name string
			hex  string
		}

		var (
			values   []string
			surfaces = []surface{
				{name: "surface-primary", hex: "#ffffff"},
				{name: "surface-elevated", hex: "#ffffff"},
				{name: "surface-sunken", hex: "#f8f9fa"},
			}
		)

		BeforeEach(func() {
			values = tokenValues(relativeTokenCSS(), "--color-text-muted")
		})

		for _, surface := range surfaces {
			surface := surface
			It(fmt.Sprintf("should clear 4.5:1 on %s", surface.name), func() {
				ratio := contrastRatio(values[0], surface.hex)
				Expect(ratio).To(BeNumerically(">=", 4.5),
					"light --color-text-muted (%s) must meet WCAG AA (≥4.5:1) on %s", values[0], surface.name)
			})
		}
	})

	Describe("when the dark-mode muted token renders on dark surfaces", func() {
		type surface struct {
			name string
			hex  string
		}

		var (
			values   []string
			surfaces = []surface{
				{name: "surface-primary", hex: "#1e1e1e"},
				{name: "surface-elevated", hex: "#2d2d2d"},
				{name: "surface-sunken", hex: "#141414"},
			}
		)

		BeforeEach(func() {
			values = tokenValues(relativeTokenCSS(), "--color-text-muted")
		})

		for _, surface := range surfaces {
			surface := surface
			It(fmt.Sprintf("should clear 4.5:1 on %s", surface.name), func() {
				ratio := contrastRatio(values[1], surface.hex)
				Expect(ratio).To(BeNumerically(">=", 4.5),
					"dark --color-text-muted (%s) must meet WCAG AA (≥4.5:1) on %s", values[1], surface.name)
			})
		}
	})
})

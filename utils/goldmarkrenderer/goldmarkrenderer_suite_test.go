//revive:disable:dot-imports
package goldmarkrenderer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGoldmarkRenderer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Goldmark Renderer Suite")
}
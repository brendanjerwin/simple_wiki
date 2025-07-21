//revive:disable:dot-imports
package goldmarkrenderer

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/internal/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGoldmarkRenderer(t *testing.T) {
	testutils.EnforceDevboxInCI()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Goldmark Renderer Suite")
}
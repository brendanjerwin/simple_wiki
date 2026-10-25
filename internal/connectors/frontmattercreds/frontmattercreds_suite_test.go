//revive:disable:dot-imports
package frontmattercreds_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFrontmattercreds(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/connectors/frontmattercreds")
}

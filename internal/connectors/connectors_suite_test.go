//revive:disable:dot-imports
package connectors_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConnectors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Connectors Suite")
}

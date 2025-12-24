//revive:disable:dot-imports
package bleve_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBleve(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bleve Suite")
}

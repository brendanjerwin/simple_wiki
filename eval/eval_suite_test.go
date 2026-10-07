//revive:disable:dot-imports
package eval_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEval(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eval Suite")
}

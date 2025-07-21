//revive:disable:dot-imports
package server

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/internal/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	testutils.EnforceDevboxInCI()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

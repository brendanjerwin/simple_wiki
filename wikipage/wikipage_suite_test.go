package wikipage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWikipage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wikipage Suite")
}
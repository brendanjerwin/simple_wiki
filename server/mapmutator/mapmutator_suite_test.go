package mapmutator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMapMutator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MapMutator Suite")
}

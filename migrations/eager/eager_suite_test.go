package eager

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eager Migrations Suite")
}
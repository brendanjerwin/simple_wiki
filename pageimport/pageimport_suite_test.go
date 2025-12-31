package pageimport_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPageimport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pageimport Suite")
}

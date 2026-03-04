package filestore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFilestoreSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filestore Suite")
}

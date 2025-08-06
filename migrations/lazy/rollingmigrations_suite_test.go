package lazy_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRollingmigrations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rollingmigrations Suite")
}
package surveymutator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSurveyMutator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Survey Mutator Suite")
}

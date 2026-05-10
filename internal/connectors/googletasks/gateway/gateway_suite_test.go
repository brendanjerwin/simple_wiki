//revive:disable:dot-imports
package gateway_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGoogleTasksGateway(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/connectors/google_tasks/gateway")
}

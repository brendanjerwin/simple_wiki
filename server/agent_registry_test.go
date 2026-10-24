//revive:disable:dot-imports
package server_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

// fakeAvailChecker is a stub AgentAvailabilityChecker for registry tests.
type fakeAvailChecker struct {
	poolConnected bool
}

func (f *fakeAvailChecker) HasInstanceRequestSubscribers() bool {
	return f.poolConnected
}

var _ = Describe("AgentRegistry", func() {
	Describe("NewAgentRegistry", func() {
		It("should pre-register the local ACP agent", func() {
			reg := server.NewAgentRegistry(&fakeAvailChecker{})
			agents := reg.List()
			Expect(agents).To(HaveLen(1))
			Expect(agents[0].GetId()).To(Equal(server.LocalACPAgentID))
		})
	})

	Describe("List", func() {
		Describe("when the pool is connected", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry(&fakeAvailChecker{poolConnected: true})
				agents = reg.List()
			})

			It("should report the local ACP agent as available", func() {
				Expect(agents[0].GetAvailable()).To(BeTrue())
			})

			It("should return an ACP_LOCAL protocol", func() {
				Expect(agents[0].GetProtocol()).To(Equal(apiv1.AgentProtocol_ACP_LOCAL))
			})
		})

		Describe("when the pool is not connected", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry(&fakeAvailChecker{poolConnected: false})
				agents = reg.List()
			})

			It("should report the local ACP agent as unavailable", func() {
				Expect(agents[0].GetAvailable()).To(BeFalse())
			})
		})

		Describe("when the availability checker is nil", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry(nil)
				agents = reg.List()
			})

			It("should report the local ACP agent as unavailable without panicking", func() {
				Expect(agents[0].GetAvailable()).To(BeFalse())
			})
		})

		It("should return copies so mutations do not affect the registry", func() {
			reg := server.NewAgentRegistry(&fakeAvailChecker{})
			agents := reg.List()
			agents[0].DisplayName = "mutated"
			freshList := reg.List()
			Expect(freshList[0].GetDisplayName()).NotTo(Equal("mutated"))
		})
	})

	Describe("FindByID", func() {
		Describe("when the agent exists", func() {
			var found *apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry(&fakeAvailChecker{})
				found = reg.FindByID(server.LocalACPAgentID)
			})

			It("should return the agent info", func() {
				Expect(found).NotTo(BeNil())
				Expect(found.GetId()).To(Equal(server.LocalACPAgentID))
			})
		})

		Describe("when the agent does not exist", func() {
			var found *apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry(&fakeAvailChecker{})
				found = reg.FindByID("nonexistent-agent")
			})

			It("should return nil", func() {
				Expect(found).To(BeNil())
			})
		})
	})

	Describe("DefaultAgentID", func() {
		It("should return the local ACP agent ID", func() {
			reg := server.NewAgentRegistry(&fakeAvailChecker{})
			Expect(reg.DefaultAgentID()).To(Equal(server.LocalACPAgentID))
		})
	})
})

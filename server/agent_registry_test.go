//revive:disable:dot-imports
package server_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

var _ = Describe("AgentRegistry", func() {
	Describe("DefaultAgentID", func() {
		It("should return the local ACP agent id", func() {
			reg := server.NewAgentRegistry()
			Expect(reg.DefaultAgentID()).To(Equal(server.LocalACPAgentID))
		})
	})

	Describe("when freshly constructed", func() {
		var agents []*apiv1.AgentInfo

		BeforeEach(func() {
			agents = server.NewAgentRegistry().List()
		})

		It("should list no agents until one registers", func() {
			Expect(agents).To(BeEmpty())
		})
	})

	Describe("RegisterPoolAgent", func() {
		Describe("when the pool registers the local ACP agent", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				err := reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:          server.LocalACPAgentID,
					DisplayName: "Local Agent",
					Description: "The bundled ACP agent.",
					Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
					Route:       "pi-acp",
				})
				Expect(err).NotTo(HaveOccurred())
				agents = reg.List()
			})

			It("should have registered the descriptor", func() {
				Expect(agents[0].GetId()).To(Equal(server.LocalACPAgentID))
			})

			It("should mark the agent available after registration", func() {
				Expect(agents[0].GetAvailable()).To(BeTrue())
			})

			It("should carry the route reference", func() {
				Expect(agents[0].GetRoute()).To(Equal("pi-acp"))
			})
		})

		Describe("when the pool re-registers after reconnect", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				err := reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:          server.LocalACPAgentID,
					DisplayName: "Local Agent",
					Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
				})
				Expect(err).NotTo(HaveOccurred())
				err = reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:          server.LocalACPAgentID,
					DisplayName: "Local Agent (renamed)",
					Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
				})
				Expect(err).NotTo(HaveOccurred())
				agents = reg.List()
			})

			It("should be an idempotent upsert with fresh fields", func() {
				Expect(agents).To(HaveLen(1))
				Expect(agents[0].GetDisplayName()).To(Equal("Local Agent (renamed)"))
				Expect(agents[0].GetAvailable()).To(BeTrue())
			})
		})

		Describe("when a non-ACP protocol is registered", func() {
			var err error

			BeforeEach(func() {
				err = server.NewAgentRegistry().RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:       "agui-x",
					Protocol: apiv1.AgentProtocol_AG_UI_REMOTE,
				})
			})

			It("should reject it — AG-UI entries come from server config", func() {
				Expect(err).To(MatchError(ContainSubstring("ACP_LOCAL")))
			})
		})

		Describe("when the id differs from the local ACP id", func() {
			var err error

			BeforeEach(func() {
				err = server.NewAgentRegistry().RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:       "not-local-acp",
					Protocol: apiv1.AgentProtocol_ACP_LOCAL,
				})
			})

			It("should reject it — only the bundled pool id may register", func() {
				Expect(err).To(MatchError(ContainSubstring("not-local-acp")))
			})
		})
	})

	Describe("PoolDisconnected", func() {
		Describe("when the registered pool connection drops", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				Expect(reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:       server.LocalACPAgentID,
					Protocol: apiv1.AgentProtocol_ACP_LOCAL,
				})).NotTo(HaveOccurred())
				reg.PoolDisconnected()
				agents = reg.List()
			})

			It("should mark the ACP agent unavailable", func() {
				Expect(agents[0].GetAvailable()).To(BeFalse())
			})

			It("should keep the entry listed", func() {
				Expect(agents).To(HaveLen(1))
				Expect(agents[0].GetId()).To(Equal(server.LocalACPAgentID))
			})
		})
	})

	Describe("SetConfiguredAgents", func() {
		Describe("when AG-UI entries load from server config", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				Expect(reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:       server.LocalACPAgentID,
					Protocol: apiv1.AgentProtocol_ACP_LOCAL,
				})).NotTo(HaveOccurred())
				reg.SetConfiguredAgents([]*apiv1.AgentInfo{
					{
						Id:          "agui-helper",
						DisplayName: "Helper",
						Protocol:    apiv1.AgentProtocol_AG_UI_REMOTE,
						Route:       "https://agentos.example/agent/helper",
					},
				})
				agents = reg.List()
			})

			It("should list both the pool agent and the configured entry", func() {
				Expect(agents).To(HaveLen(2))
			})

			It("should advertise the AG-UI entry as unavailable (no backend yet)", func() {
				var agui *apiv1.AgentInfo
				for _, a := range agents {
					if a.GetId() == "agui-helper" {
						agui = a
					}
				}
				Expect(agui).NotTo(BeNil())
				Expect(agui.GetAvailable()).To(BeFalse())
				Expect(agui.GetProtocol()).To(Equal(apiv1.AgentProtocol_AG_UI_REMOTE))
			})

			It("should keep the registered pool agent available", func() {
				Expect(agents[0].GetAvailable()).To(BeTrue())
			})
		})

		Describe("when reloaded config no longer includes a previous AG-UI entry", func() {
			var removed *apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				reg.SetConfiguredAgents([]*apiv1.AgentInfo{
					{Id: "agui-helper", Protocol: apiv1.AgentProtocol_AG_UI_REMOTE},
					{Id: "agui-old", Protocol: apiv1.AgentProtocol_AG_UI_REMOTE},
				})
				reg.SetConfiguredAgents([]*apiv1.AgentInfo{
					{Id: "agui-helper", Protocol: apiv1.AgentProtocol_AG_UI_REMOTE},
				})
				_, removed = reg.FindByID("agui-old"), reg.FindByID("agui-old")
			})

			It("should drop the removed entry", func() {
				Expect(removed).To(BeNil())
			})
		})

		Describe("when config tries to overwrite the pool agent", func() {
			var agents []*apiv1.AgentInfo

			BeforeEach(func() {
				reg := server.NewAgentRegistry()
				Expect(reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:          server.LocalACPAgentID,
					DisplayName: "Pool Authority",
					Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
				})).NotTo(HaveOccurred())
				reg.SetConfiguredAgents([]*apiv1.AgentInfo{
					{Id: server.LocalACPAgentID, DisplayName: "Config Impostor", Protocol: apiv1.AgentProtocol_ACP_LOCAL},
				})
				agents = reg.List()
			})

			It("should keep the pool-registered descriptor", func() {
				Expect(agents[0].GetDisplayName()).To(Equal("Pool Authority"))
			})
		})
	})

	Describe("List", func() {
		It("should return copies so mutations do not affect the registry", func() {
			reg := server.NewAgentRegistry()
			Expect(reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
				Id:          server.LocalACPAgentID,
				DisplayName: "Local Agent",
				Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
			})).NotTo(HaveOccurred())
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
				reg := server.NewAgentRegistry()
				Expect(reg.RegisterPoolAgent(&apiv1.RegisterPoolAgentRequest{
					Id:       server.LocalACPAgentID,
					Protocol: apiv1.AgentProtocol_ACP_LOCAL,
				})).NotTo(HaveOccurred())
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
				found = server.NewAgentRegistry().FindByID("nonexistent-agent")
			})

			It("should return nil", func() {
				Expect(found).To(BeNil())
			})
		})
	})
})

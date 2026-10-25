//revive:disable:dot-imports
package server_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

var _ = Describe("LoadConfiguredAGUIAgents", func() {
	var configPath string

	AfterEach(func() {
		if configPath != "" {
			_ = os.Remove(configPath)
			configPath = ""
		}
	})

	writeConfig := func(content string) string {
		path := filepath.Join(GinkgoT().TempDir(), "agents.json")
		if content != "" {
			Expect(os.WriteFile(path, []byte(content), 0o600)).NotTo(HaveOccurred())
		}
		return path
	}

	Describe("when the config path is empty", func() {
		It("should return nil", func() {
			Expect(server.LoadConfiguredAGUIAgents("")).To(BeNil())
		})
	})

	Describe("when the config file does not exist", func() {
		It("should return nil without erroring", func() {
			path := filepath.Join(GinkgoT().TempDir(), "missing.json")
			Expect(server.LoadConfiguredAGUIAgents(path)).To(BeNil())
		})
	})

	Describe("when the config holds AG-UI agents", func() {
		var agents []*apiv1.AgentInfo

		BeforeEach(func() {
			configPath = writeConfig(`{
				"agents": [
					{
						"id": "agui-helper",
						"display_name": "Helper",
						"protocol": "AG_UI_REMOTE",
						"description": "Platform-hosted helper",
						"endpoint": "https://agentos.example/agent/helper"
					}
				]
			}`)
			agents = server.LoadConfiguredAGUIAgents(configPath)
		})

		It("should load the entry", func() {
			Expect(agents).To(HaveLen(1))
			Expect(agents[0].GetId()).To(Equal("agui-helper"))
			Expect(agents[0].GetDisplayName()).To(Equal("Helper"))
			Expect(agents[0].GetProtocol()).To(Equal(apiv1.AgentProtocol_AG_UI_REMOTE))
			Expect(agents[0].GetRoute()).To(Equal("https://agentos.example/agent/helper"))
		})

		It("should not mark it available (routing awaits #1275)", func() {
			Expect(agents[0].GetAvailable()).To(BeFalse())
		})
	})

	Describe("when the config holds malformed JSON", func() {
		It("should return nil", func() {
			configPath = writeConfig(`{ not json`)
			Expect(server.LoadConfiguredAGUIAgents(configPath)).To(BeNil())
		})
	})

	Describe("when an entry lacks a required field", func() {
		It("should skip entries without an id", func() {
			configPath = writeConfig(`{"agents": [{"display_name": "NoId", "protocol": "AG_UI_REMOTE"}]}`)
			Expect(server.LoadConfiguredAGUIAgents(configPath)).To(BeEmpty())
		})

		It("should skip entries with a non-AG_UI protocol", func() {
			configPath = writeConfig(`{"agents": [{"id": "x", "protocol": "ACP_LOCAL"}]}`)
			Expect(server.LoadConfiguredAGUIAgents(configPath)).To(BeEmpty())
		})
	})
})

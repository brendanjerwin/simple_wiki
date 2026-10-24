package server

import (
	"encoding/json"
	"os"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// aguiAgentsConfig is the on-disk shape of the server's static AG-UI agent
// configuration (issue #1276 design). Missing file or malformed content yields
// no entries — the inventory simply starts empty of configured agents.
type aguiAgentsConfig struct {
	Agents []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Protocol    string `json:"protocol"`
		Description string `json:"description"`
		Endpoint    string `json:"endpoint"`
	} `json:"agents"`
}

// LoadConfiguredAGUIAgents reads AG-UI remote agent entries from the given
// config file path. Empty path, missing file, malformed JSON, and entry-level
// validation failures all yield fewer/no entries rather than failing startup:
// the registry is additive, and the pool agent registers independently.
func LoadConfiguredAGUIAgents(path string) []*apiv1.AgentInfo {
	if path == "" {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cfg aguiAgentsConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}

	out := make([]*apiv1.AgentInfo, 0, len(cfg.Agents))
	for _, a := range cfg.Agents {
		if a.ID == "" || a.Protocol != "AG_UI_REMOTE" {
			continue
		}
		out = append(out, &apiv1.AgentInfo{
			Id:          a.ID,
			DisplayName: a.DisplayName,
			Protocol:    apiv1.AgentProtocol_AG_UI_REMOTE,
			Description: a.Description,
			Route:       a.Endpoint,
			Available:   false, // routing awaits #1275's AG-UI backend
		})
	}
	return out
}

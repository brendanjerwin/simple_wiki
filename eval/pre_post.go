package eval

import "strings"

// prePRStubServices lists services whose proto comments were stubs ("see (api.v1.description)")
// before the improve-mcp-surface PR wired mcpdocs.Decorate into the in-process path.
// On the pre-PR surface, these tools had stub descriptions instead of curated text.
var prePRStubServices = []string{
	"AgentMetadataService",
	"ChecklistService",
	"Frontmatter",
	"PageHistoryService",
	"InventoryManagementService",
	"MapService",
	"PageImportService",
	"PageManagementService",
	"SearchService",
	"SystemInfoService",
	"ConnectorService",
}

// prePREmptyDescTools lists tools whose descriptions were empty before the PR
// because the proto had no comment and no (api.v1.description) extension.
// SurveyService RPCs — all 9 — were empty on both MCP paths.
var prePREmptyDescTools = []string{
	"api_v1_SurveyService_GetSurvey",
	"api_v1_SurveyService_UpsertSurvey",
	"api_v1_SurveyService_AddField",
	"api_v1_SurveyService_UpdateField",
	"api_v1_SurveyService_RemoveField",
	"api_v1_SurveyService_ReorderField",
	"api_v1_SurveyService_SubmitResponse",
	"api_v1_SurveyService_ListResponses",
	"api_v1_SurveyService_DeleteResponse",
}

// prePRExtraTools lists tools that were registered on the pre-PR in-process path
// but are excluded post-PR: ScheduledTurnService (removed entirely) and the 7
// ChatService RPCs annotated with exclude_from_mcp. These tools had real
// descriptions (from proto comments) but should not have been exposed to agents.
var prePRExtraTools = []ToolDef{
	{
		Name:        "api_v1_ScheduledTurnService_CompleteScheduledTurn",
		Description: "CompleteScheduledTurn is called by the pool when a headless turn ends\n(success, error, or timeout). The server uses request_id to wake the\nblocked AgentTurnJob so it can record the terminal transition.\n",
	},
	{
		Name:        "api_v1_ChatService_SendChatReply",
		Description: "SendChatReply is called by the pool daemon when the agent uses the reply tool.\nAccepts optional reply_to message ID for threading.\n",
	},
	{
		Name:        "api_v1_ChatService_EditChatMessage",
		Description: "EditChatMessage is called by the pool daemon when the agent uses the edit_message tool.\nUpdates an existing message's content.\n",
	},
	{
		Name:        "api_v1_ChatService_ReactToMessage",
		Description: "ReactToMessage is called by the pool daemon when the agent uses the react tool.\nAdds an emoji reaction to a message.\n",
	},
	{
		Name:        "api_v1_ChatService_SendToolCallNotification",
		Description: "SendToolCallNotification is called by the pool daemon or ACP client\nwhen the agent invokes a tool. The notification is broadcast to page subscribers.\n",
	},
	{
		Name:        "api_v1_ChatService_SendPlanNotification",
		Description: "SendPlanNotification is called by the pool daemon or ACP client when the\nagent reports or updates its execution plan. The plan is broadcast to page\nsubscribers so the UI can show live task progress.\n",
	},
	{
		Name:        "api_v1_ChatService_SendTurnStatus",
		Description: "SendTurnStatus is called by the pool daemon when an agent turn starts and\ncompletes, so the UI can show the Stop button for the whole turn.\n",
	},
	{
		Name:        "api_v1_ChatService_RespondToPermission",
		Description: "RespondToPermission is called by the frontend when the user responds to a permission request.\n",
	},
}

// prePRStubDesc generates the stub description a pre-PR tool would have had.
// The codegen bakes "<MethodName> — see (api.v1.description).\n" from the proto comment.
func prePRStubDesc(toolName string) string {
	// Extract method name from "api_v1_<Service>_<Method>"
	parts := strings.Split(toolName, "_")
	if len(parts) < 4 {
		return ""
	}
	method := parts[len(parts)-1]
	return method + " — see (api.v1.description).\n"
}

// isFromService returns true if the tool name contains the given service name
// in the "api_v1_<Service>_<Method>" pattern.
func isFromService(toolName, service string) bool {
	return strings.Contains(toolName, "_"+service+"_")
}

// ToPrePR transforms a post-PR ToolSurface into the pre-PR equivalent by:
//  1. Replacing descriptions for stub services with the "see (api.v1.description)" stub.
//  2. Replacing SurveyService descriptions with empty strings.
//  3. Re-adding the excluded tools (ScheduledTurn + pool-only ChatService RPCs).
//
// This is a data-driven transform, not a git checkout — it works against any
// future branch to isolate the PR's effect.
func ToPrePR(post ToolSurface) ToolSurface {
	pre := ToolSurface{
		Label: "pre-PR",
		Tools: make([]ToolDef, 0, len(post.Tools)+len(prePRExtraTools)),
	}

	// Build a set of tools that should get empty descriptions pre-PR.
	emptySet := make(map[string]bool, len(prePREmptyDescTools))
	for _, name := range prePREmptyDescTools {
		emptySet[name] = true
	}

	for _, tool := range post.Tools {
		t := tool

		// SurveyService: empty descriptions pre-PR
		if emptySet[t.Name] {
			t.Description = ""
		} else {
			// Check if this tool belongs to a stub service
			for _, svc := range prePRStubServices {
				if isFromService(t.Name, svc) {
					t.Description = prePRStubDesc(t.Name)
					break
				}
			}
		}

		pre.Tools = append(pre.Tools, t)
	}

	// Re-add excluded tools
	pre.Tools = append(pre.Tools, prePRExtraTools...)

	return pre
}

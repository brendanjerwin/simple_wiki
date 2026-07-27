package eval

// Case is one evaluation scenario: a natural-language user request and the
// expected tool selection.
type Case struct {
	ID           string         `json:"id"`
	Query        string         `json:"query"`
	ExpectedTool string         `json:"expected_tool"`
	ExpectedArgs map[string]any `json:"expected_args,omitempty"`
	// ExcludedTool is set for exclusion cases: a tool that must NOT be selected.
	// When set, ExpectedTool may be empty (meaning "select nothing / decline").
	ExcludedTool string   `json:"excluded_tool,omitempty"`
	Services     []string `json:"services"`
	Tags         []string `json:"tags"`
}

// Cases is the golden case set. Each case maps a natural-language user request
// to the expected MCP tool. Cases are organized by service and cover happy path,
// disambiguation, exclusion, naming sensitivity, and cross-service confusion.
//
// To add cases: append to this slice. IDs must be stable and unique.
var Cases = []Case{
	// --- PageManagementService ---
	{
		ID:           "pm-create-page",
		Query:        "Create a new wiki page called 'garden_plan' with the title 'Garden Plan'",
		ExpectedTool: "api_v1_PageManagementService_CreatePage",
		ExpectedArgs: map[string]any{"identifier": "garden_plan"},
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "create", "happy-path"},
	},
	{
		ID:           "pm-read-page",
		Query:        "Show me the contents of the 'weekly_menu' page",
		ExpectedTool: "api_v1_PageManagementService_ReadPage",
		ExpectedArgs: map[string]any{"identifier": "weekly_menu"},
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "read", "happy-path"},
	},
	{
		ID:           "pm-read-outline",
		Query:        "I need to see the heading structure of a very large page before reading it",
		ExpectedTool: "api_v1_PageManagementService_ReadPageOutline",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "read", "large-pages"},
	},
	{
		ID:           "pm-delete-page",
		Query:        "Delete the 'old_drafts' page",
		ExpectedTool: "api_v1_PageManagementService_DeletePage",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "delete", "happy-path"},
	},
	{
		ID:           "pm-render-markdown",
		Query:        "Render this markdown snippet to HTML for me without saving it to a page",
		ExpectedTool: "api_v1_PageManagementService_RenderMarkdown",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "render", "disambiguation"},
	},
	{
		ID:           "pm-generate-identifier",
		Query:        "Convert 'My New Recipe Page' into a wiki page identifier",
		ExpectedTool: "api_v1_PageManagementService_GenerateIdentifier",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "identifier", "happy-path"},
	},

	// --- Frontmatter ---
	{
		ID:           "fm-get",
		Query:        "What's the frontmatter on the 'project_roadmap' page?",
		ExpectedTool: "api_v1_Frontmatter_GetFrontmatter",
		ExpectedArgs: map[string]any{"page": "project_roadmap"},
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "read", "happy-path"},
	},
	{
		ID:           "fm-merge",
		Query:        "Add a 'status' field set to 'published' to the frontmatter of 'blog_post_42'",
		ExpectedTool: "api_v1_Frontmatter_MergeFrontmatter",
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "write", "happy-path"},
	},
	{
		ID:           "fm-replace",
		Query:        "Replace the entire frontmatter of 'test_page' with a fresh set of metadata",
		ExpectedTool: "api_v1_Frontmatter_ReplaceFrontmatter",
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "write", "disambiguation"},
	},
	{
		ID:           "fm-remove-key",
		Query:        "Remove the 'deprecated_field' key from the frontmatter of 'config_page'",
		ExpectedTool: "api_v1_Frontmatter_RemoveKeyAtPath",
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "write", "happy-path"},
	},

	// --- SearchService ---
	{
		ID:           "search-content",
		Query:        "Search the wiki for pages mentioning 'solar panels'",
		ExpectedTool: "api_v1_SearchService_SearchContent",
		Services:     []string{"SearchService"},
		Tags:         []string{"search", "happy-path"},
	},
	{
		ID:           "search-by-frontmatter",
		Query:        "List all blog posts with status=published, sorted by published-date",
		ExpectedTool: "api_v1_SearchService_ListPagesByFrontmatter",
		Services:     []string{"SearchService"},
		Tags:         []string{"search", "frontmatter", "disambiguation"},
	},

	// --- PageHistoryService ---
	{
		ID:           "history-list-versions",
		Query:        "Show me the version history of the 'meeting_notes' page",
		ExpectedTool: "api_v1_PageHistoryService_ListPageVersions",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "read", "happy-path"},
	},
	{
		ID:           "history-read-version",
		Query:        "Show me what the 'meeting_notes' page looked like in version 5",
		ExpectedTool: "api_v1_PageHistoryService_ReadPageVersion",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "read", "disambiguation"},
	},
	{
		ID:           "history-diff",
		Query:        "What changed between version 3 and version 7 of the 'roadmap' page?",
		ExpectedTool: "api_v1_PageHistoryService_DiffPageVersions",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "read", "disambiguation"},
	},
	{
		ID:           "history-restore",
		Query:        "Restore the 'roadmap' page back to how it was in version 3",
		ExpectedTool: "api_v1_PageHistoryService_RestorePageVersion",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "write", "disambiguation"},
	},

	// --- ChecklistService ---
	{
		ID:           "checklist-add-item",
		Query:        "Add 'Buy milk #urgent' to the 'grocery_list' checklist on the 'grocery_planning' page",
		ExpectedTool: "api_v1_ChecklistService_AddItem",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "write", "happy-path", "naming"},
	},
	{
		ID:           "checklist-toggle-item",
		Query:        "Check off the first item in the 'todo' checklist on 'project_page'",
		ExpectedTool: "api_v1_ChecklistService_ToggleItem",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "write", "happy-path"},
	},
	{
		ID:           "checklist-list-items",
		Query:        "Show me all items in the 'this_week' checklist on 'grocery_planning'",
		ExpectedTool: "api_v1_ChecklistService_ListItems",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "read", "happy-path"},
	},
	{
		ID:           "checklist-deduplicate",
		Query:        "Remove duplicate items from the 'shopping' checklist on 'grocery_planning'",
		ExpectedTool: "api_v1_ChecklistService_DeduplicateItems",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "write", "disambiguation"},
	},

	// --- MapService ---
	{
		ID:           "map-add-marker",
		Query:        "Add a marker at latitude 41.1, longitude -72.2 to the 'yard' map on 'garden_plan'",
		ExpectedTool: "api_v1_MapService_AddMarker",
		Services:     []string{"MapService"},
		Tags:         []string{"map", "write", "happy-path"},
	},
	{
		ID:           "map-move-marker",
		Query:        "Move the garden bed marker 10 feet north on the 'yard' map",
		ExpectedTool: "api_v1_MapService_MoveMarker",
		Services:     []string{"MapService"},
		Tags:         []string{"map", "write", "disambiguation"},
	},
	{
		ID:           "map-delete-polygon",
		Query:        "Delete the 'fence_area' polygon from the 'yard' map on 'garden_plan'",
		ExpectedTool: "api_v1_MapService_DeletePolygon",
		Services:     []string{"MapService"},
		Tags:         []string{"map", "write", "happy-path"},
	},

	// --- SurveyService ---
	{
		ID:           "survey-get",
		Query:        "Show me the 'feedback_form' survey on the 'product_page'",
		ExpectedTool: "api_v1_SurveyService_GetSurvey",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "read", "happy-path", "naming"},
	},
	{
		ID:           "survey-upsert",
		Query:        "Change the question on the 'feedback_form' survey to 'How was your experience?'",
		ExpectedTool: "api_v1_SurveyService_UpsertSurvey",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "write", "disambiguation", "naming"},
	},
	{
		ID:           "survey-submit-response",
		Query:        "Submit a response to the 'feedback_form' survey with rating 5",
		ExpectedTool: "api_v1_SurveyService_SubmitResponse",
		ExpectedArgs: map[string]any{"survey_name": "feedback_form"},
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "write", "happy-path", "naming"},
	},
	{
		ID:           "survey-add-field",
		Query:        "Add a 'comments' text field to the 'feedback_form' survey",
		ExpectedTool: "api_v1_SurveyService_AddField",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "write", "disambiguation", "naming"},
	},
	{
		ID:           "survey-list-responses",
		Query:        "Show me all responses submitted to the 'feedback_form' survey",
		ExpectedTool: "api_v1_SurveyService_ListResponses",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "read", "happy-path", "naming"},
	},

	// --- AgentMetadataService ---
	{
		ID:           "agent-list-schedules",
		Query:        "What scheduled agent tasks are set up on the 'pastry_project' page?",
		ExpectedTool: "api_v1_AgentMetadataService_ListSchedules",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "read", "happy-path"},
	},
	{
		ID:           "agent-upsert-schedule",
		Query:        "Schedule a background agent to run every Friday at 6pm on 'pastry_project' to draft the weekend order",
		ExpectedTool: "api_v1_AgentMetadataService_UpsertSchedule",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "write", "happy-path"},
	},
	{
		ID:           "agent-get-chat-context",
		Query:        "What does the agent remember about the conversation on 'pastry_project'?",
		ExpectedTool: "api_v1_AgentMetadataService_GetChatContext",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "read", "happy-path"},
	},
	{
		ID:           "agent-update-chat-context",
		Query:        "Save a memory that the user prefers bullet points on the 'pastry_project' page",
		ExpectedTool: "api_v1_AgentMetadataService_UpdateChatContext",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "write", "happy-path"},
	},

	// --- ConnectorService ---
	{
		ID:           "connector-get-state",
		Query:        "Am I connected to Google Keep?",
		ExpectedTool: "api_v1_ConnectorService_GetState",
		Services:     []string{"ConnectorService"},
		Tags:         []string{"connector", "read", "happy-path"},
	},
	{
		ID:           "connector-bind",
		Query:        "Bind the 'grocery_list' checklist on 'grocery_planning' to my Google Keep",
		ExpectedTool: "api_v1_ConnectorService_Bind",
		Services:     []string{"ConnectorService"},
		Tags:         []string{"connector", "write", "happy-path"},
	},
	{
		ID:           "connector-sync-now",
		Query:        "Sync the 'grocery_list' checklist on 'grocery_planning' with Google Keep right now",
		ExpectedTool: "api_v1_ConnectorService_SyncNow",
		Services:     []string{"ConnectorService"},
		Tags:         []string{"connector", "write", "disambiguation"},
	},

	// --- FileStorageService ---
	{
		ID:           "file-upload",
		Query:        "Upload this PDF file to the wiki",
		ExpectedTool: "api_v1_FileStorageService_UploadFile",
		Services:     []string{"FileStorageService"},
		Tags:         []string{"file", "write", "happy-path"},
	},
	{
		ID:           "file-get-info",
		Query:        "What's the metadata for the file with hash abc123?",
		ExpectedTool: "api_v1_FileStorageService_GetFileInfo",
		Services:     []string{"FileStorageService"},
		Tags:         []string{"file", "read", "happy-path"},
	},

	// --- InventoryManagementService ---
	{
		ID:           "inv-create-item",
		Query:        "Create a new inventory item page for my cordless drill",
		ExpectedTool: "api_v1_InventoryManagementService_CreateInventoryItem",
		Services:     []string{"InventoryManagementService"},
		Tags:         []string{"inventory", "write", "happy-path"},
	},
	{
		ID:           "inv-move-item",
		Query:        "Move the cordless drill from the workshop shelf to the garage shelf",
		ExpectedTool: "api_v1_InventoryManagementService_MoveInventoryItem",
		Services:     []string{"InventoryManagementService"},
		Tags:         []string{"inventory", "write", "cross-service-confusion"},
	},
	{
		ID:           "inv-find-location",
		Query:        "Where is the cordless drill stored?",
		ExpectedTool: "api_v1_InventoryManagementService_FindItemLocation",
		Services:     []string{"InventoryManagementService"},
		Tags:         []string{"inventory", "read", "happy-path"},
	},
	{
		ID:           "inv-list-contents",
		Query:        "List everything on the garage shelf, including items in nested containers",
		ExpectedTool: "api_v1_InventoryManagementService_ListContainerContents",
		Services:     []string{"InventoryManagementService"},
		Tags:         []string{"inventory", "read", "happy-path"},
	},

	// --- PageImportService ---
	{
		ID:           "import-preview",
		Query:        "Preview this CSV import — show me what pages would be created without actually writing them",
		ExpectedTool: "api_v1_PageImportService_ParseCSVPreview",
		Services:     []string{"PageImportService"},
		Tags:         []string{"import", "read", "disambiguation"},
	},
	{
		ID:           "import-start-job",
		Query:        "Start importing this CSV file into the wiki as pages",
		ExpectedTool: "api_v1_PageImportService_StartPageImportJob",
		Services:     []string{"PageImportService"},
		Tags:         []string{"import", "write", "happy-path"},
	},

	// --- SystemInfoService ---
	{
		ID:           "sysinfo-get-version",
		Query:        "What version of the wiki server am I connected to?",
		ExpectedTool: "api_v1_SystemInfoService_GetVersion",
		Services:     []string{"SystemInfoService"},
		Tags:         []string{"sysinfo", "read", "happy-path"},
	},
	{
		ID:           "sysinfo-get-job-status",
		Query:        "What's the status of the background job queues?",
		ExpectedTool: "api_v1_SystemInfoService_GetJobStatus",
		Services:     []string{"SystemInfoService"},
		Tags:         []string{"sysinfo", "read", "happy-path"},
	},

	// --- ChatService (exclusion cases) ---
	{
		ID:           "chat-send-message",
		Query:        "Send a chat message 'Hello, can you help me?' on the 'home' page",
		ExpectedTool: "api_v1_ChatService_SendMessage",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "write", "happy-path"},
	},
	{
		ID:           "chat-get-status",
		Query:        "Is an agent currently connected to the 'home' page chat?",
		ExpectedTool: "api_v1_ChatService_GetChatStatus",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "read", "happy-path"},
	},
	{
		ID:           "chat-exclude-reply",
		Query:        "Reply to the user's chat message with 'Sure, I can help with that'",
		ExcludedTool: "api_v1_ChatService_SendChatReply",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion"},
	},
	{
		ID:           "chat-exclude-edit",
		Query:        "Edit my last chat message to fix a typo",
		ExcludedTool: "api_v1_ChatService_EditChatMessage",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion"},
	},
	{
		ID:           "chat-exclude-react",
		Query:        "Add a thumbs-up emoji reaction to the user's message",
		ExcludedTool: "api_v1_ChatService_ReactToMessage",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion"},
	},

	// --- Cross-service confusion ---
	{
		ID:           "confusion-move-inventory-not-map",
		Query:        "Move the workshop drill to the garage shelf",
		ExpectedTool: "api_v1_InventoryManagementService_MoveInventoryItem",
		Services:     []string{"InventoryManagementService", "MapService"},
		Tags:         []string{"cross-service-confusion", "inventory", "map"},
	},
	{
		ID:           "confusion-history-not-delete",
		Query:        "Show me what the 'roadmap' page looked like last week",
		ExpectedTool: "api_v1_PageHistoryService_ListPageVersions",
		Services:     []string{"PageHistoryService", "PageManagementService"},
		Tags:         []string{"cross-service-confusion", "history", "page"},
	},
	{
		ID:           "confusion-search-not-frontmatter",
		Query:        "Find all pages that contain the word 'solar'",
		ExpectedTool: "api_v1_SearchService_SearchContent",
		Services:     []string{"SearchService", "Frontmatter"},
		Tags:         []string{"cross-service-confusion", "search", "frontmatter"},
	},

	// --- HARD: Same-service disambiguation (description quality matters) ---
	{
		ID:           "hard-survey-upsert-vs-update-field",
		Query:        "Change the label on the 'rating' field of the 'feedback_form' survey from 'Rate' to 'Your Rating'",
		ExpectedTool: "api_v1_SurveyService_UpdateField",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "disambiguation", "hard"},
	},
	{
		ID:           "hard-survey-upsert-vs-add-field",
		Query:        "Set the survey question to 'How was your experience?' and mark it as closed",
		ExpectedTool: "api_v1_SurveyService_UpsertSurvey",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "disambiguation", "hard", "naming"},
	},
	{
		ID:           "hard-checklist-toggle-vs-update",
		Query:        "Mark the 'Buy milk' item as done on the 'grocery_list' checklist on 'grocery_planning'",
		ExpectedTool: "api_v1_ChecklistService_ToggleItem",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "disambiguation", "hard"},
	},
	{
		ID:           "hard-checklist-delete-vs-toggle",
		Query:        "Remove the 'Buy milk' item entirely from the 'grocery_list' checklist on 'grocery_planning'",
		ExpectedTool: "api_v1_ChecklistService_DeleteItem",
		Services:     []string{"Checkmatter"},
		Tags:         []string{"checklist", "disambiguation", "hard"},
	},
	{
		ID:           "hard-frontmatter-merge-vs-replace",
		Query:        "I want to add a 'tags' array with 'cooking' and 'recipes' to the frontmatter of 'recipe_page' without losing the existing title field",
		ExpectedTool: "api_v1_Frontmatter_MergeFrontmatter",
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "disambiguation", "hard"},
	},
	{
		ID:           "hard-frontmatter-replace-vs-merge",
		Query:        "Wipe the frontmatter on 'test_page' and set only 'title' to 'Fresh Start' — nothing else should survive",
		ExpectedTool: "api_v1_Frontmatter_ReplaceFrontmatter",
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "disambiguation", "hard"},
	},
	{
		ID:           "hard-page-update-content-vs-update-page",
		Query:        "Change just the markdown body of 'meeting_notes' — leave the frontmatter exactly as it is",
		ExpectedTool: "api_v1_PageManagementService_UpdatePageContent",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "disambiguation", "hard"},
	},
	{
		ID:           "hard-page-update-whole-vs-content",
		Query:        "Replace the entire 'meeting_notes' page including both frontmatter and body in one write",
		ExpectedTool: "api_v1_PageManagementService_UpdateWholePage",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "disambiguation", "hard"},
	},
	{
		ID:           "hard-history-search-vs-list",
		Query:        "Search through all past versions of every page for mentions of 'budget'",
		ExpectedTool: "api_v1_PageHistoryService_SearchHistory",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "disambiguation", "hard"},
	},
	{
		ID:           "hard-history-search-page-vs-global",
		Query:        "Search within just the 'roadmap' page's version history for 'milestone'",
		ExpectedTool: "api_v1_PageHistoryService_SearchPageHistory",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "disambiguation", "hard"},
	},
	{
		ID:           "hard-map-add-circle-vs-add-polygon",
		Query:        "Draw a 50-meter radius circle around the mailbox on the 'yard' map on 'garden_plan'",
		ExpectedTool: "api_v1_MapService_AddCircle",
		Services:     []string{"MapService"},
		Tags:         []string{"map", "disambiguation", "hard"},
	},
	{
		ID:           "hard-map-add-track-vs-add-marker",
		Query:        "Trace the walking path from the shed to the garden beds on the 'yard' map",
		ExpectedTool: "api_v1_MapService_AddTrack",
		Services:     []string{"MapService"},
		Tags:         []string{"map", "disambiguation", "hard"},
	},
	{
		ID:           "hard-connector-unbind-vs-sync",
		Query:        "Stop syncing the 'grocery_list' checklist on 'grocery_planning' with Google Keep — disconnect it entirely",
		ExpectedTool: "api_v1_ConnectorService_Unbind",
		Services:     []string{"ConnectorService"},
		Tags:         []string{"connector", "disambiguation", "hard"},
	},
	{
		ID:           "hard-connector-disconnect-vs-unbind",
		Query:        "Completely revoke my Google Keep credentials so nothing syncs anymore",
		ExpectedTool: "api_v1_ConnectorService_Disconnect",
		Services:     []string{"ConnectorService"},
		Tags:         []string{"connector", "disambiguation", "hard"},
	},
	{
		ID:           "hard-agent-delete-vs-upsert",
		Query:        "Remove the 'friday_draft' schedule from 'pastry_project' — it should no longer fire",
		ExpectedTool: "api_v1_AgentMetadataService_DeleteSchedule",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "disambiguation", "hard"},
	},
	{
		ID:           "hard-agent-append-vs-update",
		Query:        "Add a one-line summary 'Drafted the order list for the weekend' to the background activity log for the last scheduled run on 'pastry_project'",
		ExpectedTool: "api_v1_AgentMetadataService_AppendBackgroundActivitySummary",
		Services:     []string{"AgentMetadataService"},
		Tags:         []string{"agent", "disambiguation", "hard"},
	},

	// --- HARD: Vague natural language (no tool keywords in query) ---
	{
		ID:           "hard-vague-create-page",
		Query:        "I want to start a new page about my tomato garden",
		ExpectedTool: "api_v1_PageManagementService_CreatePage",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "vague", "hard"},
	},
	{
		ID:           "hard-vague-checklist-add",
		Query:        "I need to remember to pick up dry cleaning on Thursday",
		ExpectedTool: "api_v1_ChecklistService_AddItem",
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "vague", "hard"},
	},
	{
		ID:           "hard-vague-search",
		Query:        "Where did I write about the roof repair timeline?",
		ExpectedTool: "api_v1_SearchService_SearchContent",
		Services:     []string{"SearchService"},
		Tags:         []string{"search", "vague", "hard"},
	},
	{
		ID:           "hard-vague-inventory-find",
		Query:        "Where did I put the pipe wrench?",
		ExpectedTool: "api_v1_InventoryManagementService_FindItemLocation",
		Services:     []string{"InventoryManagementService"},
		Tags:         []string{"inventory", "vague", "hard"},
	},
	{
		ID:           "hard-vague-history-diff",
		Query:        "What's different between how the page looked on Monday vs how it looks now?",
		ExpectedTool: "api_v1_PageHistoryService_DiffPageVersions",
		Services:     []string{"PageHistoryService"},
		Tags:         []string{"history", "vague", "hard"},
	},
	{
		ID:           "hard-vague-survey-submit",
		Query:        "I'd like to give my feedback on the product — rate it 4 out of 5",
		ExpectedTool: "api_v1_SurveyService_SubmitResponse",
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "vague", "hard", "naming"},
	},

	// --- HARD: Naming traps (F6-sensitive — pre-PR should fail more) ---
	{
		ID:           "hard-naming-survey-name-field",
		Query:        "Submit a response to the 'customer_satisfaction' survey on 'product_page' with values {\"rating\": 5}",
		ExpectedTool: "api_v1_SurveyService_SubmitResponse",
		ExpectedArgs: map[string]any{"survey_name": "customer_satisfaction"},
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "naming", "hard", "args"},
	},
	{
		ID:           "hard-naming-survey-vs-page-identity",
		Query:        "Read the 'feedback_form' survey that lives on the 'product_page' page",
		ExpectedTool: "api_v1_SurveyService_GetSurvey",
		ExpectedArgs: map[string]any{"page": "product_page", "name": "feedback_form"},
		Services:     []string{"SurveyService"},
		Tags:         []string{"survey", "naming", "hard", "args"},
	},
	{
		ID:           "hard-naming-checklist-page-field",
		Query:        "Add 'water the plants' to the 'garden_tasks' checklist on the 'garden_plan' page",
		ExpectedTool: "api_v1_ChecklistService_AddItem",
		ExpectedArgs: map[string]any{"page": "garden_plan", "list_name": "garden_tasks"},
		Services:     []string{"ChecklistService"},
		Tags:         []string{"checklist", "naming", "hard", "args"},
	},
	{
		ID:           "hard-naming-frontmatter-page-field",
		Query:        "Read the frontmatter from the 'project_roadmap' page",
		ExpectedTool: "api_v1_Frontmatter_GetFrontmatter",
		ExpectedArgs: map[string]any{"page": "project_roadmap"},
		Services:     []string{"Frontmatter"},
		Tags:         []string{"frontmatter", "naming", "hard", "args"},
	},
	{
		ID:           "hard-naming-page-identifier-vs-name",
		Query:        "Show me the page whose identifier is 'weekly_menu'",
		ExpectedTool: "api_v1_PageManagementService_ReadPage",
		ExpectedArgs: map[string]any{"identifier": "weekly_menu"},
		Services:     []string{"PageManagementService"},
		Tags:         []string{"page", "naming", "hard", "args"},
	},

	// --- HARD: Cross-service confusion (expanded) ---
	{
		ID:           "hard-confusion-map-move-vs-inventory-move",
		Query:        "Relocate the shed marker 20 feet east on the yard map",
		ExpectedTool: "api_v1_MapService_MoveMarker",
		Services:     []string{"MapService", "InventoryManagementService"},
		Tags:         []string{"cross-service-confusion", "map", "inventory", "hard"},
	},
	{
		ID:           "hard-confusion-agent-vs-frontmatter",
		Query:        "Save a note that the user likes Italian food as part of the agent's memory on 'pastry_project'",
		ExpectedTool: "api_v1_AgentMetadataService_UpdateChatContext",
		Services:     []string{"AgentMetadataService", "Frontmatter"},
		Tags:         []string{"cross-service-confusion", "agent", "frontmatter", "hard"},
	},
	{
		ID:           "hard-confusion-read-page-vs-read-outline",
		Query:        "I just need a quick overview of the headings on 'meeting_notes' — I don't want the full text",
		ExpectedTool: "api_v1_PageManagementService_ReadPageOutline",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"cross-service-confusion", "page", "hard"},
	},
	{
		ID:           "hard-confusion-read-section-vs-read-page",
		Query:        "Show me just the 'Budget' section of the 'roadmap' page, not the whole thing",
		ExpectedTool: "api_v1_PageManagementService_ReadPageSection",
		Services:     []string{"PageManagementService"},
		Tags:         []string{"cross-service-confusion", "page", "hard"},
	},
	{
		ID:           "hard-confusion-list-trash-vs-search",
		Query:        "Show me pages I've recently deleted",
		ExpectedTool: "api_v1_PageManagementService_ListTrash",
		Services:     []string{"PageManagementService", "SearchService"},
		Tags:         []string{"cross-service-confusion", "page", "search", "hard"},
	},

	// --- HARD: Exclusion (expanded — pre-PR must fail these) ---
	{
		ID:           "hard-exclude-turn-status",
		Query:        "Tell the UI that the agent is currently working on a turn",
		ExcludedTool: "api_v1_ChatService_SendTurnStatus",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion", "hard"},
	},
	{
		ID:           "hard-exclude-plan-notification",
		Query:        "Notify the page that the agent's plan has been updated with three new tasks",
		ExcludedTool: "api_v1_ChatService_SendPlanNotification",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion", "hard"},
	},
	{
		ID:           "hard-exclude-respond-permission",
		Query:        "Respond to the permission prompt the agent sent me — allow it",
		ExcludedTool: "api_v1_ChatService_RespondToPermission",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion", "hard"},
	},
	{
		ID:           "hard-exclude-tool-call-notification",
		Query:        "Notify the page subscribers that the agent just called a tool",
		ExcludedTool: "api_v1_ChatService_SendToolCallNotification",
		Services:     []string{"ChatService"},
		Tags:         []string{"chat", "exclusion", "hard"},
	},
	{
		ID:           "hard-exclude-scheduled-turn",
		Query:        "Report back to the wiki server that the scheduled turn finished successfully",
		ExcludedTool: "api_v1_ScheduledTurnService_CompleteScheduledTurn",
		Services:     []string{"ScheduledTurnService"},
		Tags:         []string{"exclusion", "hard"},
	},
}

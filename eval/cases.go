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
}

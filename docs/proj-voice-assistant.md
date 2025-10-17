# Voice Assistant Integration for Private Wiki

## Executive Summary

Enable Google Assistant (Gemini Nano) to query simple_wiki via voice commands with comprehensive, contextual responses. This AI-native integration leverages on-device LLM capabilities by providing complete rendered page content (frontmatter + rendered HTML with all templating applied) rather than search snippets, eliminating information bottlenecks and enabling the LLM to answer detailed questions including those that depend on template-expanded content.

**Key Design Principles:**

- **Android-First**: Start with Android to validate concept without app store requirements
- **Two-Phase Retrieval**: Search identifies pages → Fetch template-rendered markdown + frontmatter → LLM reasons over complete context
- **Token-Efficient Format**: Return **template-rendered markdown** (templates applied, but in markdown format not HTML) + JSON frontmatter to maximize context window usage while preserving template-expanded content
- **Cloud or Device LLM**: Let Gemini decide (Nano on-device or cloud-based) based on device capabilities and query complexity
- **Context Window Management**: Explicit token budgeting and page limits based on empirically measured constraints
- **Test-Driven Development**: Comprehensive TDD throughout all implementation phases, including LLM response validation
- **Structured Error Handling**: Explicit error types for LLM interpretation
- **API Extension Required**: ReadPageResponse needs new `rendered_content_markdown` field for template-expanded markdown

**Example Flow:**

```
User: "Hey Google, what's the warranty on my Space Navigator?"
  ↓ Gemini extracts: search for Space Navigator + warranty info needed
  ↓ Phase 1: SearchContent → finds Space Navigator page
  ↓ Phase 2: ReadPage → fetches markdown + frontmatter + structured template data
  ↓ Token budget check: Does content fit in ~4K-8K context window?
  ↓ If yes: Gemini reasons over complete context (on-device or cloud)
  ↓ If no: Platform routes to cloud for larger context window
  ↓ Gemini reasons: Warranty is "2 years, expires June 2025"
  ↓ "Your Space Navigator has a 2-year warranty that expires in June 2025"
```

**Token Efficiency Matters**: We design for a **4K-8K token context window** to optimize for on-device inference (privacy, latency, battery). Using markdown instead of HTML could mean the difference between fitting 1 page vs 3 pages in context. Cloud fallback handles edge cases requiring more context. This approach maximizes the percentage of queries that can run on-device.

## Quick Reference: APIs & Technologies

### Backend APIs (gRPC)

- **SearchContent** - Already exists, returns page identifiers
- **ReadPage** - Needs extension to add `rendered_content_markdown` field
  - Current: `content_markdown`, `front_matter_toml`, `rendered_content_html`
  - **ADD**: `rendered_content_markdown` (templates applied, markdown format)

### Android Platform APIs

- **AICore System Service** - Access to Gemini (on-device Nano or cloud)
  - Package: `com.google.android.aicore`
  - Requires: Android 14+, Pixel 8/8 Pro/8a (or future devices)
  
- **App Actions** - Voice command integration
  - Config: `res/xml/shortcuts.xml`
  - Built-in Intents (BIIs) for standard actions
  - Custom capabilities for domain-specific actions
  
- **Gemini Function Calling** - Tool integration
  - Define: `FunctionDeclaration` for wiki search capability
  - Handle: `FunctionCallPart` responses from LLM
  - Return: Results to LLM for final answer generation

### Bridge Technology

- **Capacitor** (MIT license)
  - Package: `@capacitor/core`, `@capacitor/android`
  - Plugin: Custom `WikiSearchPlugin` to expose native functionality to web
  - Bridge: Web ↔ Native via `@CapacitorPlugin` annotations

### Implementation Files

```
Android App Structure:
├── res/xml/shortcuts.xml              (App Actions declarations)
├── MainActivity.kt                    (Capacitor bridge setup)
├── WikiSearchPlugin.kt                (Native voice handler)
└── web/                               (Existing PWA, reused)

Backend Extension:
└── api/proto/api/v1/
    └── page_management.proto          (Add rendered_content_markdown)
```

### Key Dependencies

- `com.google.android.aicore:aicore` - Gemini access
- `@capacitor/core` v6+ - Native bridge
- Existing: gRPC client (already in project)

---

## Feasibility Research Findings (Updated 2025-10-13)

**DESIGN PHILOSOPHY - OPTIMIZE FOR DEVICE, ALLOW CLOUD FALLBACK:**

We design for **device-optimized context windows** (~4K-8K tokens) to enable:

- **On-device inference** when available (privacy, latency, battery efficiency)
- **Automatic cloud fallback** when needed (device lacks hardware or battery, query too complex)
- **Token efficiency** benefits all execution modes (faster network, lower costs, better battery life)

**Context Window Strategy:**

Primary design target: **4K-8K tokens** (on-device Gemini Nano capability)

- Fits 2-4 wiki pages with template-rendered markdown
- Enables fast, private, battery-efficient responses
- Works on Pixel 8+ devices with AICore enabled

Cloud fallback available: **Up to 1M+ tokens** (cloud-based Gemini models)

- Platform automatically routes based on device capability, battery state, network availability
- Allows handling complex queries requiring more context
- Not the primary design target, but useful fallback capability

**Why design for smaller context?**

1. **Device optimization** - Most queries answerable with 1-3 pages
2. **Battery efficiency** - On-device inference uses less power than network calls
3. **Privacy** - On-device keeps wiki data local
4. **Latency** - Device inference faster than cloud round-trip
5. **Universal benefit** - Token efficiency helps cloud mode too (faster, cheaper)

**KEY INSIGHT - TEMPLATE-RENDERED MARKDOWN:**

The wiki uses **Go templates as a primary extension mechanism** to generate dynamic content from structured data. Templates embedded in markdown expand to additional markdown content before HTML rendering. This is a core feature of the wiki, not just presentation logic.

**Why This Matters for LLM Integration:**

The critical innovation is **template-rendered markdown** - a new API field that provides the LLM with the same complete view humans see:

1. **Templates Generate Semantic Content** - Functions like `ShowInventoryContentsOf` create lists, links, and structured content that don't exist in raw markdown. This content is essential for answering questions about inventory, relationships, and computed data.

2. **Human-LLM Parity** - Humans see fully-rendered pages with all template expansions. The LLM must have the same complete view to answer questions accurately about template-generated content.

3. **Token Efficiency** - Markdown format is ~40-60% more compact than HTML while preserving all semantic information (47% token savings measured).

4. **Avoids Template Syntax Confusion** - Explaining Go template syntax (`{{or .Title .Identifier}}`, `{{ShowInventoryContentsOf .Identifier}}`) to the LLM adds complexity and risks parsing errors. Better to provide the expanded output.

5. **Frontmatter Stays Structured** - TOML frontmatter is provided separately as JSON, giving the LLM both structured metadata AND rendered prose.

**Example - Template Expansion:**

Raw markdown with templates (what's stored):

```markdown
# {{or .Title .Identifier}}
### Goes in: {{LinkTo .Inventory.Container}}

{{ ShowInventoryContentsOf .Identifier }}
```

Template-rendered markdown (what LLM receives):

```markdown
# Wall Bin F1
### Goes in: [Lab Wall Bins](/lab_wall_bins)

## Contents
- [5mm Barrel Plug to 10a 12v Socket](/item1)
- [12AWG inline blade fuse pigtails](/item2)
- [RCA Patch Cable](/item3)
```

The `ShowInventoryContentsOf` function dynamically queries the wiki's inventory index to generate the contents list. This information doesn't exist in the raw markdown—it's computed from frontmatter relationships across multiple pages. Without template expansion, the LLM cannot answer "what's in bin F1?"

**Data Flow:**

```
Raw markdown + Frontmatter → Template expansion → Rendered markdown + Frontmatter (JSON) → LLM
```

This solves the debate between "complete context" vs "token efficiency" - we get both.

**REQUIRED CHANGES:**

1. **API Extension**: Add `rendered_content_markdown` field to ReadPageResponse
   - Backend: Capture intermediate template-rendered markdown before HTML conversion
   - Current pipeline: `markdown with templates → template expansion → markdown → HTML`
   - API Extension: Expose the template-expanded markdown stage via new field
   - Estimated effort: 1-2 hours (templating system already does this internally)

2. **Data Format**: Use template-rendered markdown + JSON frontmatter
   - 47% token savings vs HTML
   - 100% semantic completeness vs raw markdown (includes all template-generated content)

3. **Cloud/Device Agnostic**: App Actions work with both on-device and cloud Gemini
   - Platform handles routing automatically
   - No code changes needed for hybrid approach

**CONSTRAINTS CLARIFIED:**

- ✅ **Primary design target: 4K-8K token context** (on-device optimization)
- ✅ **Cloud fallback available** (automatic, platform-managed)
- ✅ Token efficiency critical (benefits all execution modes)
- ✅ Template content required (inventory lists must be visible to LLM)
- ✅ Grounding required (prevent hallucination)
- ✅ Tailscale-only networking (no public API exposure)

**STATUS**: ✅ **FEASIBLE WITH API EXTENSION** - No empirical validation blocker, just needs new proto field

---

## Overview

Enable Google Assistant and Siri to query simple_wiki's existing gRPC APIs via voice commands and speak responses back to users. This integration leverages modern on-device LLM capabilities (Apple Intelligence and Gemini Nano) by exposing wiki search as a tool/capability that the assistant's AI agent can invoke and reason about.

**Core Flow**:

```
"Hey Google, where are my CR2032 batteries?"
    ↓
Gemini (cloud or on-device based on device capability)
    ↓
Invokes: SearchWikiAction with query "CR2032 batteries"
    ↓
App Action handler → Two-phase retrieval:
    1. SearchContent gRPC API → get matching page identifiers
    2. ReadPage gRPC API (per result, limited by token budget) → get template-rendered markdown + frontmatter
    ↓
Returns token-efficient context with template-expanded content:
    JSON frontmatter + template-rendered markdown (inventory lists included)
    ↓
Gemini reasons over complete page content
    ↓
"Your CR2032 batteries are in Lab Desk Wall Bin C1" (spoken)
```

**Note on Cloud vs Device**: We're not forcing on-device Gemini Nano. The platform (Google Assistant/Gemini) will automatically choose between on-device Nano or cloud-based models based on query complexity, device capability, and network availability. Our job is just to provide token-efficient data.

**Integration Architecture**: The app exposes wiki search as a **tool/capability** to the platform's native AI agent (Gemini on Android, which can use Nano on-device or cloud-based models). The agent handles natural language understanding, parameter extraction, and response generation. Our app provides the data access layer.

**Key Principle - Template-Rendered Markdown**: Return **template-rendered markdown** (templates applied, but still in markdown format, not HTML) + JSON frontmatter.

**Why Template-Rendered Markdown is Required:**

The wiki uses **Go templates as a core extension mechanism**. Templates embedded in markdown generate dynamic content from structured data:

- `{{or .Title .Identifier}}` - Fallback logic for titles
- `{{LinkTo .Inventory.Container}}` - Generate wiki links from frontmatter relationships  
- `{{ShowInventoryContentsOf .Identifier}}` - Query inventory index and generate markdown lists
- `{{if IsContainer .Identifier}}...{{end}}` - Conditional rendering based on computed state

**These templates create semantic content that doesn't exist in raw markdown.** Without template expansion, the LLM cannot see inventory lists, computed relationships, or dynamic links—content that humans see when viewing the same page.

**Example:**

Raw markdown (stored in wiki):

```markdown
# {{or .Title .Identifier}}
### Goes in: {{LinkTo .Inventory.Container}}

{{ ShowInventoryContentsOf .Identifier }}
```

Template-rendered markdown (sent to LLM):

```markdown
# Wall Bin F1
### Goes in: [Lab Wall Bins](/lab_wall_bins)

## Contents
- [5mm Barrel Plug to 10a 12v Socket](/item1)
- [12AWG inline blade fuse pigtails](/item2)
- [RCA Patch Cable](/item3)
```

The `ShowInventoryContentsOf` function dynamically queries the wiki's inventory index (pages with `inventory.container = 'lab_wallbins_f1'`) to generate the contents list. This information doesn't exist in the raw markdown—it's computed from frontmatter relationships across multiple pages.

**Benefits:**

1. **Human-LLM parity**: Humans see template-expanded content; LLM must see the same to answer accurately
2. **Semantic completeness**: Template-generated lists/links are essential content, not just presentation
3. **Token efficiency**: Markdown is ~47% more compact than HTML while preserving all semantic meaning
4. **Avoid complexity**: Don't explain Go template syntax to LLM; just provide expanded output
5. **Structured + prose**: Frontmatter (JSON) provides metadata; rendered markdown provides context

**API Extension Needed**: The ReadPageResponse proto currently provides:

- `content_markdown` (raw markdown, no templates applied)
- `rendered_content_html` (templates applied, converted to HTML)

We need to add:

- `rendered_content_markdown` (templates applied, but still markdown) ← **NEW FIELD**

This gives the LLM access to template-expanded content (like inventory lists that don't exist in raw markdown) while maintaining token efficiency.

**Implementation Vehicle**: Capacitor provides the minimal native wrapper needed to register platform-specific capabilities (iOS App Intents, Android App Actions) while preserving the existing web-first architecture.

**Distribution Model**: Sideloaded APK installation only. No app store distribution due to lack of authentication on APIs—this is a private, Tailscale-only application. Android was chosen for initial implementation because it allows sideloading without registration with a central authority, enabling faster iteration and testing.

**Target Platform**: Android with Gemini Nano support (initial implementation). iOS support may be added later once the concept is validated.

**Hardware Requirements**:

- Google Pixel 8 Pro, Pixel 8, or Pixel 8a
- Android 14+ with AICore system service
- Developer options enabled to activate Gemini Nano
- Tailscale network connectivity for API access

**Technical Stack**:

- **Android AICore API**: Provides access to on-device Gemini Nano model
- **App Actions + Built-in Intents (BIIs)**: Enables voice command integration via Google Assistant
- **Function Calling**: Allows Gemini to invoke wiki search as a tool
- **Capacitor**: MIT-licensed native bridge for web-to-native communication
- **gRPC**: Existing wiki APIs (SearchContent, ReadPage) over Tailscale

**Context**: This builds on the PWA implementation (see `proj-pwa.md`). The PWA provides the visual interface; this project adds voice-driven, eyes-free access to the same underlying data over your private Tailnet.

**License**: Capacitor is MIT licensed open source software by Ionic. All tooling will be managed via devbox for consistency with the project's development environment approach.

## Goals

1. **Natural Voice Interaction**: User asks in natural language ("where are my batteries?") → Assistant understands intent and speaks contextual answer
2. **LLM-Enhanced Tool Integration**: Expose wiki search as a capability to Gemini Nano that can reason about when and how to use it
3. **Complete Context with Token Efficiency**: Return markdown + JSON + structured data for search results, maximizing pages-per-context-window while maintaining semantic completeness
4. **Hands-Free Access**: Query wiki content without opening the app or looking at a screen  
5. **Leverage Existing APIs**: Reuse simple_wiki's gRPC infrastructure (SearchContent + ReadPage); no backend rewrite needed
6. **Two-Phase Retrieval with Budget Management**: Search identifies relevant pages, then fetch content for top N results (limited by token budget)
7. **Android-First Development**: Start with Android/Gemini Nano to validate the concept without requiring central authority registration
8. **Comprehensive Test Coverage**: All development follows TDD principles with unit, integration, and LLM response validation tests
9. **Tailscale-Only Operation**: All API calls must route through Tailscale; no public internet exposure
10. **Empirical Validation**: Measure actual context limits and token usage before finalizing implementation

## Required API Extensions

### ReadPageResponse: Add Template-Rendered Markdown

**Current API** (`api/proto/api/v1/page_management.proto`):

```protobuf
message ReadPageResponse {
  string content_markdown = 1;        // Raw markdown, no templates
  string front_matter_toml = 2;       // TOML frontmatter
  string rendered_content_html = 3;   // Templates + HTML conversion
}
```

**Proposed Addition**:

```protobuf
message ReadPageResponse {
  string content_markdown = 1;              // Raw markdown, no templates
  string front_matter_toml = 2;             // TOML frontmatter
  string rendered_content_html = 3;         // Templates + HTML conversion
  string rendered_content_markdown = 4;     // Templates applied, markdown format ← NEW
}
```

**Rationale**:

- `content_markdown`: Doesn't include template-expanded content (e.g., inventory lists)
- `rendered_content_html`: Includes template-expanded content BUT 2-3x token overhead
- `rendered_content_markdown`: **Best of both** - template-expanded content in token-efficient format

**Implementation Notes**:

1. The templating system already applies templates to generate HTML
2. We need to capture the intermediate step: templates applied to markdown, BEFORE HTML conversion
3. This is the markdown with inventory lists, related pages, etc. already expanded
4. Then pass this to markdown-to-HTML converter separately for HTML response

**Example**:

Raw markdown (`content_markdown`):

```markdown
# Lab Desk Wall Bin C1

Inventory container for small electronics.
```

Template-rendered markdown (`rendered_content_markdown` - NEW):

```markdown
# Lab Desk Wall Bin C1

Inventory container for small electronics.

## Contents
- [CR2032 Batteries](/cr2032_batteries)
- [CR2025 Batteries](/cr2025_batteries)
- [LED Assortment](/led_assortment)
```

Rendered HTML (`rendered_content_html`):

```html
<h1>Lab Desk Wall Bin C1</h1>
<p>Inventory container for small electronics.</p>
<h2>Contents</h2>
<ul>
<li><a href="/cr2032_batteries">CR2032 Batteries</a></li>
<li><a href="/cr2025_batteries">CR2025 Batteries</a></li>
<li><a href="/led_assortment">LED Assortment</a></li>
</ul>
```

**Token Comparison**:

- Raw markdown: ~50 tokens (missing inventory!)
- Template-rendered markdown: ~85 tokens (complete, efficient)
- Rendered HTML: ~160 tokens (complete, inefficient)

**Savings**: 47% fewer tokens than HTML while maintaining complete semantic content.

## Android Integration Architecture

### Android AICore and Gemini Nano

**AICore System Service**:

- Provides on-device access to Gemini Nano LLM
- Runs locally without network connectivity
- Low latency, high privacy
- Available on Pixel 8 Pro, Pixel 8, Pixel 8a (via developer options)

**Activation Steps**:

1. Enable Developer Options (tap build number 7 times)
2. Navigate to Settings → System → Developer options
3. Enable AICore settings
4. Download Gemini Nano model (~1.5GB)

### App Actions and Built-in Intents

**App Actions** enable voice-triggered functionality via Google Assistant by declaring capabilities in `shortcuts.xml`:

```xml
<shortcuts>
  <capability android:name="actions.intent.SEARCH_WIKI">
    <intent
      android:action="android.intent.action.VIEW"
      android:targetPackage="com.example.simplewiki"
      android:targetClass="com.example.simplewiki.MainActivity">
      <parameter
        android:name="query"
        android:key="searchQuery" />
    </intent>
  </capability>
</shortcuts>
```

**Built-in Intents (BIIs)**:

- Standard Android intent patterns recognized by Google Assistant
- Examples: `actions.intent.GET_THING`, `actions.intent.SEARCH`
- Custom capabilities can be defined for domain-specific actions
- Parameters extracted from user's natural language query

### Function Calling / Tool Integration

Gemini Nano supports **function calling**, allowing the LLM to invoke external functions/APIs:

**Define Function Declaration**:

```kotlin
val searchWikiFunction = FunctionDeclaration(
    name = "search_wiki",
    description = "Search the personal wiki for information about items, locations, and inventory",
    parameters = jsonObject {
        "type" to "object"
        "properties" to jsonObject {
            "query" to jsonObject {
                "type" to "string"
                "description" to "The search query extracted from user's question"
            }
        }
        "required" to jsonArray("query")
    }
)
```

**Configure Gemini Client**:

```kotlin
val tools = listOf(Tool(functionDeclarations = listOf(searchWikiFunction)))
val config = GenerateContentConfig(tools = tools)

val response = geminiClient.generateContent(
    model = "gemini-nano",
    prompt = userQuery,
    config = config
)
```

**Handle Function Call**:

```kotlin
when (val part = response.candidates[0].content.parts[0]) {
    is FunctionCallPart -> {
        val functionName = part.functionCall.name
        val args = part.functionCall.args
        
        // Execute wiki search
        val results = searchWiki(args["query"] as String)
        
        // Return results to LLM for final response generation
        val finalResponse = geminiClient.generateContent(
            model = "gemini-nano",
            prompt = FunctionResponsePart(functionName, results),
            config = config
        )
    }
}
```

### Capacitor Native Bridge

**Purpose**: Enable web-based PWA to invoke native Android capabilities

**Plugin Structure**:

```kotlin
@CapacitorPlugin(name = "WikiSearch")
class WikiSearchPlugin : Plugin() {
    @PluginMethod
    fun searchAndRespond(call: PluginCall) {
        val query = call.getString("query") ?: return call.reject("Missing query")
        
        // Phase 1: Search
        val searchResults = grpcClient.searchContent(query)
        
        // Phase 2: Fetch pages (with token budget)
        val pages = fetchPagesWithinBudget(searchResults, maxTokens = 4096)
        
        // Phase 3: LLM reasoning
        val response = geminiClient.generateResponse(query, pages)
        
        call.resolve(JSObject().put("response", response))
    }
}
```

**Registration** in `MainActivity.kt`:

```kotlin
class MainActivity : BridgeActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        registerPlugin(WikiSearchPlugin::class.java)
        super.onCreate(savedInstanceState)
    }
}
```

### Token Budget Management

**Design Target**: 4K-8K token context window (on-device Gemini Nano optimization)

**Why this target?**

- Optimizes for on-device inference (privacy, latency, battery)
- Fits 2-4 typical wiki pages with template-rendered markdown
- Platform automatically falls back to cloud if query exceeds budget
- Token efficiency benefits all modes (device and cloud)

**Token Counting Strategy**:

```kotlin
fun estimateTokenCount(text: String): Int {
    // Rough estimate: 1 token ≈ 4 characters for English text
    // More accurate: Use Gemini's token counting API if available
    return (text.length / 4.0).toInt()
}

fun fetchPagesWithinBudget(searchResults: List<SearchResult>, maxTokens: Int): List<Page> {
    val systemPromptTokens = 200 // Reserve for system instructions
    val responseTokens = 500     // Reserve for LLM response
    val availableTokens = maxTokens - systemPromptTokens - responseTokens
    
    val pages = mutableListOf<Page>()
    var usedTokens = 0
    
    for (result in searchResults) {
        val page = grpcClient.readPage(result.identifier)
        val pageTokens = estimateTokenCount(page.markdown + page.frontmatter)
        
        if (usedTokens + pageTokens > availableTokens) {
            break // Stop adding pages
        }
        
        pages.add(page)
        usedTokens += pageTokens
    }
    
    return pages
}
```

**Data Format Strategy**:

```kotlin
data class WikiContext(
    val pages: List<PageContext>
)

data class PageContext(
    val identifier: String,
    val title: String,
    val frontmatter: Map<String, Any>,  // Structured JSON, not TOML string
    val markdown: String,                // Raw markdown, NOT HTML
    val templateData: TemplateData?      // Structured template-expanded content
)

data class TemplateData(
    val inventoryItems: List<InventoryItem>? = null,
    val relatedPages: List<RelatedPage>? = null
)
```

**Why This Format**:

- JSON frontmatter: Structured, easy for LLM to parse
- Markdown body: 2-3x more token-efficient than HTML
- Separate templateData: Template-expanded content in structured format
- Total token savings: 50-70% compared to rendered HTML

### System Prompt Design

**Critical for Grounding**:

```kotlin
val systemPrompt = """
You are a helpful assistant that answers questions about a personal wiki.
You have access to wiki pages retrieved by searching.

IMPORTANT RULES:
1. Only answer based on the provided wiki pages
2. If the information is not in the pages, say "I don't have that information in the wiki"
3. Cite which page contains the answer when possible
4. Do not make up or infer information not explicitly stated

The user asked: "$userQuery"

Here are the relevant wiki pages:
$wikiContext
""".trimIndent()
```

## Required Implementation Steps

**Phase 0: API Extension** (Required first)

### Add Template-Rendered Markdown to ReadPageResponse

**Step 1**: Update proto definition

```protobuf
// In api/proto/api/v1/page_management.proto
message ReadPageResponse {
  string content_markdown = 1;
  string front_matter_toml = 2;
  string rendered_content_html = 3;
  string rendered_content_markdown = 4;  // NEW
}
```

**Step 2**: Update backend to capture template-rendered markdown

- The templating system already generates this internally
- Just need to return it before HTML conversion
- Add field to response builder

**Step 3**: Test with existing pages

- Verify inventory lists appear in rendered markdown
- Compare token counts: rendered_markdown vs rendered_html
- Confirm 40-50% token savings

**Estimated effort**: 2-4 hours

---

**Phase 1: Validation Testing** (Can proceed after API extension)

### Test 1: End-to-End Voice Query

```kotlin
// Objective: Validate full flow works
fun testVoiceQuery() {
    // Manual test: Say "Hey Google, where are my CR2032 batteries?"
    // Expected: Assistant responds with location
    // Verify: Check logs for search query, page fetch, LLM response
}
```

### Test 2: Template Content Accessibility

```kotlin
// Objective: Confirm LLM can read template-expanded content
fun testTemplateContent() {
    val query = "What's in bin C1?"
    val page = grpcClient.readPage("lab_desk_wall_bin_c1")
    
    // Use the NEW rendered_content_markdown field
    val context = """
    Page: ${page.title}
    
    ${page.renderedContentMarkdown}  // Has inventory list
    """.trimIndent()
    
    val response = geminiClient.generateContent(
        prompt = context,
        query = query
    )
    
    // Expected: Lists CR2032, CR2025, LED Assortment
    println("Response: ${response.text}")
}
```

**Success Criteria**:

- LLM sees and uses template-expanded content
- Token efficiency verified (markdown vs HTML)
- End-to-end voice query works
- No empirical context window testing needed (platform handles it)

### Validation Checklist

- [ ] API extension implemented and deployed
- [ ] Template-rendered markdown includes inventory lists
- [ ] Token savings confirmed (40-50% vs HTML)
- [ ] End-to-end voice query successful  
- [ ] LLM can extract info from template content
- [ ] Grounding prevents hallucination
- [ ] Response quality acceptable
- [ ] Latency acceptable (<5 seconds)

**DECISION GATE**: Much simpler than before - just validate API extension works correctly.

## Non-Goals

- **App store distribution**: APIs lack authentication, must remain private via Tailscale
- **Custom LLM infrastructure**: Leverage Gemini Nano rather than building our own
- **Pre-formatted spoken responses**: App returns structured data; let the assistant's LLM generate natural language
- **Offline-first architecture**: Network-dependent over Tailscale (though Gemini Nano is on-device)
- **Complex native UI**: Web components remain primary interface; this is about voice
- **Replacing the PWA**: Capacitor app and PWA coexist; PWA for visual, native for voice
- **Rewriting the backend**: gRPC APIs already exist (SearchContent + ReadPage provide everything needed)
- **Public API exposure**: Must stay within Tailnet
- **iOS support initially**: Focus on Android to prove concept; iOS can be added later if validated
- **Semantic search**: Current text-based search is sufficient for initial implementation; embeddings-based search is a future enhancement

## Prerequisites

**Must be completed first:**

- PWA implementation (Phase 1 from `proj-pwa.md`)
- gRPC APIs functional and accessible over Tailscale
- PWA tested and validated on mobile browsers
- Device(s) connected to the Tailscale network

**Infrastructure Requirements:**

- Active Tailscale network with wiki server accessible
- Mobile device(s) on the same Tailnet
- gRPC endpoints returning structured, queryable data

## LLM Integration Architecture

### Apple Intelligence (iOS 18+)

**How App Intents Integrate with On-Device LLM:**

```swift
import AppIntents

struct SearchWikiIntent: AppIntent {
    static var title: LocalizedStringResource = "Search Wiki"
    static var description = IntentDescription("Search your private wiki for information")
    
    @Parameter(title: "Search Query", description: "What to search for")
    var query: String
    
    static var parameterSummary: some ParameterSummary {
        Summary("Search wiki for \(\.$query)")
    }
    
    func perform() async throws -> some IntentResult & ReturnsValue<SearchResult> {
        // Call wiki gRPC API over Tailscale
        let apiResponse = try await WikiAPIClient.shared.search(query: query)
        
        // Return structured context - let Apple Intelligence format the response
        let result = SearchResult(
            title: apiResponse.results.first?.title ?? "No results",
            content: apiResponse.results.first?.fragment ?? "",
            location: extractLocationIfPresent(apiResponse),
            identifier: apiResponse.results.first?.identifier
        )
        
        return .result(value: result)
    }
}

// Structured result that Apple Intelligence can reason about
struct SearchResult: Codable {
    let title: String
    let content: String
    let location: String?
    let identifier: String?
}
```

**How Apple Intelligence Uses This:**

1. **Natural Language Understanding**: User says anything wiki-related ("where's my multimeter?", "find batteries", "what's in bin C1?")
2. **Intent Matching**: Apple Intelligence recognizes this matches SearchWikiIntent capability
3. **Parameter Extraction**: LLM extracts query = "multimeter" from natural speech
4. **Intent Invocation**: Calls your perform() method
5. **Response Reasoning**: LLM receives SearchResult, reasons about what user asked, generates natural response

**Key Points**:

- App just needs to register the intent and return structured data
- All NLU, parameter extraction, and response generation handled by Apple Intelligence
- On-device processing preserves privacy
- User doesn't need to memorize specific commands

### Gemini Nano (Android)

**How App Actions Integrate with On-Device Gemini:**

```xml
<!-- res/xml/actions.xml -->
<actions>
    <action intentName="actions.intent.SEARCH">
        <fulfillment urlTemplate="app://search?query={query}">
            <parameter name="query">
                <entity-set-reference entitySetId="SearchQuery"/>
            </parameter>
        </fulfillment>
    </action>
</actions>
```

```kotlin
// Intent handler
class SearchWikiActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        
        handleVoiceIntent(intent)
    }
    
    private fun handleVoiceIntent(intent: Intent) {
        // Gemini Nano extracts query from natural language
        val query = intent.getStringExtra("query") ?: return
        
        lifecycleScope.launch {
            val result = wikiApiClient.search(query)
            
            // Return structured response for Gemini to interpret
            val response = SearchResponse(
                title = result.results.firstOrNull()?.title,
                content = result.results.firstOrNull()?.fragment,
                success = result.results.isNotEmpty()
            )
            
            // Gemini formats this into natural language
            setResult(RESULT_OK, Intent().apply {
                putExtra("response", response.toJson())
            })
        }
    }
}
```

**How Gemini Nano Uses This:**

1. **Natural Language Understanding**: Enhanced NLU for understanding search intent
2. **Parameter Extraction**: Extracts query from various phrasings
3. **Action Invocation**: Routes to your app action
4. **Response Generation**: Gemini interprets your structured response and speaks naturally

**Platform Comparison** (Future iOS Support):

When iOS support is added later, the same two-phase retrieval pattern will be used with Apple Intelligence and App Intents. The core architecture (search → fetch full pages → LLM reasoning) remains platform-agnostic.

| Feature | Gemini Nano (Current) | Apple Intelligence (Future) |
|---------|----------------------|----------------------------|
| On-device LLM | ✓ (when available) | ✓ Full |
| Privacy | Full (when using Nano) | Full |
| Fallback | Cloud Gemini | Standard Siri |
| API Maturity | Evolving | Stable (iOS 18+) |
| Implementation Priority | Primary (Phase 1) | Deferred (Phase 2+) |
| Intent Definition | Swift App Intents | XML + Kotlin |
| Response Format | Structured types | JSON in Intent extras |

### Why This Architecture Matters

**Traditional App Actions/Intents** (Pre-LLM):

```
User: "Search wiki for batteries"
  ↓ [Rigid slot filling]
Action: search(query="batteries")
  ↓ [App formats response]
Response: "CR2032 Batteries go in Lab Desk Wall Bin C1"
```

**LLM-Enhanced App Actions** (Modern with Full Context):

```
User: "Hey, where did I put those coin cell batteries?"
  ↓ [LLM reasoning: user wants location of batteries]
Action: SearchWiki(query="coin cell batteries")
  ↓ [Two-phase retrieval]
Phase 1: SearchContent API → finds "CR2032 Batteries" page
Phase 2: ReadPage API → fetches rendered HTML + frontmatter
  ↓ [App returns complete context]
Data: {
  frontmatter: "location = 'Lab Desk Wall Bin C1'\ncategory = 'Electronics'",
  renderedHtml: "<h1>CR2032 Batteries</h1><p>Coin cell batteries...</p>...[full rendered content with templating applied]..."
}
  ↓ [LLM reasons over complete rendered content]
Response: "I found your CR2032 coin cell batteries in Lab Desk Wall Bin C1"
```

The app becomes a **data provider to an intelligent agent**, not a speech formatter. By providing complete rendered HTML (with all templating logic applied) instead of fragments, the LLM can answer questions that require information beyond search snippets or that depends on template-expanded content like inventory relationships.

### Existing Infrastructure

Simple_wiki already has gRPC APIs that provide everything needed:

1. **SearchService.SearchContent**: Finds relevant pages, returns identifiers and snippets
   - Input: search query string
   - Output: Array of `SearchResult` (identifier, title, fragment, highlights)

2. **PageManagementService.ReadPage**: Retrieves complete page content
   - Input: page identifier (page_name)
   - Output: `ReadPageResponse` containing:
     - `content_markdown`: Raw markdown source (not used for voice)
     - `front_matter_toml`: Structured metadata (location, tags, categories, etc.)
     - `rendered_content_html`: **Fully rendered HTML with all templating logic applied** (inventory expansion, cross-references, dynamic content)

**Two-Phase Retrieval Pattern**:

1. Use `SearchContent` to identify relevant pages (fast, returns identifiers)
2. Call `ReadPage` for top N results to get rendered HTML + frontmatter (complete processed context for LLM)

**Design Principle for LLM Integration**: Return complete rendered HTML (with templating applied) + frontmatter rather than search snippets or raw markdown. The templating system processes inventory relationships, cross-references, and dynamic transformations. This eliminates the information bottleneck and enables the LLM to reason over full semantic context, answering questions even when the answer depends on template-expanded content.

### Voice Integration Flow

```
┌─────────────────────────────────────────────────────────────┐
│  User: "Hey Google, where did I put my CR2032 batteries?"  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Gemini Nano (On-Device LLM)                               │
│  - Understands: User wants location of CR2032 batteries    │
│  - Reasons: This is a wiki search task                     │
│  - Decides: Invoke SearchWikiAction capability             │
│  - Extracts parameter: query = "CR2032 batteries"          │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  App Action Handler (in Capacitor app)                      │
│  - Receives: query parameter from Gemini Nano              │
│  - No rigid parsing needed - LLM did semantic extraction   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Phase 1: Search API Call over Tailscale                   │
│  - SearchService.SearchContent(query: "CR2032 batteries")  │
│  - Returns: SearchContentResponse with results[]           │
│    - result[0].identifier = "cr2032_batteries"             │
│    - result[0].title = "CR2032 Batteries"                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Phase 2: Fetch Full Content (Top 3 Results)               │
│  - PageManagementService.ReadPage("cr2032_batteries")      │
│  - Returns: ReadPageResponse with:                         │
│    - front_matter_toml: Structured metadata                │
│    - rendered_content_html: Fully rendered with templating │
│  - Parallel fetches for top N results                      │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Intent Handler Returns Complete Context                   │
│  - Frontmatter: "location = 'Lab Desk Wall Bin C1'..."     │
│  - Rendered HTML: "<h1>CR2032...</h1><p>Coin cell...</p>"  │
│  - NO pre-formatted speech - return rich, complete data    │
│  - Multiple pages if multiple results found                │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Gemini Nano (LLM Response Generation)                     │
│  - Input: Complete rendered HTML + frontmatter             │
│  - Reasoning: User asked "where", extract location from    │
│              frontmatter or HTML content                   │
│  - Generates: Natural response matching query style        │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Siri/Assistant: "Your CR2032 batteries are in Lab Desk    │
│                   Wall Bin C1"                              │
└─────────────────────────────────────────────────────────────┘
```

**Key Architectural Difference from Traditional App Actions/Intents:**

Traditional approach: App does all the work (parse query → call API → format speech → return string)

LLM-enhanced approach: Platform LLM handles reasoning on both ends:

1. **Input side**: Natural language → structured intent invocation
2. **Output side**: Structured data → contextual natural language

The app is a **thin data access layer** between two LLM reasoning steps.

### Response Transformation

**Philosophy Shift**: With LLM-enhanced assistants, the app's responsibility changes from "format speech" to "provide rich context."

**Old Approach (Pre-LLM Assistants)**:

- App must parse fragments, extract location, format exact speech
- Brittle string manipulation ("Goes in:" pattern matching)
- One-size-fits-all response regardless of user's actual question

**New Approach (LLM-Enhanced Assistants)**:

- App returns structured, semantic data
- Platform LLM interprets and generates contextual response
- Response adapts to user's phrasing and intent

**API Response Structure**:

Current API returns:

```json
{
  "results": [
    {
      "identifier": "cr2032_batteries",
      "title": "CR2032 Batteries",
      "fragment": "CR2032 Batteries\n\n Goes in: Lab Desk Wall Bin C1"
    }
  ]
}
```

**Intent Handler Response Strategy**:

Instead of parsing and formatting, return structured context:

```typescript
// iOS App Intent
return .result(value: {
  title: result.title,
  content: result.fragment,  // Full context
  identifier: result.identifier
})
```

The platform LLM then reasons:

- User asked "where are my batteries?" → emphasize location from fragment
- User asked "what are CR2032 batteries?" → emphasize description from fragment
- User asked "do I have batteries?" → confirm existence and location

**Benefits of LLM Interpretation**:

1. **No fragile parsing**: Don't need to extract "Goes in:" patterns
2. **Contextual responses**: LLM adapts answer to user's specific question
3. **Graceful degradation**: If fragment format changes, LLM can still extract meaning
4. **Natural paraphrasing**: User gets conversational responses, not templated strings
6. **Fallback**: "I found [N] pages about [query]. The top result is [title]."

**Real Example Transformation**:

```
API Response for "batteries":
{
  "identifier": "cr2032_batteries",
  "title": "CR2032 Batteries",
  "fragment": "CR2032 Batteries\n\n Goes in: Lab Desk Wall Bin C1"
}

Speakable Response:
"CR2032 Batteries go in Lab Desk Wall Bin C1"

---

API Response for "space navigator":
{
  "identifier": "3dconnexion_space_navigator", 
  "title": "3Dconnexion Space Navigator",
  "fragment": "3Dconnexion Space Navigator\n\n Goes in: Lab Desk Wall Bin L1 (Big Yellow) \n\n Product Information\n\nProduct Name: SpaceNavigator \nManufacturer: 3Dconnexion\n\nFeatures\n\n3D navigation knob \nUSB connectivity"
}

Speakable Response:
"The 3Dconnexion Space Navigator is in Lab Desk Wall Bin L1. It's a 3D navigation knob with USB connectivity."
```

### gRPC Integration Details

**Available API Endpoint** (already implemented):

- `SearchService.SearchContent(query string) → SearchContentResponse`
  - Returns: Array of search results with `identifier`, `title`, `fragment`, `highlights`
  - Fragment: Plain text excerpt (~200 chars) from matching content
  - Results: Ranked by relevance, typically 10+ results

**API Response Structure**:

```protobuf
message SearchContentResponse {
  repeated SearchResult results = 1;
}

message SearchResult {
  string identifier = 1;  // Page identifier (e.g., "cr2032_batteries")
  string title = 2;        // Human-readable title (e.g., "CR2032 Batteries")
  string fragment = 3;     // Plain text excerpt with context
  repeated HighlightSpan highlights = 4;  // Match positions
}
```

**Voice Handler Implementation Pattern**:

```kotlin
// Android (Kotlin)
suspend fun handleVoiceSearch(query: String): String {
    val response = wikiClient.searchContent(SearchContentRequest(query = query))
    
    if (response.results.isEmpty()) {
        return "I couldn't find anything about $query in the wiki"
    }
    
    val topResult = response.results.first()
    return extractSpeakableAnswer(topResult)
}

fun extractSpeakableAnswer(result: SearchResult): String {
    val fragment = result.fragment
    
    // Extract "Goes in:" location pattern (common in inventory items)
    val locationMatch = Regex("Goes in: ([^\n]+)").find(fragment)
    if (locationMatch != null) {
        return "${result.title} goes in ${locationMatch.groupValues[1]}"
    }
    
    // Otherwise, use first 1-2 sentences from fragment
    val sentences = fragment.split(Regex("[.\\n]+"))
        .filter { it.isNotBlank() }
        .take(2)
        .joinToString(". ")
        .take(250) // ~50 words
    
    return sentences.ifEmpty { 
        "I found ${result.title} but couldn't extract a clear answer" 
    }
}
```

**Swift Implementation** (iOS):

```swift
func handleVoiceSearch(query: String) async throws -> String {
    let response = try await wikiClient.searchContent(query: query)
    
    guard let topResult = response.results.first else {
        return "I couldn't find anything about \(query) in the wiki"
    }
    
    return extractSpeakableAnswer(from: topResult)
}

func extractSpeakableAnswer(from result: SearchResult) -> String {
    let fragment = result.fragment
    
    // Extract location if present
    if let range = fragment.range(of: #"Goes in: ([^\n]+)"#, options: .regularExpression) {
        let location = String(fragment[range]).replacingOccurrences(of: "Goes in: ", with: "")
        return "\(result.title) goes in \(location)"
    }
    
    // Use first sentences
    let sentences = fragment.components(separatedBy: CharacterSet(charactersIn: ".\n"))
        .filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }
        .prefix(2)
        .joined(separator: ". ")
        .prefix(250)
    
    return String(sentences).isEmpty ? 
        "I found \(result.title) but couldn't extract a clear answer" : 
        String(sentences)
}
```

**Error Handling**:

- No results found → "I couldn't find anything about [query] in the wiki"
- Multiple results → Use top result (already ranked by relevance)
- Network error → "I couldn't reach the wiki. Check your Tailscale connection"
- Empty/malformed fragment → "I found [title] but couldn't extract a clear answer"

**Performance Considerations**:

- Search API is fast (< 500ms typical)
- Fragment is pre-extracted (no full page retrieval needed)
- No additional API calls required for basic queries
- Total latency target: query → spoken response < 2 seconds

## Why Capacitor Over Alternatives?

### The Core Requirement

Google Assistant and Siri require **platform-native code** to register voice handlers:

- **Android**: App Actions require native Android app with actions.xml manifest
- **iOS**: App Intents require Swift code conforming to AppIntent protocol

**No web-only solution exists** for deep voice assistant integration with spoken responses. PWAs can't register these handlers.

### Capacitor vs PWABuilder

- **Capacitor**: Full native API access, can implement App Intents/Actions with custom code
- **PWABuilder**: Simple wrapper but no access to platform voice APIs

### Capacitor vs Custom Native Apps

- **Custom Native**: Maximum control but requires full iOS (Swift) + Android (Kotlin) development
- **Capacitor**: Minimal native code (just voice handlers) + reuse entire web app and gRPC client logic

### Capacitor vs Bubblewrap (Google's TWA)

- **Bubblewrap**: Android-only, limited to web app features
- **Capacitor**: Cross-platform with native plugin system for voice integration

### Why Not Pure PWA?

PWAs **cannot**:

- Register App Actions (Android) or App Intents (iOS)
- Return spoken responses to voice assistants
- Access platform voice integration APIs

PWAs **can** (but insufficient for this use case):

- Handle URL schemes (requires manual user setup)
- Show visual results (but we want *spoken* answers)

**Conclusion**: Capacitor is the minimal viable approach to expose gRPC APIs to platform voice assistants while reusing the existing web architecture.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│   Native App Shell (Capacitor) - Sideloaded Installation    │
│   ┌──────────────────────────────────────────────────────┐   │
│   │   WebView (existing PWA)                             │   │
│   │   - Visual interface when opened directly            │   │
│   │   - Web components, JavaScript code                  │   │
│   │   - Fallback UI for complex responses                │   │
│   └──────────────────────────────────────────────────────┘   │
│                                                              │
│   Native Voice Integration Layer:                           │
│   ┌──────────────────────────────────────────────────────┐   │
│   │  App Actions (Android) / App Intents (iOS)           │   │
│   │  - Register voice command handlers                   │   │
│   │  - Parse user queries                                │   │
│   │  - Call gRPC APIs over Tailscale                     │   │
│   │  - Transform responses to speakable format           │   │
│   │  - Return spoken text to assistant                   │   │
│   └──────────────────────────────────────────────────────┘   │
│                                                              │
│   Network Layer:                                             │
│   - Tailscale VPN connection required                       │
│   - gRPC client communicating with wiki server              │
│   - No public internet API access                           │
└──────────────────────────────────────────────────────────────┘
                              ↓
                    (Over Tailscale Network)
                              ↓
┌──────────────────────────────────────────────────────────────┐
│   Simple Wiki Server (existing)                              │
│   - gRPC APIs                                                │
│   - Page search and retrieval                               │
│   - No authentication (protected by Tailscale)              │
└──────────────────────────────────────────────────────────────┘
```

## Technical Design

### Directory Structure

```
simple_wiki/
├── static/              # Existing web app (unchanged)
├── ios/                 # Generated iOS project (Xcode)
│   └── App/
│       └── AppIntents/  # Voice handler Swift code
├── android/             # Generated Android project (Android Studio)  
│   └── app/src/main/
│       ├── res/xml/actions.xml          # Voice action definitions
│       └── java/.../VoiceActionHandler  # Voice handler Kotlin code
├── capacitor.config.ts  # Capacitor configuration
├── package.json         # Dependencies (add Capacitor + gRPC client)
└── docs/
    ├── proj-pwa.md
    └── proj-voice-assistant.md
```

### Key Integration Points

#### 1. Capacitor Configuration (`capacitor.config.ts`)

```typescript
import { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'net.ts.monster-orfe.wiki', // Reverse domain notation
  appName: 'Simple Wiki',
  webDir: 'static', // Points to existing static assets
  server: {
    // Development: point to local Tailscale address
    url: 'https://wiki.monster-orfe.ts.net',
    cleartext: false
  },
  plugins: {
    SplashScreen: {
      launchShowDuration: 0 // Instant load
    }
  }
};

export default config;
```

**Note**: Production and development use the same Tailscale URL since there's no public deployment.

#### 2. Android App Actions Integration

**Purpose**: Register wiki search capability with Google Assistant. Gemini Nano (on-device LLM) handles natural language understanding and response generation.

**File**: `android/app/src/main/res/xml/actions.xml`

```xml
<?xml version="1.0" encoding="utf-8"?>
<actions>
  <action intentName="actions.intent.SEARCH">
    <fulfillment urlTemplate="intent://search?query={search_query}#Intent;scheme=wiki;end">
      <parameter name="search_query">
        <entity-set-reference entitySetId="SearchQuery"/>
      </parameter>
    </fulfillment>
  </action>
</actions>
```

**Handler**: `android/app/src/main/java/.../VoiceActionHandler.kt`

```kotlin
class VoiceActionHandler : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        
        // Gemini Nano has extracted query from natural language
        val query = intent.getStringExtra("query") ?: return finish()
        
        // Call gRPC API over Tailscale
        lifecycleScope.launch {
            try {
                val result = wikiGrpcClient.searchContent(query)
                
                // Return structured context for Gemini to interpret
                // NOT pre-formatted speech - let Gemini generate response
                val response = Intent().apply {
                    putExtra("title", result.results.first().title)
                    putExtra("content", result.results.first().fragment)
                    putExtra("identifier", result.results.first().identifier)
                }
                setResult(RESULT_OK, response)
            } catch (e: Exception) {
                val errorResponse = Intent().apply {
                    putExtra("error", "Could not reach wiki")
                    putExtra("suggestion", "Check Tailscale connection")
                }
                setResult(RESULT_OK, errorResponse)
            }
            finish()
        }
    }
}
```

**Key Change**: Return structured data in Intent extras, not pre-formatted speech. Gemini Nano interprets the context and generates natural language appropriate to the user's original query.

**File**: `android/app/src/main/AndroidManifest.xml` (additions)

```xml
<application>
  <activity>
    <!-- App Actions metadata -->
    <meta-data
      android:name="android.app.shortcuts"
      android:resource="@xml/actions" />
    
    <!-- Deep link handling -->
    <intent-filter>
      <action android:name="android.intent.action.VIEW" />
      <category android:name="android.intent.category.DEFAULT" />
      <category android:name="android.intent.category.BROWSABLE" />
      <data android:scheme="wiki" android:host="search" />
    </intent-filter>
  </activity>
</application>
```

**Natural Language Commands Enabled:**

- "Hey Google, where are my [item]?"
- "Hey Google, find [item] in the wiki"
- "Hey Google, do I have any [item]?"
- Any natural phrasing - Gemini Nano handles interpretation

#### 3. iOS App Intents Integration

**Purpose**: Register wiki search capability with Siri. Apple Intelligence (on-device LLM) handles natural language understanding and response generation.

**File**: `ios/App/AppIntents/WikiSearchIntent.swift`

```swift
import AppIntents
import Foundation

@available(iOS 18.0, *)
struct WikiSearchIntent: AppIntent {
    static var title: LocalizedStringResource = "Search Wiki"
    static var description = IntentDescription("Search the wiki for information")
    
    @Parameter(title: "Search Query")
    var query: String
    
    static var parameterSummary: some ParameterSummary {
        Summary("Search wiki for \(\.$query)")
    }
    
    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<SearchResult> {
        do {
            // Call gRPC API over Tailscale
            let grpcClient = WikiGrpcClient(baseURL: "https://wiki.monster-orfe.ts.net")
            let apiResponse = try await grpcClient.searchContent(query: query)
            
            // Return structured context for Apple Intelligence to interpret
            // NOT pre-formatted speech - let Apple Intelligence generate response
            let result = SearchResult(
                title: apiResponse.results.first?.title ?? "No results",
                content: apiResponse.results.first?.fragment ?? "",
                identifier: apiResponse.results.first?.identifier
            )
            
            return .result(value: result)
        } catch {
            // Return error context, Apple Intelligence will generate appropriate message
            let errorResult = SearchResult(
                title: "Error",
                content: "Could not reach wiki. Check Tailscale connection.",
                identifier: nil
            )
            return .result(value: errorResult)
        }
    }
}

// Structured result for Apple Intelligence to reason about
struct SearchResult: Codable {
    let title: String
    let content: String
    let identifier: String?
}

@available(iOS 18.0, *)
struct WikiAppShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: WikiSearchIntent(),
            phrases: [
                "Search \(.applicationName) for \(\.$query)",
                "Look up \(\.$query) in \(.applicationName)",
                "Find \(\.$query) in the wiki",
                "Where is \(\.$query)",
                "Do I have \(\.$query)"
            ]
        )
    }
}
```

**Key Change**: Return structured `SearchResult` type, not pre-formatted dialog. Apple Intelligence interprets the content and generates natural language response appropriate to the user's original question.

**Natural Language Commands Enabled:**

- "Hey Siri, where are my [item]?"
- "Hey Siri, find [item] in the wiki"
- "Hey Siri, do I have any [item]?"
- Any natural phrasing - Apple Intelligence handles interpretation

#### 4. gRPC Client Integration

Both Android and iOS intent handlers need to call the existing gRPC APIs over Tailscale to retrieve wiki content.

**Shared gRPC Client Interface** (implemented in each platform's native language):

```
interface WikiGrpcClient {
    async searchContent(query: string) -> SearchContentResponse
    async getPage(pageId: string) -> PageContent
}

SearchContentResponse {
    results: [
        {
            title: string
            fragment: string  // Plain text excerpt for context
            identifier: string
        }
    ]
}

Configuration:
- Base URL: https://wiki.monster-orfe.ts.net (Tailscale address)
- Transport: gRPC-Web or HTTP/2
- Timeout: 5 seconds (voice responses need quick turnaround)
- Error handling: Network errors, timeouts, empty results
```

**Android Implementation**: Use gRPC-Kotlin or gRPC-Java libraries
**iOS Implementation**: Use gRPC-Swift or Connect-Swift libraries

**Key Requirement**: Both implementations must work over Tailscale network without authentication (protected by network boundary).

**Response Handling**: Return full `fragment` content to intent handlers. Let platform LLMs interpret and extract relevant information based on user's query context.

## Development Workflow

### Initial Setup

**Add required packages to devbox:**

```bash
# Add Node.js tools (Capacitor CLI requires Node/Bun)
# Already have: bun@latest

# Add Android SDK components (for Android builds)
devbox add android-tools  # adb, fastboot, etc.
devbox add gradle         # Build system for Android

# Note: Xcode (iOS) must be installed via Mac App Store (macOS only)
# Android Studio and Xcode are GUI tools, installed separately
```

**Install Capacitor via Bun (managed in package.json):**

```bash
# In devbox shell
devbox shell

# Add Capacitor dependencies to package.json
cd /home/brendanjerwin/src/simple_wiki
bun add @capacitor/core @capacitor/cli
bun add @capacitor/android @capacitor/ios @capacitor/app

# Initialize Capacitor
npx cap init "Simple Wiki" "net.ts.monster-orfe.wiki" --web-dir=static

# Add platforms
npx cap add android
npx cap add ios
```

**Update devbox.json scripts:**

Add Capacitor-specific scripts to `devbox.json`:

```json
{
  "shell": {
    "scripts": {
      "cap:sync": ["npx cap sync"],
      "cap:android": ["npx cap open android"],
      "cap:ios": ["npx cap open ios"],
      "cap:build:android": ["./scripts/build-android-release.sh $@"]
    }
  }
}
```

### Daily Development

```bash
# Enter devbox environment
devbox shell

# Start the Go server (existing workflow)
devbox services start

# Option 1: Run in browser (existing workflow)
# Just visit https://wiki.monster-orfe.ts.net

# Option 2: Run on Android emulator/device
devbox run cap:sync         # Copy web assets to native projects
devbox run cap:android      # Opens Android Studio
# Then run from Android Studio

# Option 3: Run on iOS simulator/device (macOS only)
devbox run cap:sync         # Copy web assets to native projects
devbox run cap:ios          # Opens Xcode
# Then run from Xcode
```

### Making Web Changes

```bash
# Make changes to static/js/*, static/templates/*, etc.
# (existing workflow)

# Sync changes to native apps
devbox run cap:sync
```

### Making Native Changes

- **Android**: Edit files in `android/` with Android Studio
- **iOS**: Edit files in `ios/` with Xcode
- Native changes persist, tracked in git

## Build and Distribution

### Distribution Model: Sideloading Only

Since the app requires Tailscale network access and APIs have no authentication, **app store distribution is not viable**. Instead:

**Android**:

- Build APK locally or via GitHub Actions
- Distribute via:
  - Direct APK download (host on wiki itself)
  - Obtainium (auto-update from GitHub Releases)
  - F-Droid (if desired, but requires additional setup)

**iOS**:

- Build IPA via GitHub Actions (macOS runner) or local Mac
- Distribute via:
  - TestFlight (for personal use, up to 100 internal testers)
  - Direct IPA install (requires developer certificate, limited to registered devices)
  - AltStore/SideStore (third-party sideloading tools)

**Key Consideration**: Users must already have Tailscale installed and configured on their devices for the app to work.

### Android Build Process

**Requirements:**

- Android SDK via devbox (android-tools, gradle)
- Signing keystore (generated once, stored securely)
- No Google Play Developer account needed (sideloading)

**Local build process:**

```bash
# In devbox shell (Linux)
devbox shell

# Sync latest web assets
devbox run cap:sync

# Open Android Studio (or build via command line)
devbox run cap:android

# In Android Studio:
# 1. Build → Generate Signed Bundle / APK
# 2. Choose BOTH:
#    a) "Android App Bundle" (.aab) for Play Store
#    b) "APK" for Obtainium direct distribution
# 3. Select/create signing key
# 4. Build both artifacts

# Or use automated build script:
devbox run cap:build:android 1.0.0

# For Play Store:
# - Upload .aab file to Google Play Console
# - App details, screenshots, description
# - Submit for review (1-3 days)

# For Obtainium:
# - Upload signed APK to GitHub Releases or direct hosting
# - Tag release with version (e.g., v1.0.0)
# - Users add via Obtainium app
```

### iOS Build Process

**Requirements:**

- Apple Developer account ($99/year) - optional, can use free account with limitations
- GitHub Actions with macOS runners (for builds without local Mac), OR
- Local Mac for manual builds
- Signing certificates (Developer ID or free provisioning)

**Distribution Options:**

1. **TestFlight (recommended for personal use)**:
   - Upload IPA to App Store Connect
   - Share with internal testers (up to 100)
   - No app review required for internal testing
   - Builds expire after 90 days (must rebuild periodically)

2. **Direct IPA Sideloading** (free developer account):
   - Build locally with Xcode
   - Install via cable or AltStore/SideStore
   - Limited to registered devices (up to 100 per year with paid account, 3 with free)
   - Apps expire after 7 days (free account) or 1 year (paid)

3. **GitHub Actions Build** (recommended approach):
   - Automatic builds on tag push
   - No local Mac required after initial setup
   - Upload to TestFlight or create IPA artifact

**GitHub Actions Setup (One-Time)**:

```bash
# 1. Generate certificates (requires temporary Mac access or Mac VM)
# Use Xcode to create signing certificate
# Export .p12 file and provisioning profile

# 2. Add GitHub Secrets:
# - IOS_CERTIFICATE_BASE64: Base64-encoded .p12 file
# - IOS_CERTIFICATE_PASSWORD: Password for .p12
# - IOS_PROVISION_PROFILE_BASE64: Base64-encoded provisioning profile
# (Optional, for TestFlight):
# - APPSTORE_CONNECT_KEY_ID
# - APPSTORE_CONNECT_ISSUER_ID  
# - APPSTORE_CONNECT_KEY

# 3. Trigger builds by pushing tags
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions builds on macOS runner and uploads artifact
```

# Sync latest web assets

devbox run cap:sync

**Manual iOS Build (if you have Mac access)**:

```bash
# On macOS
devbox shell
devbox run cap:sync
devbox run cap:ios

# In Xcode:
# 1. Product → Archive
# 2. Distribute App → Ad Hoc (for direct IPA) or App Store Connect (for TestFlight)
# 3. Export IPA or upload to TestFlight
```

### Distribution Summary

**Android**:

- ✅ Build locally on Linux via devbox
- ✅ Distribute via GitHub Releases + Obtainium for auto-updates
- ✅ Direct APK download from wiki server
- ❌ No Google Play Store (APIs lack authentication)

**iOS**:

- ✅ Build via GitHub Actions (no local Mac needed after setup)
- ✅ Distribute via TestFlight for internal testing (up to 100 users)
- ✅ Direct IPA sideloading (limited devices, requires renewal)
- ❌ No App Store (APIs lack authentication)

**Critical**: All users must have Tailscale installed and connected to access the wiki APIs.

# 6. Create App Store Connect API key

# Encode files for GitHub Secrets

base64 -i certificate.p12 | pbcopy  # Copy to clipboard
base64 -i profile.mobileprovision | pbcopy

```

**GitHub Actions workflow:**

Create `.github/workflows/build-ios.yml`:

```yaml
name: Build iOS

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:  # Manual trigger

jobs:
  build:
    runs-on: macos-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3
      
      - name: Setup Bun
        uses: oven-sh/setup-bun@v1
      
      - name: Install dependencies
        run: bun install
      
      - name: Sync Capacitor
        run: npx cap sync ios
      
      - name: Install Apple Certificate
        env:
          CERTIFICATE_BASE64: ${{ secrets.IOS_CERTIFICATE_BASE64 }}
          CERTIFICATE_PASSWORD: ${{ secrets.IOS_CERTIFICATE_PASSWORD }}
          PROVISION_PROFILE_BASE64: ${{ secrets.IOS_PROVISION_PROFILE_BASE64 }}
        run: |
          # Decode certificate and provision profile
          echo "$CERTIFICATE_BASE64" | base64 --decode > certificate.p12
          echo "$PROVISION_PROFILE_BASE64" | base64 --decode > profile.mobileprovision
          
          # Create keychain and import certificate
          security create-keychain -p "" build.keychain
          security import certificate.p12 -k build.keychain -P "$CERTIFICATE_PASSWORD" -T /usr/bin/codesign
          security list-keychains -s build.keychain
          security default-keychain -s build.keychain
          security unlock-keychain -p "" build.keychain
          security set-key-partition-list -S apple-tool:,apple: -s -k "" build.keychain
          
          # Install provisioning profile
          mkdir -p ~/Library/MobileDevice/Provisioning\ Profiles
          cp profile.mobileprovision ~/Library/MobileDevice/Provisioning\ Profiles/
      
      - name: Build iOS app
        run: |
          cd ios/App
          xcodebuild -workspace App.xcworkspace \
                     -scheme App \
                     -archivePath App.xcarchive \
                     -configuration Release \
                     archive
          
          xcodebuild -exportArchive \
                     -archivePath App.xcarchive \
                     -exportPath build \
                     -exportOptionsPlist ExportOptions.plist
      
      - name: Upload to TestFlight
        env:
          APPSTORE_CONNECT_KEY_ID: ${{ secrets.APPSTORE_CONNECT_KEY_ID }}
          APPSTORE_CONNECT_ISSUER_ID: ${{ secrets.APPSTORE_CONNECT_ISSUER_ID }}
          APPSTORE_CONNECT_KEY: ${{ secrets.APPSTORE_CONNECT_KEY }}
        run: |
          # Upload using App Store Connect API
          xcrun altool --upload-app \
                       --type ios \
                       --file "ios/App/build/App.ipa" \
                       --apiKey "$APPSTORE_CONNECT_KEY_ID" \
                       --apiIssuer "$APPSTORE_CONNECT_ISSUER_ID"
      
      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: ios-app
          path: ios/App/build/App.ipa
```

**Automated release process:**

```bash
# From Linux development machine
devbox shell

# Tag and push to trigger iOS build on GitHub
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will:
# 1. Build iOS app on macOS runner
# 2. Upload to TestFlight automatically
# 3. Save IPA as artifact (downloadable from Actions tab)

# No macOS required! 🎉
```

## Implementation Phases

### Phase 0: Prerequisites ✓

**Status**: Partially complete (PWA exists, needs enhancement)

- [x] PWA basic implementation (manifest exists)
- [ ] Enhanced manifest with shortcuts and share_target
- [ ] PWA tested on mobile devices

**Estimated Effort**: 1-2 hours (complete manifest enhancements)

### Phase 1: Capacitor Setup and Android Infrastructure (TDD Foundation) ✓

**Status**: Complete

**What Was Completed:**

- [x] Android SDK setup via devbox (non-interactive mode for CI)
- [x] Capacitor 7 initialized with correct app ID (com.github.brendanjerwin.simple_wiki)
- [x] Test frameworks configured (JUnit 5, MockK, AndroidX Test)
- [x] CI/CD pipeline for Android builds and tests
- [x] Environment-based configuration (dev vs prod)
- [x] Comprehensive documentation (docs/android-development.md)
- [x] All placeholder tests removed (ready for real tests)
- [x] node_modules bloat fixed (433MB → 12MB)

**Key Decisions Made:**

- Environment-based server URLs (CAPACITOR_DEV variable)
- CI-aware Gradle daemon (disabled in CI, enabled locally)
- Android SDK in ~/.android-sdk (not via Nix)
- No mock gRPC server yet (deferred to Phase 2)
- No code coverage yet (deferred - add when real tests exist)

**Commit**: d02c2638 on feature/voice-assistant-phase-1

**Actual Effort**: ~6 hours (including code review fixes)

**TDD Emphasis**: Infrastructure before implementation ✓

---

### Phase 2: gRPC Client Implementation (TDD)

**Goal**: Implement wiki API client with comprehensive test coverage

**Tasks (TDD Order):**

1. **SearchContent Client** (TDD):
   - Write test: `searchContent returns results when API succeeds`
   - Implement: Basic search client
   - Write test: `searchContent throws on network error`
   - Implement: Error handling
   - Write test: `searchContent handles empty results`
   - Implement: Empty result handling
   - Write test: `searchContent respects timeout`
   - Implement: Timeout configuration

2. **ReadPage Client** (TDD):
   - Write test: `readPage returns markdown and frontmatter`
   - Implement: Basic page fetching
   - Write test: `readPage handles missing pages`
   - Implement: 404 error handling
   - Write test: `readPage parses frontmatter correctly`
   - Implement: Frontmatter parsing

3. **Tailscale Integration** (TDD):
   - Write test: `client connects over Tailscale`
   - Implement: Tailscale URL configuration
   - Write test: `client fails gracefully when Tailscale unreachable`
   - Implement: Connection error handling

**Acceptance Criteria:**

- All gRPC client tests passing
- 90%+ code coverage on client layer
- Integration tests pass against real wiki server
- Network error scenarios tested and handled
- Timeout and retry logic verified

**Estimated Effort**: 6-8 hours

**TDD Emphasis**: Test → Implement → Refactor for every feature

---

### Phase 3: Two-Phase Retrieval Logic (TDD)

**Goal**: Implement search → fetch full pages pattern

**Tasks (TDD Order):**

1. **Search Orchestration** (TDD):
   - Write test: `searches and fetches top result`
   - Implement: Basic search + fetch
   - Write test: `limits to top 3 results for context window`
   - Implement: Result limiting
   - Write test: `handles empty search results`
   - Implement: Empty result handling
   - Write test: `fetches pages in parallel`
   - Implement: Parallel fetching (coroutines)

2. **Error Resilience** (TDD):
   - Write test: `continues when individual page fetch fails`
   - Implement: Partial failure handling
   - Write test: `returns error when all fetches fail`
   - Implement: Complete failure detection
   - Write test: `times out on slow API calls`
   - Implement: Timeout enforcement

3. **Data Transformation** (TDD):
   - Write test: `maps API responses to WikiPage objects`
   - Implement: Data mapping layer
   - Write test: `handles missing frontmatter`
   - Implement: Null-safe frontmatter parsing
   - Write test: `truncates oversized content for context window`
   - Implement: Content size management

**Acceptance Criteria:**

- Two-phase retrieval works end-to-end
- Parallel fetching reduces latency
- Graceful handling of partial failures
- All edge cases tested (empty, timeout, errors)
- Integration tests verify real API behavior

**Estimated Effort**: 8-10 hours

**TDD Emphasis**: Complex orchestration requires comprehensive testing

---

### Phase 4: Android App Action Integration (TDD)

**Goal**: Wire up Google Assistant with two-phase retrieval

**Tasks (TDD Order):**

1. **App Action Definition** (TDD):
   - Write test: `action receives query parameter from Assistant`
   - Implement: Basic action handler
   - Write test: `action calls search orchestrator`
   - Implement: Orchestrator integration
   - Write test: `action returns structured WikiSearchResult`
   - Implement: Result packaging

2. **Gemini Nano Integration** (Manual Testing Required):
   - Configure actions.xml for search capability
   - Implement action fulfillment handler
   - Structure response for Gemini reasoning
   - Test manually with Google Assistant

3. **Error Handling** (TDD):
   - Write test: `action returns empty on no results`
   - Implement: Empty result response
   - Write test: `action returns error on network failure`
   - Implement: Error response with retry flag
   - Write test: `action provides helpful error messages`
   - Implement: User-friendly error text

**Acceptance Criteria:**

- App Action registered and discoverable
- "Hey Google, search my wiki" triggers action
- Complete page context passed to Gemini
- Gemini generates natural spoken responses
- Error scenarios provide helpful feedback
- All deterministic logic tested

**Estimated Effort**: 6-8 hours

**Manual Testing**: LLM responses require human verification

---

### Phase 5: Performance Optimization and Testing

**Goal**: Ensure acceptable latency and reliability

**Tasks (TDD Order):**

1. **Performance Tests** (TDD):
   - Write test: `complete flow under 2 seconds`
   - Optimize: Parallel fetching, caching
   - Write test: `handles slow network gracefully`
   - Implement: Progress indicators, timeouts

2. **Chaos Testing** (TDD):
   - Write test: `survives server crashes`
   - Verify: Graceful degradation
   - Write test: `handles malformed responses`
   - Verify: Parse error handling

3. **Manual LLM Validation**:
   - Execute documented test cases with real Assistant
   - Verify semantic correctness of responses
   - Test various query phrasings
   - Validate error message clarity

**Acceptance Criteria:**

- Voice query → response < 2 seconds (90th percentile)
- Chaos tests all pass
- Manual LLM test cases verified
- No crashes or hangs under any condition
- Performance metrics documented

**Estimated Effort**: 4-6 hours

---

### Phase 6: Android Distribution

**Goal**: Enable sideloading with Obtainium auto-updates

**Tasks:**

1. Set up Android signing keys
2. Create GitHub Actions workflow for APK builds
3. Configure automatic builds on git tags
4. Test Obtainium installation and updates
5. Create installation documentation

**Acceptance Criteria:**

- APK auto-builds on version tags
- Obtainium successfully installs and updates
- Users can sideload without developer mode hassles
- Update mechanism verified

**Estimated Effort**: 3-4 hours

---

### Phase 7: iOS Voice Integration (Future - Optional)

**Goal**: Only proceed if Android validates the concept

**Decision Point**: Stop here if:

- Android voice integration isn't useful in practice
- Gemini responses are poor quality
- Latency is unacceptable
- No iOS users request it

**If proceeding** with iOS:

- Follow same TDD patterns as Android
- Reuse protocol architecture
- Implement App Intents equivalent
- Test with Apple Intelligence
- Android app meets all user needs
- No iOS users requesting the app
- Complexity/cost not justified
- Apple Developer account ($99/year) not desired

**Tasks (if proceeding):**

1. Acquire Apple Developer account (free or $99/year)
2. Borrow/access Mac for one-time certificate generation (or use free provisioning)
3. Generate signing certificate and provisioning profile
4. Add iOS platform to Capacitor
5. Implement gRPC client in Swift
6. Implement App Intents (Swift) with voice handlers
7. Implement response transformation in Swift
8. Test on iOS simulator locally (if Mac available) or via GitHub Actions
9. Set up GitHub Actions for iOS builds (if using)
10. Configure TestFlight or direct IPA distribution

**Acceptance Criteria:**

- iOS app builds successfully
- Voice command "Hey Siri, search the wiki for batteries" works
- gRPC API calls function over Tailscale from iOS
- Spoken responses delivered correctly
- Distribution method works (TestFlight or IPA sideloading)

**Estimated Effort**: 10-14 hours (includes Swift gRPC client and App Intents)

### Phase 5: Monitoring and Iteration

**Goal**: Monitor usage and maintain the voice integration

**Tasks:**

1. Monitor voice query success rates
2. Collect feedback on response quality
3. Refine response transformation based on usage
4. Update apps when backend changes
5. Maintain compatibility with OS updates

**Ongoing Effort**: 1-2 hours per month

## Total Estimated Effort

### Android-Only Path (Recommended Starting Point)

- **Phase 0 (Prerequisites)**: 1-2 hours
- **Phase 1 (Android Voice Integration)**: 8-12 hours
- **Phase 2 (Android Testing)**: 4-6 hours
- **Phase 3 (Android Distribution)**: 2-3 hours
- **Phase 5 (Ongoing)**: 1-2 hours/month

**Total Android Investment**: 15-23 hours
**Ongoing Maintenance**: 1-2 hours/month

### Full iOS Addition (If Validated)

- **Phase 4 (iOS Voice Integration)**: 10-14 hours

**Total With iOS**: 25-37 hours
**Ongoing Maintenance**: 1-2 hours/month (covers both platforms)

**Strategy**: Ship Android first (Phases 0-3), validate voice integration works well, then decide if iOS (Phase 4) is worth the additional investment.

**Note on macOS**: One-time Mac access needed (2-3 hours) for iOS certificate generation if you proceed to Phase 4. All subsequent iOS builds can happen via GitHub Actions on macOS runners - no local macOS required.

## Risks and Mitigations

### Risk: Assistant Platform Limitations

**Likelihood**: Medium | **Impact**: High

Google and Apple may have restrictions on third-party voice integrations:

- **Mitigation**: Validate with proof-of-concept before full implementation
- **Fallback**: If spoken responses aren't possible, fall back to launching app with visual results

### Risk: Tailscale Network Issues

**Likelihood**: Low | **Impact**: Medium

Voice queries fail if device isn't connected to Tailscale:

- **Mitigation**: Clear error messages: "Check your Tailscale connection"
- **Detection**: Test network connectivity before API calls
- **Documentation**: Explain Tailscale requirement upfront
- Be responsive to reviewer feedback

### Risk: Maintenance Burden

**Mitigation**:

- Keep native code minimal

### Risk: Response Quality

**Likelihood**: Medium | **Impact**: Medium

Wiki content may not always transform well into speakable answers:

- **Mitigation**: Start with simple extraction (first paragraph)
- **Enhancement**: Add structured metadata to wiki pages (location, summary fields)
- **Fallback**: "I found the page but couldn't extract a clear answer. Opening the wiki for you."

### Risk: Latency Issues

**Likelihood**: Low | **Impact**: Medium

Voice query → gRPC call → response → speech may take too long:

- **Mitigation**: Set 5-second timeout on API calls
- **Optimization**: Consider caching frequently accessed pages
- **Measurement**: Log latency metrics, optimize if >2 seconds average

### Risk: Platform API Changes

**Likelihood**: Low | **Impact**: Medium

Google/Apple may change App Actions/Intents APIs:

- **Mitigation**: Stay on stable Capacitor versions
- **Monitoring**: Watch platform release notes
- **Strategy**: Update only when necessary for compatibility

### Risk: No App Store Distribution

**Likelihood**: Certain | **Impact**: Low

Cannot use app stores due to lack of API authentication:

- **Acceptance**: This is a known constraint
- **Users**: Must be comfortable with sideloading (Obtainium, TestFlight, IPA)
- **Documentation**: Clear installation instructions required

## Decision Points

### After Phase 0 (Prerequisites Complete)

**Question**: Do users actually want voice assistant integration with spoken responses?

**Validation**:

- Ask users: "Would you use voice commands to get spoken answers from the wiki?"
- Survey current user base about voice assistant usage habits
- Gauge interest in hands-free access

**Decision**:

- ✅ **Multiple users interested** → Proceed to Phase 1
- ❌ **Lukewarm response** → Defer, revisit in 6 months
- ✅ **Personal need** → Proceed (it's your wiki, build what you want!)

### After Phase 2 (Android Testing Complete)

**Question**: Is the voice integration working well enough to distribute?

**Validation**:

- Test voice queries with real users
- Measure response quality and latency
- Assess user satisfaction with spoken answers

**Decision**:

- ✅ **High satisfaction** → Proceed to Phase 3 (distribution)
- ⚠️ **Mixed results** → Iterate on response transformation, retest
- ❌ **Poor results** → Reassess feasibility, may need different approach

### After Phase 3 (Android Distribution Working)

**Question**: Should we invest in iOS support?

**Validation**:

- Are there iOS users requesting the app?
- Is Android voice integration being used regularly?
- Is the added complexity (10-14 hours) justified?

**Decision**:

- ✅ **iOS users actively requesting it** → Proceed to Phase 4
- ❌ **Android sufficient** → Stop here, focus on refinement
- ⚠️ **One power user wants it** → Your call (it's your wiki!)

### After Phase 4 or 3 (Apps in Use)

**Question**: Are the voice-enabled apps providing value?

**Metrics**:

- Voice query frequency
- Response quality (users getting correct answers?)
- User feedback
- Usage patterns (which queries work well vs poorly)

**Decision**:

- ✅ **Regular usage, positive feedback** → Continue maintenance and enhancement
- ⚠️ **Low usage** → Minimal maintenance mode, update only for critical issues
- ❌ **Zero usage** → Consider deprecating, focus on PWA
- ✅ **Multiple iOS users requesting** → Proceed to Phase 4
  - Borrow Mac for certificate setup
  - Build CI/CD pipeline
  - Add Siri support
- ❌ **No iOS demand** → Stop at Phase 3
  - Save 8-10 hours
  - Users can still use PWA on iOS
  - Reassess in 6 months
- ⚠️ **One power user wants it** → Your call
  - It's your wiki, build what you want!
  - But consider if PWA on iOS is "good enough"

### After Phase 5 (Apps in Use)

**Question**: Are the native apps providing value?

**Metrics**:

- Download counts (if using stores)
- Active users (if analytics enabled)
- User feedback/reviews
- Voice command usage

**Decision**:

- ✅ **Healthy adoption** → Continue maintenance
  - Users are using voice commands
  - Positive feedback
  - Worth the ongoing effort
- ❌ **Zero usage** → Consider deprecating
  - Focus on PWA only
  - Reduce maintenance burden
  - Saved 1-2 hours/month
- ⚠️ **Low but personal use** → Minimal maintenance
  - Update only for critical fixes
  - Skip non-essential platform updates
  - Keep it as long as you use it

## Testing Strategy

### TDD Principles

**ALL development MUST follow Test-Driven Development practices:**

1. **Write test first**: Before implementing any feature, write a failing test that defines desired behavior
2. **Minimal implementation**: Write just enough code to make the test pass
3. **Refactor**: Clean up code while keeping tests green
4. **Comprehensive coverage**: Every feature must have corresponding tests

This applies to all layers: API clients, data transformation, error handling, and integration flows.

### Test Layers

#### 1. Unit Tests (TDD Foundation)

**API Client Layer** - Test all gRPC interactions in isolation:

```kotlin
class WikiAPIClientTest {
    @Test
    fun `searchContent returns results when API succeeds`() {
        // Given: Mock gRPC response with search results
        val mockResponse = SearchContentResponse.newBuilder()
            .addResults(SearchResult.newBuilder()
                .setIdentifier("cr2032_batteries")
                .setTitle("CR2032 Batteries")
                .build())
            .build()
        
        // When: Call search
        val results = client.searchContent("batteries")
        
        // Then: Results are parsed correctly
        assertEquals(1, results.size)
        assertEquals("cr2032_batteries", results[0].identifier)
    }
    
    @Test
    fun `searchContent throws exception on network error`() {
        // Given: Network failure
        mockServer.enqueue(MockResponse().setResponseCode(500))
        
        // When/Then: Expect specific exception
        assertThrows<NetworkException> {
            client.searchContent("batteries")
        }
    }
    
    @Test
    fun `readPage returns rendered HTML and frontmatter`() {
        // Given: Mock ReadPage response
        val mockResponse = ReadPageResponse.newBuilder()
            .setRenderedContentHtml("<h1>CR2032 Batteries</h1><p>Coin cell batteries...</p>")
            .setFrontMatterToml("location = 'Lab Desk Wall Bin C1'")
            .build()
        
        // When: Fetch page
        val page = client.readPage("cr2032_batteries")
        
        // Then: Content is available
        assertNotNull(page.renderedHtml)
        assertTrue(page.renderedHtml.contains("<h1>CR2032"))
        assertTrue(page.frontmatter.contains("location"))
    }
    
    @Test
    fun `readPage handles missing page gracefully`() {
        // Given: Page doesn't exist
        mockServer.enqueue(MockResponse().setResponseCode(404))
        
        // When/Then: Returns null or throws PageNotFoundException
        assertThrows<PageNotFoundException> {
            client.readPage("nonexistent")
        }
    }
}
```

**Data Transformation Layer** - Test result mapping:

```kotlin
class WikiSearchResultMapperTest {
    @Test
    fun `maps search results to WikiPage objects`() {
        // Given: Search result and page content
        val searchResult = SearchResult(identifier = "cr2032", title = "CR2032")
        val pageContent = ReadPageResponse(
            renderedContentHtml = "<h1>CR2032</h1><p>Details...</p>",
            frontMatterToml = "location = 'Bin C1'"
        )
        
        // When: Map to WikiPage
        val wikiPage = mapper.toWikiPage(searchResult, pageContent)
        
        // Then: Fields are correctly mapped
        assertEquals("cr2032", wikiPage.identifier)
        assertEquals("<h1>CR2032</h1><p>Details...</p>", wikiPage.renderedHtml)
        assertEquals("location = 'Bin C1'", wikiPage.frontmatter)
    }
    
    @Test
    fun `handles missing frontmatter gracefully`() {
        // Given: Page with no frontmatter
        val pageContent = ReadPageResponse(
            renderedContentHtml = "<h1>Page</h1>",
            frontMatterToml = ""
        )
        
        // When: Map to WikiPage
        val wikiPage = mapper.toWikiPage(searchResult, pageContent)
        
        // Then: Empty frontmatter doesn't cause errors
        assertEquals("", wikiPage.frontmatter)
    }
}
```

**Error Handling Layer** - Test all error scenarios:

```kotlin
class SearchWikiActionErrorTest {
    @Test
    fun `returns empty result when no search results found`() {
        // Given: Empty search response
        mockClient.setupSearchResponse(emptyList())
        
        // When: Execute action
        val result = action.execute("flux capacitor")
        
        // Then: Returns structured empty result
        assertTrue(result is ActionResult.Empty)
        assertEquals("No results found for: flux capacitor", result.message)
    }
    
    @Test
    fun `returns error when Tailscale unreachable`() {
        // Given: Network connectivity failure
        mockClient.throwOnSearch(NetworkException("Tailscale unreachable"))
        
        // When: Execute action
        val result = action.execute("batteries")
        
        // Then: Returns network error with user-friendly message
        assertTrue(result is ActionResult.Error)
        assertTrue(result.retryable)
        assertContains(result.message, "network")
    }
    
    @Test
    fun `handles partial page fetch failures gracefully`() {
        // Given: Search returns 3 results, but 1 page fails to load
        mockClient.setupSearchResponse(listOf(result1, result2, result3))
        mockClient.throwOnReadPage("page2", PageNotFoundException())
        
        // When: Execute action
        val result = action.execute("batteries")
        
        // Then: Returns successful results, skips failed page
        assertTrue(result is ActionResult.Success)
        assertEquals(2, result.pages.size)  // Only 2 pages loaded
    }
    
    @Test
    fun `respects timeout limits`() {
        // Given: API call takes too long
        mockClient.setReadPageDelay(Duration.ofSeconds(30))
        
        // When/Then: Times out appropriately
        assertThrows<TimeoutException> {
            action.execute("batteries")
        }
    }
}
```

#### 2. Integration Tests (Multi-Component)

**Two-Phase Retrieval Flow** - Test the complete search → fetch pattern:

```kotlin
@IntegrationTest
class SearchWikiActionIntegrationTest {
    @Test
    fun `complete flow from query to full page content`() {
        // Given: Real wiki server with test data
        setupTestWikiPage(
            identifier = "cr2032_batteries",
            renderedHtml = "<h1>CR2032 Batteries</h1><p>Coin cell batteries for electronics.</p>",
            frontmatter = "location = 'Lab Desk Wall Bin C1'\ncategory = 'Electronics'"
        )
        
        // When: Execute search action
        val result = action.execute("batteries")
        
        // Then: Returns complete page content
        assertTrue(result is ActionResult.Success)
        assertEquals(1, result.pages.size)
        
        val page = result.pages[0]
        assertEquals("cr2032_batteries", page.identifier)
        assertContains(page.renderedHtml, "Coin cell batteries")
        assertContains(page.frontmatter, "Lab Desk Wall Bin C1")
    }
    
    @Test
    fun `fetches top 3 results only for context window management`() {
        // Given: Search returns 5 results
        setupMultipleTestPages(5)
        
        // When: Execute search
        val result = action.execute("wire")
        
        // Then: Only top 3 are fetched
        assertEquals(3, result.pages.size)
    }
    
    @Test
    fun `parallel page fetching for performance`() {
        // Given: Multiple search results
        setupMultipleTestPages(3)
        
        // When: Execute search and measure time
        val startTime = System.currentTimeMillis()
        val result = action.execute("components")
        val elapsed = System.currentTimeMillis() - startTime
        
        // Then: Fetches complete in parallel, not sequential
        // (3 sequential would take ~300ms, parallel ~100ms)
        assertLessThan(elapsed, 150) // Parallel execution threshold
    }
}
```

**Tailscale Network Testing** - Test over actual Tailscale connection:

```kotlin
@RequiresTailscale
@IntegrationTest
class TailscaleIntegrationTest {
    @Test
    fun `can reach wiki server over Tailscale`() {
        // Verify Tailscale connectivity before running suite
        val reachable = tailscaleClient.ping("wiki.monster-orfe.ts.net")
        assertTrue(reachable, "Tailscale network unreachable")
    }
    
    @Test
    fun `search works over real Tailscale connection`() {
        // Given: Connected to Tailscale
        // When: Perform actual search
        val result = realClient.searchContent("batteries")
        
        // Then: Real results returned
        assertNotNull(result)
    }
}
```

#### 3. LLM Response Validation Tests

**Important**: We cannot test exact LLM responses (non-deterministic), but we can test semantic correctness.

```kotlin
class LLMResponseValidationTest {
    // These tests verify the ACTION returns correct data
    // Human verification needed to confirm Gemini generates good speech
    
    @Test
    fun `action provides complete context for location questions`() {
        // Given: Page with location in frontmatter
        setupTestPage(frontmatter = "location = 'Lab Desk Wall Bin C1'")
        
        // When: Execute action
        val result = action.execute("where are batteries")
        
        // Then: Result includes location data
        assertTrue(result.pages[0].frontmatter.contains("location"))
        // LLM should extract "Lab Desk Wall Bin C1" from this
    }
    
    @Test
    fun `action provides complete context for detail questions`() {
        // Given: Page with detailed rendered HTML
        setupTestPage(renderedHtml = "<h1>Item</h1><p>Warranty: 2 years</p><p>Color: Red</p>")
        
        // When: Execute action
        val result = action.execute("what is warranty")
        
        // Then: Full rendered HTML available for LLM to extract warranty
        assertContains(result.pages[0].renderedHtml, "Warranty: 2 years")
        // LLM should find "2 years" even though it's not in search snippet
    }
    
    @Test
    fun `action returns empty for truly nonexistent items`() {
        // Given: No pages match query
        
        // When: Search for nonexistent item
        val result = action.execute("flux capacitor")
        
        // Then: Empty result allows LLM to say "not found"
        assertTrue(result is ActionResult.Empty)
        // LLM should generate: "I couldn't find flux capacitor in your wiki"
    }
}

// Manual test cases (documented, run by QA/developer)
class ManualLLMTestCases {
    /*
     * These must be tested manually with real Google Assistant + Gemini Nano
     * Record actual LLM responses and verify semantic correctness
     * 
     * Test Case 1: Location Query
     *   User: "Hey Google, where are my CR2032 batteries?"
     *   Expected semantic: Response mentions "Lab Desk Wall Bin C1"
     *   
     * Test Case 2: Existence Query  
     *   User: "Hey Google, do I have magnet wire?"
     *   Expected semantic: Affirmative response + location
     *   
     * Test Case 3: Detail Query
     *   User: "Hey Google, what gauge is my magnet wire?"
     *   Expected semantic: Response mentions "28 AWG"
     *   
     * Test Case 4: Not Found
     *   User: "Hey Google, find flux capacitor in my wiki"
     *   Expected semantic: Negative response indicating not found
     *   
     * Test Case 5: Multiple Results
     *   User: "Hey Google, what batteries do I have?"
     *   Expected semantic: Lists multiple battery types or asks for clarification
     */
}
```

#### 4. Performance Tests

```kotlin
class PerformanceTest {
    @Test
    fun `complete query flow under 2 seconds`() {
        // Given: Real network conditions
        
        // When: Execute end-to-end query
        val startTime = System.currentTimeMillis()
        val result = action.execute("batteries")
        val elapsed = System.currentTimeMillis() - startTime
        
        // Then: Completes within acceptable time
        assertLessThan(elapsed, 2000) // 2 second target
    }
    
    @Test
    fun `handles slow network gracefully`() {
        // Given: Simulated network delay
        mockClient.setNetworkDelay(Duration.ofSeconds(5))
        
        // When/Then: Times out or shows loading indicator
        // Don't block indefinitely
    }
}
```

#### 5. Chaos/Resilience Tests

```kotlin
class ChaosTest {
    @Test
    fun `handles server crash mid-search`() {
        // Given: Server becomes unavailable during request
        mockServer.shutdownDuringRequest()
        
        // When/Then: Graceful error handling
        val result = action.execute("batteries")
        assertTrue(result is ActionResult.Error)
        assertTrue(result.retryable)
    }
    
    @Test
    fun `handles malformed API responses`() {
        // Given: API returns invalid protobuf
        mockServer.enqueue(MockResponse().setBody("invalid"))
        
        // When/Then: Doesn't crash, returns error
        assertThrows<ParseException> {
            action.execute("batteries")
        }
    }
    
    @Test
    fun `handles unexpectedly large responses`() {
        // Given: Page with 1MB of markdown
        setupHugePage(sizeBytes = 1_000_000)
        
        // When: Fetch page
        val result = action.execute("huge")
        
        // Then: Truncates or handles gracefully (context window limit)
        assertLessThan(result.pages[0].markdown.length, 100_000)
    }
}
```

### TDD Workflow

For each new feature:

1. **Write failing test** that defines the desired behavior
2. **Run test** - verify it fails for the right reason
3. **Write minimal code** to make test pass
4. **Run test** - verify it now passes
5. **Refactor** code while keeping tests green
6. **Repeat** for next feature

### Test Coverage Requirements

- **Unit tests**: 90%+ code coverage for all non-UI code
- **Integration tests**: All major user flows covered
- **Error scenarios**: Every error path tested
- **Manual LLM tests**: Documented test cases for all query types

### Continuous Testing

- **Pre-commit hooks**: Run unit tests before allowing commits
- **CI/CD pipeline**: Run full test suite on every PR
- **Regression suite**: Maintain collection of known-good queries and expected semantic outcomes

### Test Documentation

Each test must include:

- Clear Given/When/Then structure
- Purpose comment explaining what behavior is being verified
- Any setup requirements (network, mocks, test data)
- Expected outcomes with assertions

## Documentation Requirements

### Developer Docs

1. **Setup Guide**:
   - How to install native build tools (Android Studio on Linux)
   - Devbox package requirements
   - First-time Capacitor initialization
   - gRPC client setup and configuration
   - One-time iOS certificate generation (if proceeding with iOS)
   - GitHub Actions secrets configuration
2. **Build Guide**:
   - How to build and test Android apps locally (Linux)
   - How to trigger iOS builds via GitHub Actions (tag push, if implemented)
   - Using devbox scripts for all operations
   - Testing voice commands locally
3. **Release Guide**:
   - APK building and signing process
   - GitHub Releases workflow for Obtainium
   - TestFlight distribution (iOS, if implemented)
4. **Architecture Guide**:
   - Voice query flow documentation
   - gRPC integration details
   - Response transformation algorithm

### User Docs

1. **Installation Instructions**:
   - Sideloading APK via Obtainium (Android)
   - TestFlight or IPA installation (iOS, if implemented)
   - Tailscale setup requirements
2. **Voice Command Examples**:
   - Supported query patterns
   - Example queries that work well
   - What types of answers to expect
3. **Troubleshooting**:
   - "Check your Tailscale connection" errors
   - Voice commands not recognized
   - Poor quality responses
   - Network connectivity issues

### Maintenance Docs

1. **Update Process**: How to release new versions (APK builds, GitHub releases)
2. **Platform Compatibility**: Minimum OS versions supported
3. **Known Issues**: Platform-specific quirks and limitations
4. **Response Quality Tuning**: How to improve spoken answer quality over time

## Architecture Decision Records

### ADR-001: Android-First Implementation

**Status**: Accepted

**Context**: Need to validate voice assistant concept before investing in multi-platform support.

**Decision**: Implement Android with Gemini Nano first, defer iOS until concept is validated.

**Rationale**:

- Android allows sideloading without central authority registration
- Faster iteration cycle (no TestFlight delays)
- Developer's personal device is Android
- Can validate RAG architecture and two-phase retrieval pattern
- Reduced implementation cost (one platform instead of two)

**Consequences**:

- Positive: Faster time to working prototype
- Positive: No App Store/developer account requirements
- Positive: Can prove value before expanding scope
- Negative: iOS users cannot use initially (acceptable for personal project)
- Negative: May need to refactor for iOS later (mitigated by platform-agnostic protocol design)

---

### ADR-002: Two-Phase Retrieval with Rendered HTML

**Status**: Accepted

**Context**: Original design returned only search snippets (~200 chars), creating information bottleneck where LLM couldn't answer questions requiring details not in snippets. Additionally, the wiki uses a templating system that dynamically expands content (inventory relationships, cross-references) that doesn't exist in raw markdown.

**Decision**: Implement two-phase retrieval:

1. SearchContent API → identify relevant pages
2. ReadPage API (top N) → fetch rendered HTML + frontmatter

**Rationale**:

- Eliminates information bottleneck - LLM has complete context
- Enables answering detail questions ("what's the warranty?", "what voltage?", "what's in bin C1?")
- Frontmatter provides structured metadata (location, categories, tags)
- **Rendered HTML includes all templating logic applied** (inventory expansion, cross-references, dynamic content)
- **Template-expanded content critical**: Asking "what's in bin C1?" requires seeing the inventory list that was dynamically generated by the templating system
- Existing APIs already support this pattern (rendered_content_html field available)

**Consequences**:

- Positive: Comprehensive Q&A capability
- Positive: Better user experience (can answer any question about found pages)
- Positive: LLM can extract specific details from full rendered content
- Positive: Template-expanded content visible to LLM (inventory relationships, etc.)
- Negative: Additional API calls add latency (mitigated by parallel fetching)
- Negative: Larger payloads to LLM - HTML more verbose than markdown (mitigated by limiting to top 3 results)
- Acceptable trade-off: ~100-200ms extra latency for dramatically better answers including template-expanded content

---

### ADR-003: Leverage Platform LLMs (Gemini Nano)

**Status**: Accepted

**Context**: Could build custom LLM infrastructure or leverage platform-provided AI agents.

**Decision**: Use Gemini Nano (Android) and Apple Intelligence (future iOS) rather than custom LLM.

**Rationale**:

- Platform LLMs are on-device (privacy)
- Platform handles NLU, parameter extraction, response generation
- App becomes thin data access layer
- No need to manage models, embeddings, or inference
- Platform continuously improves models
- Reduces implementation and maintenance cost

**Consequences**:

- Positive: Minimal implementation complexity
- Positive: On-device privacy
- Positive: Natural language flexibility
- Positive: Platform improvements benefit us automatically
- Negative: Dependent on platform LLM availability and quality
- Negative: Limited control over response generation
- Acceptable trade-off: Simplicity and quality over control

---

### ADR-004: Sideload-Only Distribution (No App Stores)

**Status**: Accepted

**Context**: Wiki APIs lack authentication; must remain private via Tailscale.

**Decision**: Distribute via sideloading (APK) and Obtainium auto-updates only. No Play Store or App Store.

**Rationale**:

- APIs are Tailscale-only, cannot be exposed publicly
- App stores require public listings
- Target users comfortable with sideloading (personal/technical users)
- Obtainium provides auto-update capability
- Avoids app store fees, policies, review delays

**Consequences**:

- Positive: Can deploy immediately without app store approval
- Positive: Maintains security posture (Tailscale-only)
- Positive: No recurring fees or policies to comply with
- Negative: Limited discoverability (acceptable for private tool)
- Negative: Installation friction for non-technical users (acceptable for target audience)

---

### ADR-005: Test-Driven Development Mandatory

**Status**: Accepted

**Context**: Voice integration involves complex orchestration (two-phase retrieval), network operations, error handling, and non-deterministic LLM responses.

**Decision**: All development must follow TDD principles:

1. Write test first
2. Implement minimal code to pass test
3. Refactor while keeping tests green

**Rationale**:

- Complex orchestration requires comprehensive testing
- Network operations have many error modes
- Early testing reveals integration issues
- Regression protection as features evolve
- Documents expected behavior
- Enables confident refactoring

**Consequences**:

- Positive: High code quality and coverage
- Positive: Fewer production bugs
- Positive: Easier to refactor and evolve
- Positive: Tests document behavior
- Negative: Slower initial development (offset by fewer debugging sessions)
- Negative: Requires discipline and test infrastructure
- Acceptable trade-off: Quality and maintainability over speed

---

### ADR-006: Structured Error Responses for LLM

**Status**: Accepted

**Context**: LLM-enhanced assistants can generate contextual error messages, but need structured error information to do so.

**Decision**: Define explicit error type enumeration and return structured error results instead of pre-formatted messages.

**Rationale**:

- LLM can generate contextually appropriate error messages
- Structured errors enable retry logic
- Explicit error types improve debugging
- Better separation of concerns (app = data, LLM = language)

**Consequences**:

- Positive: Natural, contextual error messages
- Positive: Clear error contract
- Positive: Easier to debug (explicit error types)
- Negative: Requires defining error taxonomy
- Minimal trade-off: Small upfront design cost for better UX

---

### ADR-007: Context Window Management (Top 3 Results)

**Status**: Accepted

**Context**: LLMs have finite context windows. Returning all search results could exceed limits.

**Decision**: Limit full content retrieval to top 3 search results.

**Rationale**:

- Ensures payloads fit within Gemini Nano context window
- Most relevant results ranked first by search
- Reduces latency (fewer API calls)
- If answer not in top 3, query likely needs refinement

**Consequences**:

- Positive: Guaranteed to fit in context window
- Positive: Lower latency
- Positive: Simpler implementation
- Negative: Might miss relevant info in results 4+
- Acceptable trade-off: Can iterate based on usage data if needed

---

## Success Criteria

### Technical Success

- ✅ Android app builds and runs
- ✅ Voice commands trigger two-phase retrieval (search → fetch pages)
- ✅ Complete page content (frontmatter + markdown) passed to Gemini
- ✅ Gemini generates contextually appropriate spoken responses
- ✅ Error handling provides structured, helpful feedback
- ✅ Query → spoken response latency < 2 seconds (acceptable: < 3s)
- ✅ No crashes under any error condition
- ✅ Works reliably over Tailscale network
- ✅ 90%+ test coverage on all non-UI code
- ✅ All TDD tests passing (unit, integration, chaos)

### User Success

- ✅ Personal use case: Can query wiki via voice reliably
- ✅ Spoken answers are accurate and comprehensive
- ✅ Can answer detail questions (not just location lookups)
- ✅ Error messages are understandable
- ✅ Installation process (sideloading) is straightforward

### Validation Success

- ✅ Two-phase retrieval enables better answers than fragment-only approach
- ✅ Full context retrieval demonstrably improves answer quality
- ✅ Gemini Nano successfully reasons over complete page content
- ✅ Natural language queries work with various phrasings
- ✅ Concept validated - ready for iOS expansion (if desired)

### Maintenance Success

- ✅ Test suite provides confidence for refactoring
- ✅ Most updates deploy without native rebuilds (web changes only)
- ✅ Time spent on maintenance < 2 hours/month
- ✅ Can iterate on features based on usage feedback

## References

- [Capacitor Documentation](https://capacitorjs.com/docs)
- [Android App Actions](https://developers.google.com/assistant/app/overview)
- [iOS App Intents](https://developer.apple.com/documentation/appintents)
- [gRPC Documentation](https://grpc.io/docs/)
- [Tailscale Documentation](https://tailscale.com/kb/)
- [PWA Implementation Plan](./proj-pwa.md)

## Appendix: Example Voice Interactions

**Note**: These examples show natural language queries and LLM-generated responses. The app returns structured data; Apple Intelligence/Gemini Nano generate the spoken responses contextually.

### Apple Intelligence (iOS 18+)

**Example 1: Natural location query**

**User**: "Hey Siri, where are my CR2032 batteries?"

**What happens**:

- Apple Intelligence understands: user wants location of batteries
- Invokes: SearchWikiIntent(query="CR2032 batteries")
- App returns: `{title: "CR2032 Batteries", content: "Goes in: Lab Desk Wall Bin C1"}`
- Apple Intelligence generates natural response to "where" question

**Siri**: "Your CR2032 batteries are in Lab Desk Wall Bin C1."

---

**Example 2: Paraphrased query**

**User**: "Hey Siri, I'm looking for that 3D mouse thing"

**What happens**:

- Apple Intelligence interprets "3D mouse thing" → searches "3d mouse navigation"
- App returns: Space Navigator data with full fragment
- LLM emphasizes relevant details based on vague query

**Siri**: "I found the 3Dconnexion Space Navigator. It's in Lab Desk Wall Bin L1. It's a 3D navigation knob with USB connectivity."

---

**Example 3: Existence check**

**User**: "Hey Siri, do I have any magnet wire?"

**What happens**:

- Apple Intelligence understands yes/no question about existence
- App returns: magnet wire search results
- LLM generates affirmative response with location

**Siri**: "Yes, you have 28 AWG magnet wire in Lab Desk Wall Bin A2."

### Gemini Nano (Android)

**Example 1: Natural query**

**User**: "Hey Google, where did I put my batteries?"

**What happens**:

- Gemini Nano interprets intent → SearchWikiIntent
- App returns structured battery data
- Gemini generates contextual response

**Assistant**: "I found your CR2032 batteries in Lab Desk Wall Bin C1."

---

**Example 2: Complex query**

**User**: "Hey Google, what's that navigation device called and where is it?"

**What happens**:

- Gemini understands compound question (name + location)
- SearchWikiIntent(query="navigation device")
- App returns Space Navigator context
- Gemini addresses both parts of question

**Assistant**: "That's the 3Dconnexion Space Navigator. It's in Lab Desk Wall Bin L1."

### Error Scenarios (Structured Error Responses)

**User**: "Hey Google, where are my flux capacitors?"

**What happens**:

- Phase 1: SearchContent returns empty results
- App returns: `ActionResult.Empty("No results found for: flux capacitors")`
- Gemini generates helpful error message

**Assistant**: "I couldn't find anything about flux capacitors in your wiki."

---

**User**: "Hey Google, find batteries" (not connected to Tailscale)

**What happens**:

- Phase 1: SearchContent fails with network error
- App returns: `ActionResult.Error(type=NETWORK_FAILURE, retryable=true)`
- Gemini interprets error context

**Assistant**: "I'm having trouble reaching your wiki. Make sure you're connected to Tailscale."

---

**User**: "Hey Google, what's in my lab?" (search succeeds but page fetch fails)

**What happens**:

- Phase 1: SearchContent finds 3 results
- Phase 2: ReadPage fails for 1 result (partial failure)
- App returns: `ActionResult.Success(pages=[page1, page2])` (2 of 3)
- Gemini reasons over available pages

**Assistant**: "I found information about your multimeter and oscilloscope in the lab..."

---

### Error Handling Architecture

**Error Type Enumeration** (Explicit Contract):

```kotlin
enum class ErrorType {
    NETWORK_FAILURE,      // Tailscale unreachable, connection timeout
    SERVICE_UNAVAILABLE,  // Wiki server down or unresponsive
    NOT_FOUND,           // Search returned no results
    PARSE_ERROR,         // Invalid API response
    TIMEOUT,             // Request exceeded time limit
    PARTIAL_FAILURE      // Some results retrieved, some failed
}

sealed class ActionResult {
    data class Success(val pages: List<WikiPage>) : ActionResult()
    data class Empty(val message: String) : ActionResult()
    data class Error(
        val type: ErrorType,
        val message: String,
        val retryable: Boolean,
        val technicalDetail: String? = null  // For logging, not shown to user
    ) : ActionResult()
}
```

**Structured Error Responses for LLM**:

Instead of pre-formatted error messages, return structured error information that Gemini can interpret:

```kotlin
// Network failure
ActionResult.Error(
    type = ErrorType.NETWORK_FAILURE,
    message = "Cannot reach wiki server. Check Tailscale connection.",
    retryable = true,
    technicalDetail = "Connection timeout after 10s to wiki.monster-orfe.ts.net"
)

// Empty results
ActionResult.Empty(
    message = "No results found for: ${query}"
)

// Partial failure (some pages fetched successfully)
ActionResult.Success(
    pages = listOf(page1, page2)  // Only 2 of 3 fetched
    // Gemini doesn't know 1 failed - responds based on available data
)
```

The LLM receives this structured information and generates contextually appropriate natural language.

### Key Differences from Traditional Approach

**Old (Pre-LLM)**: App must format exact speech

```
User: "search wiki for batteries"
App returns: "CR2032 Batteries go in Lab Desk Wall Bin C1"
```

**New (LLM-Enhanced)**: App provides context, LLM adapts response

```
User: "where are my batteries?" OR "do I have batteries?" OR "find my CR2032s"
App returns: {title, content, location}
LLM generates: Appropriate response for the specific question asked
```

### Two-Phase API Response Examples

**Query: "batteries"**

**Phase 1 - SearchContent Response:**

```json
{
  "results": [
    {
      "identifier": "cr2032_batteries",
      "title": "CR2032 Batteries",
      "fragment": "CR2032 Batteries\n\n Goes in: Lab Desk Wall Bin C1"
    },
    {
      "identifier": "cr2016_batteries", 
      "title": "CR2016 Batteries",
      "fragment": "CR2016 Batteries\n\n Goes in: Lab Desk Wall Bin C2"
    }
  ]
}
```

**Phase 2 - ReadPage Response (for "cr2032_batteries"):**

```json
{
  "front_matter_toml": "location = 'Lab Desk Wall Bin C1'\ncategory = 'Electronics'\ntags = ['batteries', 'coin-cell', 'cr2032']\npurchase_date = '2024-01-15'",
  "rendered_content_html": "<h1>CR2032 Batteries</h1>\n<p>Coin cell batteries commonly used in electronics.</p>\n<h2>Specifications</h2>\n<ul>\n<li>Voltage: 3V</li>\n<li>Diameter: 20mm</li>\n<li>Thickness: 3.2mm</li>\n<li>Chemistry: Lithium</li>\n</ul>\n<h2>Typical Applications</h2>\n<ul>\n<li>Motherboard CMOS batteries</li>\n<li>Key fobs</li>\n<li>Small electronics</li>\n</ul>\n<h2>Inventory</h2>\n<p>Quantity: ~20 batteries in stock</p>"
}
```

**What Gemini Receives:**

```kotlin
WikiSearchResult(
  pages = listOf(
    WikiPage(
      identifier = "cr2032_batteries",
      frontmatter = "location = 'Lab Desk Wall Bin C1'...",
      renderedHtml = "<h1>CR2032 Batteries</h1>..."  // FULL RENDERED HTML
    ),
    WikiPage(
      identifier = "cr2016_batteries", 
      frontmatter = "location = 'Lab Desk Wall Bin C2'...",
      renderedHtml = "<h1>CR2016 Batteries</h1>..."  // FULL RENDERED HTML
    )
  )
)
```

**Example User Questions Gemini Can Now Answer:**

1. "Where are my CR2032 batteries?" → Extracts location from frontmatter
2. "What voltage are my CR2032s?" → Finds "Voltage: 3V" in rendered HTML
3. "How many CR2032 batteries do I have?" → Finds "~20 batteries" in inventory section
4. "When did I buy CR2032 batteries?" → Extracts purchase_date from frontmatter

**With fragment-only approach**, questions 2-4 would fail because that information isn't in the 200-character snippet. **With full rendered content**, Gemini has complete context to answer any question about the page.

---

**Query: "what's in bin C1?"** (Demonstrates templating benefit)

**Phase 1 - SearchContent:**

```json
{
  "results": [
    {
      "identifier": "lab_desk_wall_bin_c1",
      "title": "Lab Desk Wall Bin C1",
      "fragment": "Lab Desk Wall Bin C1\n\nInventory container for small electronics..."
    }
  ]
}
```

**Phase 2 - ReadPage:**

```json
{
  "front_matter_toml": "container_type = 'wall_bin'\ncategory = 'Storage'",
  "rendered_content_html": "<h1>Lab Desk Wall Bin C1</h1>\n<p>Inventory container for small electronics.</p>\n<h2>Contents</h2>\n<ul>\n<li><a href=\"/cr2032_batteries\">CR2032 Batteries</a></li>\n<li><a href=\"/cr2025_batteries\">CR2025 Batteries</a></li>\n<li><a href=\"/led_assortment\">LED Assortment</a></li>\n</ul>"
}
```

**Note**: The "Contents" list was **generated by the templating system** based on the inventory index (pages with `inventory.container = 'lab_desk_wall_bin_c1'`). This information doesn't exist in the raw markdown—it's dynamically constructed. By providing rendered HTML, the LLM sees the final processed output.

**Gemini Can Answer**: "What's in bin C1?" → "Bin C1 contains CR2032 batteries, CR2025 batteries, and an LED assortment."

---

**Query: "space navigator"**

**Phase 1 - SearchContent:**

```json
{
  "results": [
    {
      "identifier": "3dconnexion_space_navigator",
      "title": "3Dconnexion Space Navigator", 
      "fragment": "3Dconnexion Space Navigator\n\n Goes in: Lab Desk Wall Bin L1..."
    }
  ]
}
```

**Phase 2 - ReadPage:**

```json
{
  "front_matter_toml": "location = 'Lab Desk Wall Bin L1 (Big Yellow)'\ncategory = 'Hardware'\nsubcategory = 'Input Devices'\ntags = ['3d', 'cad', 'navigation']\ncondition = 'excellent'",
  "rendered_content_html": "<h1>3Dconnexion Space Navigator</h1>\n<p>3D navigation device for CAD and 3D modeling.</p>\n<h2>Product Information</h2>\n<ul>\n<li>Product Name: SpaceNavigator</li>\n<li>Manufacturer: 3Dconnexion</li>\n<li>Model: 3DX-700028</li>\n<li>Purchase Date: 2023-06-12</li>\n<li>Warranty: 2 years (expires 2025-06-12)</li>\n</ul>\n<h2>Features</h2>\n<ul>\n<li>6 degrees of freedom navigation</li>\n<li>USB connectivity (USB-A)</li>\n<li>Plug and play - no drivers needed on Linux</li>\n<li>Compatible with Blender, FreeCAD, Fusion 360</li>\n</ul>\n<h2>Usage Notes</h2>\n<p>Works great with Blender. Minor configuration needed for FreeCAD (enable 3D mouse in preferences).</p>"
}
```

**Gemini Can Answer:**

- "Where is my Space Navigator?" → "Lab Desk Wall Bin L1"
- "What's the warranty on my Space Navigator?" → "2 years, expires June 2025"
- "Does my Space Navigator work with FreeCAD?" → "Yes, but you need to enable 3D mouse in preferences"
- "When did I buy the Space Navigator?" → "June 12, 2023"

**Fragment would only contain** location. **Rendered HTML provides complete context** for comprehensive Q&A.

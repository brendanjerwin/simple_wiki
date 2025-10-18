# Voice Assistant API Specification

Quick reference for all APIs and integrations needed for voice assistant implementation.

## Backend API Extension

### Proto Definition Change

**File**: `api/proto/api/v1/page_management.proto`

**Current**:

```protobuf
message ReadPageResponse {
  string content_markdown = 1;         // Raw markdown, no template processing
  string front_matter_toml = 2;        // TOML frontmatter
  string rendered_content_html = 3;    // Templates applied + HTML conversion
}
```text

**Required Addition**:

```protobuf
message ReadPageResponse {
  string content_markdown = 1;
  string front_matter_toml = 2;
  string rendered_content_html = 3;
  string rendered_content_markdown = 4;  // ← NEW: Templates applied, markdown format
}
```text

### Implementation Notes

The backend templating system already:

1. Reads raw markdown
2. Applies templates (adds inventory lists, related pages, etc.)
3. Converts to HTML

Just need to capture step 2 output before HTML conversion and return it in the new field.

**Token Efficiency**:

- `content_markdown`: ~50 tokens (missing template content)
- `rendered_content_markdown`: ~85 tokens (complete, efficient) ← **Use this**
- `rendered_content_html`: ~160 tokens (complete, inefficient)

---

## Android Platform APIs

### 1. AICore System Service

**Purpose**: Access to on-device and cloud Gemini models

**Package**: `com.google.android.aicore`

**Requirements**:

- Android 14+
- Pixel 8 Pro, Pixel 8, or Pixel 8a (current support)
- Developer options enabled
- AICore enabled in system settings

**Basic Usage**:

```kotlin
import com.google.android.aicore.GenerativeModel
import com.google.android.aicore.GenerateContentConfig

val geminiClient = GenerativeModel(
    modelName = "gemini",  // Platform chooses Nano or cloud
    apiKey = null          // Not needed for on-device
)

val response = geminiClient.generateContent(
    prompt = userPrompt,
    config = config
)
```text

**Documentation**: <https://developer.android.com/ai/gemini-nano>

---

### 2. App Actions (Built-in Intents)

**Purpose**: Enable voice commands via Google Assistant

**Configuration File**: `res/xml/shortcuts.xml`

**Example**:

```xml
<?xml version="1.0" encoding="utf-8"?>
<shortcuts xmlns:android="http://schemas.android.com/apk/res/android">
  <capability android:name="actions.intent.SEARCH">
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
```text

**Manifest Declaration**:

```xml
<application>
  <meta-data
    android:name="com.google.android.actions"
    android:resource="@xml/shortcuts" />
</application>
```text

**User Invocation**:

```text
"Hey Google, search my wiki for batteries"
"Hey Google, find Space Navigator in my wiki"
```text

**Documentation**:

- <https://developer.android.com/reference/app-actions/built-in-intents>
- <https://codelabs.developers.google.com/codelabs/appactions/>

---

### 3. Gemini Function Calling

**Purpose**: Allow Gemini to invoke wiki search as a tool

**Function Declaration**:

```kotlin
import com.google.android.aicore.FunctionDeclaration
import com.google.android.aicore.Tool

val searchWikiFunction = FunctionDeclaration(
    name = "search_wiki",
    description = "Search the personal wiki for information about items, locations, and inventory",
    parameters = jsonObject {
        put("type", "object")
        put("properties", jsonObject {
            put("query", jsonObject {
                put("type", "string")
                put("description", "The search query extracted from the user's question")
            })
        })
        put("required", jsonArray("query"))
    }
)

val tools = listOf(Tool(functionDeclarations = listOf(searchWikiFunction)))
```text

**Configure Client**:

```kotlin
val config = GenerateContentConfig(
    tools = tools,
    systemInstruction = """
        You are a helpful assistant that answers questions about a personal wiki.
        Use the search_wiki function to find relevant information.
        Only answer based on the search results provided.
        If information is not found, say "I don't have that information in the wiki."
    """.trimIndent()
)

val response = geminiClient.generateContent(
    prompt = userQuery,
    config = config
)
```text

**Handle Function Call**:

```kotlin
when (val part = response.candidates[0].content.parts[0]) {
    is FunctionCallPart -> {
        val functionName = part.functionCall.name  // "search_wiki"
        val args = part.functionCall.args          // {"query": "batteries"}
        
        // Execute the search
        val searchResults = performWikiSearch(args["query"] as String)
        
        // Return results to Gemini for final response
        val finalResponse = geminiClient.generateContent(
            prompt = FunctionResponsePart(
                functionName = functionName,
                response = searchResults
            ),
            config = config
        )
        
        return finalResponse.text  // Spoken response
    }
}
```text

**Documentation**: <https://ai.google.dev/gemini-api/docs/function-calling>

---

### 4. Capacitor Native Bridge

**Purpose**: Enable web app to call native Android functionality

**Installation**:

```bash
npm install @capacitor/core @capacitor/android
npx cap init
npx cap add android
```text

**Plugin Definition** (`WikiSearchPlugin.kt`):

```kotlin
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

@CapacitorPlugin(name = "WikiSearch")
class WikiSearchPlugin : Plugin() {
    
    @PluginMethod
    fun searchAndRespond(call: PluginCall) {
        val query = call.getString("query") ?: run {
            call.reject("Missing query parameter")
            return
        }
        
        // Phase 1: Search via gRPC
        val searchResults = grpcClient.searchContent(query)
        
        // Phase 2: Fetch page content
        val page = grpcClient.readPage(searchResults.first().identifier)
        
        // Phase 3: LLM reasoning
        val context = buildContext(page)
        val response = geminiClient.generateResponse(query, context)
        
        call.resolve(JSObject().apply {
            put("response", response.text)
            put("success", true)
        })
    }
}
```text

**Register Plugin** (`MainActivity.kt`):

```kotlin
import com.getcapacitor.BridgeActivity

class MainActivity : BridgeActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        registerPlugin(WikiSearchPlugin::class.java)
        super.onCreate(savedInstanceState)
    }
}
```text

**Call from Web** (JavaScript):

```typescript
import { Plugins } from '@capacitor/core';
const { WikiSearch } = Plugins;

async function handleVoiceQuery(query: string) {
  const result = await WikiSearch.searchAndRespond({ query });
  console.log('Response:', result.response);
  return result.response;
}
```text

**Documentation**: <https://capacitorjs.com/docs/plugins/tutorial/android-implementation>

---

## Data Flow

```text
User Voice Command
      ↓
Google Assistant (recognizes via App Actions)
      ↓
Android Intent → MainActivity
      ↓
WikiSearchPlugin.searchAndRespond()
      ↓
┌─────────────────────────────────────┐
│ Phase 1: Search                     │
│ → grpcClient.searchContent(query)   │
│ ← [page identifiers]                │
└─────────────────────────────────────┘
      ↓
┌─────────────────────────────────────┐
│ Phase 2: Fetch Content              │
│ → grpcClient.readPage(id)           │
│ ← {                                 │
│     rendered_content_markdown,      │ ← NEW FIELD
│     front_matter_toml               │
│   }                                 │
└─────────────────────────────────────┘
      ↓
┌─────────────────────────────────────┐
│ Phase 3: LLM Reasoning              │
│ → geminiClient.generateContent()    │
│   with system prompt + page context │
│ ← "Your batteries are in bin C1"    │
└─────────────────────────────────────┘
      ↓
Text-to-Speech
      ↓
User hears response
```text

---

## Testing APIs

### Mock Data for Testing

**Sample ReadPageResponse**:

```json
{
  "content_markdown": "# CR2032 Batteries\n\nButton cell batteries.",
  "front_matter_toml": "location = 'Lab Desk Wall Bin C1'\ncategory = 'Electronics'",
  "rendered_content_html": "<h1>CR2032 Batteries</h1><p>Button cell batteries.</p>",
  "rendered_content_markdown": "# CR2032 Batteries\n\nButton cell batteries.\n\n## Location\nLab Desk Wall Bin C1"
}
```text

### Test Function Calling

```kotlin
@Test
fun `test search wiki function call`() {
    val functionCall = FunctionCallPart(
        functionName = "search_wiki",
        args = mapOf("query" to "batteries")
    )
    
    // Simulate Gemini invoking the function
    val result = handleFunctionCall(functionCall)
    
    assertNotNull(result)
    assertTrue(result.contains("CR2032"))
}
```text

### Test End-to-End

```kotlin
@Test
fun `test voice query end to end`() {
    // Simulate App Action intent
    val intent = Intent(Intent.ACTION_VIEW).apply {
        putExtra("searchQuery", "where are my batteries")
    }
    
    // Process through plugin
    val response = wikiSearchPlugin.handleIntent(intent)
    
    assertTrue(response.contains("bin C1") || response.contains("Bin C1"))
}
```text

---

## Security & Privacy

### Tailscale Requirements

All gRPC API calls MUST route through Tailscale network:

```kotlin
// In gRPC client configuration
val channel = ManagedChannelBuilder
    .forAddress("simple-wiki.tailnet-xyz.ts.net", 50051)  // Tailscale hostname
    .useTransportSecurity()  // TLS required
    .build()
```text

**Never**:

- Expose APIs to public internet
- Allow unauthenticated access
- Store Tailscale keys in version control

### Data Privacy

- Voice queries processed locally or via Google cloud (user's choice via device settings)
- Wiki content never leaves Tailscale network
- LLM sees only fetched pages, not entire wiki
- No telemetry or analytics sent to third parties

---

## Version Compatibility

| Component | Minimum Version | Tested Version |
|-----------|----------------|----------------|
| Android OS | 14.0 | 15.0 |
| Capacitor | 6.0 | 6.1 |
| Kotlin | 1.9 | 2.0 |
| gRPC | Already in project | - |
| Tailscale | Any | Latest |

---

## Next Steps for Implementation

1. **Backend**: Add `rendered_content_markdown` field to proto (2-4 hours)
2. **Test**: Verify template content appears in new field (1 hour)
3. **Android**: Set up Capacitor project structure (2-3 hours)
4. **Plugin**: Implement WikiSearchPlugin (1-2 days)
5. **Integration**: Wire up App Actions + function calling (2-3 days)
6. **Testing**: End-to-end voice queries (1-2 days)

**Total Estimate**: 1-2 weeks

---

## References

- [Android AICore/Gemini Nano](https://developer.android.com/ai/gemini-nano)
- [App Actions Documentation](https://developer.android.com/reference/app-actions/built-in-intents)
- [App Actions Codelab](https://codelabs.developers.google.com/codelabs/appactions/)
- [Gemini Function Calling](https://ai.google.dev/gemini-api/docs/function-calling)
- [Capacitor Android Plugins](https://capacitorjs.com/docs/plugins/tutorial/android-implementation)
- [Capacitor Core API](https://capacitorjs.com/docs/core-apis)
- [gRPC Kotlin Guide](https://grpc.io/docs/languages/kotlin/)

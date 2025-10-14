package com.github.brendanjerwin.simple_wiki.voice

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.util.Log
import api.v1.PageManagementServiceClient
import api.v1.SearchServiceClient
import com.connectrpc.ProtocolClientConfig
import com.connectrpc.extensions.GoogleJavaProtobufStrategy
import com.connectrpc.impl.ProtocolClient
import com.connectrpc.okhttp.ConnectOkHttpClient
import com.connectrpc.protocols.NetworkProtocol
import com.github.brendanjerwin.simple_wiki.api.ApiTimeoutException
import com.github.brendanjerwin.simple_wiki.api.ApiUnavailableException
import com.github.brendanjerwin.simple_wiki.api.GrpcWikiApiClient
import com.github.brendanjerwin.simple_wiki.retrieval.SearchOrchestrator
import com.github.brendanjerwin.simple_wiki.retrieval.TokenBudgetManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/**
 * Activity that handles voice queries from Google Assistant.
 *
 * This Activity receives voice search queries via Intent, orchestrates the wiki search,
 * and returns structured results that can be consumed by Gemini for generating responses.
 *
 * Intent handling:
 * - Receives query parameter from Google Assistant
 * - Calls SearchOrchestrator to retrieve wiki context
 * - Formats results as VoiceSearchResult
 * - Returns structured data via Intent extras
 *
 * Error handling:
 * - ApiUnavailableException -> "Could not reach wiki. Check Tailscale connection."
 * - ApiTimeoutException -> "Wiki search timed out. Try again."
 * - PageNotFoundException / Empty results -> "No results found for your query."
 */
class VoiceActionHandler : Activity() {

    companion object {
        private const val TAG = "VoiceActionHandler"
        // Production Tailscale URL - matches capacitor.config.ts production setting
        private const val WIKI_BASE_URL = "https://wiki.monster-orfe.ts.net"
    }

    private lateinit var orchestrator: SearchOrchestrator

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Log.d(TAG, "onCreate: Starting voice action handler")

        // Initialize gRPC client and orchestrator
        Log.d(TAG, "onCreate: Initializing orchestrator")
        orchestrator = createSearchOrchestrator()

        // Extract query from Intent URI or extras
        val query = intent.data?.getQueryParameter("query") ?: intent.getStringExtra("query")
        Log.d(TAG, "onCreate: Intent URI: ${intent.data}")
        Log.d(TAG, "onCreate: Received query: $query")

        if (query == null || query.isEmpty()) {
            Log.w(TAG, "onCreate: No query provided")
            returnError("No search query provided.")
            finish()
            return
        }

        // Execute search in coroutine
        Log.d(TAG, "onCreate: Starting search for query: $query")
        CoroutineScope(Dispatchers.Main).launch {
            try {
                val result = performSearch(query)
                Log.d(TAG, "Search successful: ${result.totalPages} pages found")
                returnSuccess(result)
            } catch (e: ApiUnavailableException) {
                Log.e(TAG, "API unavailable", e)
                returnError("Could not reach wiki. Check Tailscale connection.")
            } catch (e: ApiTimeoutException) {
                Log.e(TAG, "API timeout", e)
                returnError("Wiki search timed out. Try again.")
            } catch (e: Exception) {
                Log.e(TAG, "Unexpected error during search", e)
                returnError("An error occurred while searching. Please try again.")
            } finally {
                finish()
            }
        }
    }

    /**
     * Creates the SearchOrchestrator with all required dependencies.
     *
     * This sets up the gRPC client stack:
     * 1. ConnectOkHttpClient for networking
     * 2. ProtocolClient for Connect RPC
     * 3. Generated service clients (SearchService, PageManagementService)
     * 4. GrpcWikiApiClient wrapper
     * 5. TokenBudgetManager
     * 6. SearchOrchestrator
     *
     * @return Configured SearchOrchestrator ready for queries
     */
    private fun createSearchOrchestrator(): SearchOrchestrator {
        // Create Connect RPC protocol client
        val protocolClient = ProtocolClient(
            httpClient = ConnectOkHttpClient(),
            config = ProtocolClientConfig(
                host = WIKI_BASE_URL,
                serializationStrategy = GoogleJavaProtobufStrategy(),
                networkProtocol = NetworkProtocol.GRPC_WEB
            )
        )

        // Create gRPC service clients
        val searchService = SearchServiceClient(protocolClient)
        val pageService = PageManagementServiceClient(protocolClient)

        // Create API client wrapper
        val apiClient = GrpcWikiApiClient(searchService, pageService)

        // Create token budget manager (default 100k tokens)
        val budgetManager = TokenBudgetManager()

        // Create and return orchestrator
        return SearchOrchestrator(apiClient, budgetManager)
    }

    /**
     * Performs the wiki search using the orchestrator.
     *
     * @param query The search query
     * @return VoiceSearchResult containing the search results
     */
    private suspend fun performSearch(query: String): VoiceSearchResult = withContext(Dispatchers.IO) {
        val wikiContext = orchestrator.search(query)

        // Convert WikiContext to VoiceSearchResult
        val pages = wikiContext.pages.map { pageContext ->
            PageSummary(
                title = pageContext.title,
                identifier = pageContext.identifier,
                frontmatterJson = convertFrontmatterToJson(pageContext.frontmatter),
                renderedMarkdown = pageContext.renderedMarkdown
            )
        }

        VoiceSearchResult(
            success = true,
            pages = pages,
            totalPages = wikiContext.pages.size,
            error = null
        )
    }

    /**
     * Converts frontmatter Map to JSON string for LLM consumption.
     *
     * @param frontmatter The frontmatter map
     * @return JSON string representation
     */
    private fun convertFrontmatterToJson(frontmatter: Map<String, Any>): String {
        val json = JSONObject()
        frontmatter.forEach { (key, value) ->
            when (value) {
                is List<*> -> {
                    val array = JSONArray()
                    value.forEach { item ->
                        array.put(item)
                    }
                    json.put(key, array)
                }
                else -> json.put(key, value)
            }
        }
        return json.toString()
    }

    /**
     * Returns success result via Intent extras.
     *
     * @param result The voice search result
     */
    private fun returnSuccess(result: VoiceSearchResult) {
        val resultIntent = Intent().apply {
            putExtra("success", result.success)
            putExtra("totalPages", result.totalPages)

            // Convert pages to ArrayList of Bundles
            val pagesBundles = ArrayList<Bundle>()
            result.pages.forEach { page ->
                val bundle = Bundle().apply {
                    putString("title", page.title)
                    putString("identifier", page.identifier)
                    putString("frontmatter", page.frontmatterJson)
                    putString("content", page.renderedMarkdown)
                }
                pagesBundles.add(bundle)
            }
            putParcelableArrayListExtra("pages", pagesBundles)
        }

        setResult(RESULT_OK, resultIntent)
    }

    /**
     * Returns error result via Intent extras.
     *
     * @param errorMessage The user-friendly error message
     */
    private fun returnError(errorMessage: String) {
        val resultIntent = Intent().apply {
            putExtra("success", false)
            putExtra("totalPages", 0)
            putParcelableArrayListExtra("pages", ArrayList<Bundle>())
            putExtra("error", errorMessage)
        }

        setResult(RESULT_OK, resultIntent)
    }
}

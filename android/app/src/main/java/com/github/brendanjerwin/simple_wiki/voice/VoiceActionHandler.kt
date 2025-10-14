package com.github.brendanjerwin.simple_wiki.voice

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import com.github.brendanjerwin.simple_wiki.api.ApiTimeoutException
import com.github.brendanjerwin.simple_wiki.api.ApiUnavailableException
import com.github.brendanjerwin.simple_wiki.retrieval.SearchOrchestrator
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
 *
 * @property orchestrator The search orchestrator for executing queries
 */
class VoiceActionHandler(
    private val orchestrator: SearchOrchestrator
) : Activity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Extract query from Intent
        val query = intent.getStringExtra("query")

        if (query == null || query.isEmpty()) {
            returnError("No search query provided.")
            finish()
            return
        }

        // Execute search in coroutine
        CoroutineScope(Dispatchers.Main).launch {
            try {
                val result = performSearch(query)
                returnSuccess(result)
            } catch (e: ApiUnavailableException) {
                returnError("Could not reach wiki. Check Tailscale connection.")
            } catch (e: ApiTimeoutException) {
                returnError("Wiki search timed out. Try again.")
            } catch (e: Exception) {
                returnError("An error occurred while searching. Please try again.")
            } finally {
                finish()
            }
        }
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

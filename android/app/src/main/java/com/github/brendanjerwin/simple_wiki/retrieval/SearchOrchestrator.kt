package com.github.brendanjerwin.simple_wiki.retrieval

import android.util.Log
import com.github.brendanjerwin.simple_wiki.api.WikiApiClient
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope

/**
 * Orchestrates the two-phase retrieval pattern for wiki queries.
 *
 * This class implements the search → fetch flow:
 * 1. Search for relevant pages using the query
 * 2. Fetch full page content for top N results in parallel
 * 3. Apply token budget management to fit within context window
 * 4. Handle partial failures gracefully
 *
 * @property apiClient The wiki API client for making requests
 * @property budgetManager Token budget manager for enforcing limits
 */
class SearchOrchestrator(
    private val apiClient: WikiApiClient,
    private val budgetManager: TokenBudgetManager
) {
    companion object {
        private const val TAG = "SearchOrchestrator"
    }
    /**
     * Executes a two-phase search and retrieval for the given query.
     *
     * Phase 1: Search for pages matching the query
     * Phase 2: Fetch full content for top results in parallel, within token budget
     *
     * @param query The search query
     * @param maxResults Maximum number of search results to consider (default: 10)
     * @return WikiContext containing the fetched pages and metadata
     * @throws WikiApiException if the operation fails completely
     */
    suspend fun search(query: String, maxResults: Int = 10): WikiContext = coroutineScope {
        // Phase 1: Search for pages
        Log.d(TAG, "search: Starting search for query: $query")
        val searchResults = apiClient.searchContent(query)
        Log.d(TAG, "search: Found ${searchResults.size} search results")

        if (searchResults.isEmpty()) {
            return@coroutineScope WikiContext(emptyList(), 0, false)
        }

        // Limit to maxResults
        val limitedResults = searchResults.take(maxResults)
        Log.d(TAG, "search: Fetching full content for ${limitedResults.size} pages")

        // Phase 2: Fetch pages in parallel
        val fetchJobs = limitedResults.map { result ->
            async {
                try {
                    val pageResponse = apiClient.readPage(result.identifier)

                    // Parse frontmatter TOML into flexible key-value map
                    val frontmatter = parseFrontmatter(pageResponse.frontMatterToml)

                    // Extract title from search result or frontmatter
                    val title = result.title.ifEmpty {
                        frontmatter["title"]?.toString() ?: result.identifier
                    }

                    val renderedMarkdown = pageResponse.renderedContentMarkdown
                    val tokenCount = budgetManager.estimateTokenCount(renderedMarkdown)

                    PageContext(
                        identifier = result.identifier,
                        title = title,
                        frontmatter = frontmatter,
                        renderedMarkdown = renderedMarkdown,
                        tokenCount = tokenCount
                    )
                } catch (e: Exception) {
                    // Swallow individual page fetch failures - partial results are acceptable
                    Log.w(TAG, "search: Failed to fetch page ${result.identifier}: ${e.message}", e)
                    null
                }
            }
        }

        // Wait for all fetches to complete
        val fetchedPages = fetchJobs.mapNotNull { it.await() }
        Log.d(TAG, "search: Successfully fetched ${fetchedPages.size} pages")

        // Phase 3: Apply token budget
        val (includedPages, truncated) = budgetManager.filterPagesWithinBudget(fetchedPages)
        Log.d(TAG, "search: After budget filtering: ${includedPages.size} pages (truncated=$truncated)")

        // Calculate total tokens
        val totalTokens = includedPages.sumOf { it.tokenCount }

        WikiContext(
            pages = includedPages,
            totalTokens = totalTokens,
            truncated = truncated
        )
    }

    /**
     * Parses TOML frontmatter into flexible key-value map.
     *
     * Handles common TOML patterns from the wiki backend:
     * - Simple key = "value" pairs → String values
     * - Arrays: tags = ["one", "two", "three"] → List<String> values
     * - ISO 8601 timestamps: created = "2025-01-10T14:30:00Z" → String values
     *
     * Note: This is a pragmatic parser for the specific TOML format returned by
     * the Go backend (using pelletier/go-toml/v2). It handles the common cases we
     * actually encounter rather than supporting the full TOML 1.0 spec.
     *
     * Following the proto pattern of keeping frontmatter flexible and untyped,
     * this returns Map<String, Any> where values can be String or List<String>.
     *
     * Limitations documented but acceptable for production:
     * - Nested tables/inline tables not currently needed
     * - Multiline strings not used in frontmatter
     * - Complex data types can be added when needed
     *
     * @param toml The TOML frontmatter string from the backend
     * @return Map of key-value pairs with String or List<String> values
     */
    private fun parseFrontmatter(toml: String): Map<String, Any> {
        if (toml.isBlank()) {
            return emptyMap()
        }

        return try {
            val result = mutableMapOf<String, Any>()

            toml.lines().forEach { line ->
                val trimmed = line.trim()
                if (trimmed.isEmpty() || trimmed.startsWith("#")) {
                    return@forEach
                }

                val parts = trimmed.split("=", limit = 2)
                if (parts.size != 2) {
                    return@forEach
                }

                val key = parts[0].trim()
                val value = parts[1].trim()

                // Check if value is an array: ["value1", "value2"]
                if (value.startsWith("[") && value.endsWith("]")) {
                    val arrayContent = value.removeSurrounding("[", "]").trim()
                    if (arrayContent.isEmpty()) {
                        result[key] = emptyList<String>()
                    } else {
                        val items = arrayContent.split(",").mapNotNull { item ->
                            val cleanItem = item.trim().removeSurrounding("\"")
                            if (cleanItem.isNotEmpty()) cleanItem else null
                        }
                        result[key] = items
                    }
                } else {
                    // Simple string value
                    result[key] = value.removeSurrounding("\"")
                }
            }

            result
        } catch (e: Exception) {
            // If parsing fails, return empty map rather than crashing
            // This gracefully handles malformed TOML
            emptyMap()
        }
    }
}

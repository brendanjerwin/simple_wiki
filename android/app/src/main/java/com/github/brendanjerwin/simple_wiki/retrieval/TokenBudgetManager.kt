package com.github.brendanjerwin.simple_wiki.retrieval

/**
 * Manages token budget for LLM context windows.
 *
 * This class is responsible for:
 * - Estimating token counts for text content
 * - Enforcing token budget limits (4K-8K range)
 * - Reserving tokens for system prompts and LLM responses
 * - Determining which pages fit within the available budget
 *
 * Token Budget Breakdown:
 * - Total: 4K-8K tokens (configurable)
 * - Reserved for prompt: 200 tokens
 * - Reserved for response: 500 tokens
 * - Available for page content: Total - 700 tokens
 *
 * @property maxTokens Maximum total tokens available (default: 4000, minimum safe value)
 * @property promptReserveTokens Tokens reserved for system prompt (default: 200)
 * @property responseReserveTokens Tokens reserved for LLM response (default: 500)
 */
class TokenBudgetManager(
    private val maxTokens: Int = 4000,
    private val promptReserveTokens: Int = 200,
    private val responseReserveTokens: Int = 500
) {
    /**
     * Calculates available tokens for page content after reserving tokens for prompt and response.
     */
    val availableTokens: Int
        get() = maxTokens - promptReserveTokens - responseReserveTokens

    /**
     * Estimates the token count for a given text string.
     *
     * Uses a simple heuristic: 1 token ≈ 4 characters.
     * This is a rough approximation suitable for budget enforcement.
     *
     * @param text The text to estimate tokens for
     * @return Estimated token count
     */
    fun estimateTokenCount(text: String): Int {
        if (text.isEmpty()) {
            return 0
        }
        return text.length / 4
    }

    /**
     * Filters a list of pages to fit within the available token budget.
     *
     * Takes pages in priority order and includes as many as possible
     * without exceeding the available token budget.
     *
     * @param pages List of pages to filter, in priority order (highest priority first)
     * @return Pair of (pages that fit within budget, whether truncation occurred)
     */
    fun filterPagesWithinBudget(pages: List<PageContext>): Pair<List<PageContext>, Boolean> {
        if (pages.isEmpty()) {
            return Pair(emptyList(), false)
        }

        val included = mutableListOf<PageContext>()
        var tokensUsed = 0

        for (page in pages) {
            val newTotal = tokensUsed + page.tokenCount
            if (newTotal <= availableTokens) {
                included.add(page)
                tokensUsed = newTotal
            } else {
                // Can't fit this page, truncation occurred
                return Pair(included, true)
            }
        }

        // All pages fit within budget
        return Pair(included, false)
    }
}

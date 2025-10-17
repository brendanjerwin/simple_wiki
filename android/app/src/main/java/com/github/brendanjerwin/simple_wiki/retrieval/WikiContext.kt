package com.github.brendanjerwin.simple_wiki.retrieval

/**
 * Represents the complete wiki context for an LLM query.
 *
 * This data class aggregates multiple wiki pages and metadata about the retrieval process,
 * providing everything needed to construct an LLM prompt for answering user questions.
 *
 * @property pages List of wiki pages that were fetched and included in the context
 * @property totalTokens Total token count across all pages
 * @property truncated True if additional pages were available but excluded due to token budget
 */
data class WikiContext(
    val pages: List<PageContext>,
    val totalTokens: Int,
    val truncated: Boolean
)

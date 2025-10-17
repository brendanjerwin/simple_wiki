package com.github.brendanjerwin.simple_wiki.voice

/**
 * Represents a summarized wiki page for voice action response.
 *
 * This data class provides a condensed view of a wiki page optimized for
 * passing to Gemini through App Actions. It includes the essential information
 * needed for LLM context: title, identifier, frontmatter (as JSON), and
 * rendered markdown content.
 *
 * @property title The page title
 * @property identifier The unique page identifier
 * @property frontmatterJson The frontmatter as JSON string for structured data
 * @property renderedMarkdown The template-expanded markdown content
 */
data class PageSummary(
    val title: String,
    val identifier: String,
    val frontmatterJson: String,
    val renderedMarkdown: String
)

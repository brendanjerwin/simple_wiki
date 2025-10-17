package com.github.brendanjerwin.simple_wiki.retrieval

/**
 * Represents a single wiki page with its content and metadata for LLM context.
 *
 * This data class encapsulates all the information needed from a wiki page
 * to provide context to an LLM for answering user queries.
 *
 * @property identifier The unique identifier of the page
 * @property title The page title
 * @property frontmatter The parsed TOML frontmatter as flexible key-value map
 * @property renderedMarkdown The template-expanded markdown content (from rendered_content_markdown field)
 * @property tokenCount Estimated token count for this page's content
 */
data class PageContext(
    val identifier: String,
    val title: String,
    val frontmatter: Map<String, Any>,
    val renderedMarkdown: String,
    val tokenCount: Int
)

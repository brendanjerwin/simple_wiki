package com.github.brendanjerwin.simple_wiki.api

import api.v1.PageManagement
import api.v1.Search

/**
 * Client interface for interacting with the Simple Wiki API.
 *
 * This interface provides high-level methods for searching and reading wiki pages,
 * abstracting away the underlying gRPC implementation details.
 */
interface WikiApiClient {
    /**
     * Searches the wiki content and returns matching results.
     *
     * @param query The search query string
     * @return List of search results
     * @throws WikiApiException if the search fails
     */
    suspend fun searchContent(query: String): List<Search.SearchResult>

    /**
     * Reads a wiki page by its identifier.
     *
     * @param identifier The page identifier
     * @return The page data including frontmatter and rendered content
     * @throws WikiApiException if the page cannot be read
     */
    suspend fun readPage(identifier: String): PageManagement.ReadPageResponse
}

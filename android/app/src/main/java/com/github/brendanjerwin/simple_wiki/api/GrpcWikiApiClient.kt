package com.github.brendanjerwin.simple_wiki.api

import api.v1.PageManagement
import api.v1.PageManagementServiceClientInterface
import api.v1.Search
import api.v1.SearchServiceClientInterface
import api.v1.searchContentRequest
import com.connectrpc.Code
import com.connectrpc.ConnectException

/**
 * gRPC implementation of the WikiApiClient interface.
 *
 * This class wraps the generated gRPC service clients and provides
 * a cleaner, more domain-focused API for interacting with the wiki.
 *
 * @param searchService The gRPC search service client
 * @param pageService The gRPC page management service client
 */
class GrpcWikiApiClient(
    private val searchService: SearchServiceClientInterface,
    private val pageService: PageManagementServiceClientInterface
) : WikiApiClient {

    override suspend fun searchContent(query: String): List<Search.SearchResult> {
        try {
            val request = searchContentRequest {
                this.query = query
            }

            val response = searchService.searchContent(request)
            return when (response) {
                is com.connectrpc.ResponseMessage.Success -> response.message.resultsList
                is com.connectrpc.ResponseMessage.Failure -> throw response.cause
            }
        } catch (e: ConnectException) {
            throw mapConnectException(e)
        }
    }

    override suspend fun readPage(identifier: String): PageManagement.ReadPageResponse {
        try {
            val request = PageManagement.ReadPageRequest.newBuilder()
                .setPageName(identifier)
                .build()

            val response = pageService.readPage(request)
            return when (response) {
                is com.connectrpc.ResponseMessage.Success -> response.message
                is com.connectrpc.ResponseMessage.Failure -> throw response.cause
            }
        } catch (e: ConnectException) {
            throw mapConnectException(e)
        }
    }

    private fun mapConnectException(e: ConnectException): WikiApiException {
        return when (e.code) {
            Code.UNAVAILABLE -> ApiUnavailableException(
                "Wiki API service is unavailable",
                e
            )
            Code.DEADLINE_EXCEEDED -> ApiTimeoutException(
                "Request timeout while contacting Wiki API",
                e
            )
            Code.NOT_FOUND -> PageNotFoundException(
                "Requested page was not found",
                e
            )
            else -> WikiApiException(
                "Wiki API error: ${e.message}",
                e
            )
        }
    }
}

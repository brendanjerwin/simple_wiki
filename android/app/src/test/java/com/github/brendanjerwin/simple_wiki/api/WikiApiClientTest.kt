package com.github.brendanjerwin.simple_wiki.api

import api.v1.PageManagement
import api.v1.PageManagementServiceClientInterface
import api.v1.Search
import api.v1.SearchServiceClientInterface
import com.connectrpc.Code
import com.connectrpc.ConnectException
import com.connectrpc.ResponseMessage
import io.mockk.coEvery
import io.mockk.mockk
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.DisplayName
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

@DisplayName("WikiApiClient")
class WikiApiClientTest {

    private lateinit var searchService: SearchServiceClientInterface
    private lateinit var pageService: PageManagementServiceClientInterface
    private lateinit var client: WikiApiClient

    @BeforeEach
    fun setUp() {
        searchService = mockk()
        pageService = mockk()
        client = GrpcWikiApiClient(searchService, pageService)
    }

    @Nested
    @DisplayName("searchContent")
    inner class SearchContent {

        @Nested
        @DisplayName("when search returns results")
        inner class WhenSearchReturnsResults {

            private lateinit var results: List<Search.SearchResult>

            @BeforeEach
            fun arrange() = runTest {
                val mockResults = listOf(
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page1")
                        .setTitle("Test Page 1")
                        .setFragment("This is test content")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page2")
                        .setTitle("Test Page 2")
                        .setFragment("More test content")
                        .build()
                )

                val response = Search.SearchContentResponse.newBuilder()
                    .addAllResults(mockResults)
                    .build()

                coEvery { searchService.searchContent(any(), any()) } returns
                    ResponseMessage.Success(response, emptyMap(), emptyMap())

                results = client.searchContent("test query")
            }

            @Test
            fun `should return list of results`() {
                assertEquals(2, results.size)
            }

            @Test
            fun `should contain first result with correct data`() {
                assertEquals("page1", results[0].identifier)
                assertEquals("Test Page 1", results[0].title)
                assertEquals("This is test content", results[0].fragment)
            }

            @Test
            fun `should contain second result with correct data`() {
                assertEquals("page2", results[1].identifier)
                assertEquals("Test Page 2", results[1].title)
                assertEquals("More test content", results[1].fragment)
            }
        }

        @Nested
        @DisplayName("when search returns no results")
        inner class WhenSearchReturnsNoResults {

            private lateinit var results: List<Search.SearchResult>

            @BeforeEach
            fun arrange() = runTest {
                val response = Search.SearchContentResponse.newBuilder()
                    .build()

                coEvery { searchService.searchContent(any(), any()) } returns
                    ResponseMessage.Success(response, emptyMap(), emptyMap())

                results = client.searchContent("nonexistent query")
            }

            @Test
            fun `should return empty list`() {
                assertTrue(results.isEmpty())
            }
        }

        @Nested
        @DisplayName("when service is unavailable")
        inner class WhenServiceIsUnavailable {

            @BeforeEach
            fun arrange() {
                coEvery { searchService.searchContent(any(), any()) } throws
                    ConnectException(Code.UNAVAILABLE, message = "Service unavailable")
            }

            @Test
            fun `should throw ApiUnavailableException`() {
                val exception = assertThrows(ApiUnavailableException::class.java) {
                    runTest {
                        client.searchContent("test query")
                    }
                }
                assertTrue(exception.message!!.contains("unavailable", ignoreCase = true))
            }
        }

        @Nested
        @DisplayName("when request times out")
        inner class WhenRequestTimesOut {

            @BeforeEach
            fun arrange() {
                coEvery { searchService.searchContent(any(), any()) } throws
                    ConnectException(Code.DEADLINE_EXCEEDED, message = "Deadline exceeded")
            }

            @Test
            fun `should throw ApiTimeoutException`() {
                val exception = assertThrows(ApiTimeoutException::class.java) {
                    runTest {
                        client.searchContent("test query")
                    }
                }
                assertTrue(exception.message!!.contains("timeout", ignoreCase = true))
            }
        }

        @Nested
        @DisplayName("when service returns an error")
        inner class WhenServiceReturnsError {

            @BeforeEach
            fun arrange() {
                coEvery { searchService.searchContent(any(), any()) } throws
                    ConnectException(Code.UNKNOWN, message = "Internal server error")
            }

            @Test
            fun `should throw WikiApiException`() {
                assertThrows(WikiApiException::class.java) {
                    runTest {
                        client.searchContent("test query")
                    }
                }
            }
        }
    }

    @Nested
    @DisplayName("readPage")
    inner class ReadPage {

        @Nested
        @DisplayName("when page exists")
        inner class WhenPageExists {

            private lateinit var response: PageManagement.ReadPageResponse

            @BeforeEach
            fun arrange() = runTest {
                val mockResponse = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# Test Page\n\nContent here")
                    .setFrontMatterToml("title = \"Test Page\"\ntags = [\"test\"]")
                    .setRenderedContentHtml("<h1>Test Page</h1><p>Content here</p>")
                    .setRenderedContentMarkdown("# Test Page\n\nContent here")
                    .build()

                coEvery { pageService.readPage(any(), any()) } returns
                    ResponseMessage.Success(mockResponse, emptyMap(), emptyMap())

                response = client.readPage("test-page")
            }

            @Test
            fun `should return response with content markdown`() {
                assertEquals("# Test Page\n\nContent here", response.contentMarkdown)
            }

            @Test
            fun `should return response with frontmatter TOML`() {
                assertEquals("title = \"Test Page\"\ntags = [\"test\"]", response.frontMatterToml)
            }

            @Test
            fun `should return response with rendered HTML`() {
                assertEquals("<h1>Test Page</h1><p>Content here</p>", response.renderedContentHtml)
            }

            @Test
            fun `should return response with rendered markdown`() {
                assertEquals("# Test Page\n\nContent here", response.renderedContentMarkdown)
            }
        }

        @Nested
        @DisplayName("when page not found")
        inner class WhenPageNotFound {

            @BeforeEach
            fun arrange() {
                coEvery { pageService.readPage(any(), any()) } throws
                    ConnectException(Code.NOT_FOUND, message = "Page not found")
            }

            @Test
            fun `should throw PageNotFoundException`() {
                val exception = assertThrows(PageNotFoundException::class.java) {
                    runTest {
                        client.readPage("nonexistent-page")
                    }
                }
                assertTrue(exception.message!!.contains("not found", ignoreCase = true))
            }
        }

        @Nested
        @DisplayName("when service is unavailable")
        inner class WhenServiceIsUnavailable {

            @BeforeEach
            fun arrange() {
                coEvery { pageService.readPage(any(), any()) } throws
                    ConnectException(Code.UNAVAILABLE, message = "Service unavailable")
            }

            @Test
            fun `should throw ApiUnavailableException`() {
                val exception = assertThrows(ApiUnavailableException::class.java) {
                    runTest {
                        client.readPage("test-page")
                    }
                }
                assertTrue(exception.message!!.contains("unavailable", ignoreCase = true))
            }
        }

        @Nested
        @DisplayName("when request times out")
        inner class WhenRequestTimesOut {

            @BeforeEach
            fun arrange() {
                coEvery { pageService.readPage(any(), any()) } throws
                    ConnectException(Code.DEADLINE_EXCEEDED, message = "Deadline exceeded")
            }

            @Test
            fun `should throw ApiTimeoutException`() {
                val exception = assertThrows(ApiTimeoutException::class.java) {
                    runTest {
                        client.readPage("test-page")
                    }
                }
                assertTrue(exception.message!!.contains("timeout", ignoreCase = true))
            }
        }
    }
}

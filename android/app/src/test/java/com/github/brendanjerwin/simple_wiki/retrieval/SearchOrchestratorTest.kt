package com.github.brendanjerwin.simple_wiki.retrieval

import api.v1.PageManagement
import api.v1.Search
import com.github.brendanjerwin.simple_wiki.api.ApiUnavailableException
import com.github.brendanjerwin.simple_wiki.api.PageNotFoundException
import com.github.brendanjerwin.simple_wiki.api.WikiApiClient
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.DisplayName
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

@DisplayName("SearchOrchestrator")
class SearchOrchestratorTest {

    private lateinit var apiClient: WikiApiClient
    private lateinit var budgetManager: TokenBudgetManager
    private lateinit var orchestrator: SearchOrchestrator

    @BeforeEach
    fun setUp() {
        apiClient = mockk()
        budgetManager = TokenBudgetManager(maxTokens = 4000)
        orchestrator = SearchOrchestrator(apiClient, budgetManager)
    }

    @Nested
    @DisplayName("search")
    inner class SearchMethod {

        @Nested
        @DisplayName("when search returns no results")
        inner class WhenSearchReturnsNoResults {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                coEvery { apiClient.searchContent("nonexistent") } returns emptyList()

                context = orchestrator.search("nonexistent")
            }

            @Test
            fun `should return empty context`() {
                assertTrue(context.pages.isEmpty())
            }

            @Test
            fun `should have zero tokens`() {
                assertEquals(0, context.totalTokens)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(context.truncated)
            }
        }

        @Nested
        @DisplayName("when search returns single result")
        inner class WhenSearchReturnsSingleResult {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                val searchResult = Search.SearchResult.newBuilder()
                    .setIdentifier("test-page")
                    .setTitle("Test Page")
                    .setFragment("Test content")
                    .build()

                coEvery { apiClient.searchContent("test") } returns listOf(searchResult)

                val pageResponse = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# Test Page\n\nContent here")
                    .setFrontMatterToml("title = \"Test Page\"")
                    .setRenderedContentMarkdown("# Test Page\n\nTemplate-expanded content")
                    .build()

                coEvery { apiClient.readPage("test-page") } returns pageResponse

                context = orchestrator.search("test")
            }

            @Test
            fun `should fetch the page`() = runTest {
                coVerify(exactly = 1) { apiClient.readPage("test-page") }
            }

            @Test
            fun `should include one page in context`() {
                assertEquals(1, context.pages.size)
            }

            @Test
            fun `should have correct page identifier`() {
                assertEquals("test-page", context.pages[0].identifier)
            }

            @Test
            fun `should have correct page title`() {
                assertEquals("Test Page", context.pages[0].title)
            }

            @Test
            fun `should use rendered markdown`() {
                assertEquals("# Test Page\n\nTemplate-expanded content", context.pages[0].renderedMarkdown)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(context.truncated)
            }
        }

        @Nested
        @DisplayName("when search returns multiple results")
        inner class WhenSearchReturnsMultipleResults {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                val results: List<Search.SearchResult> = listOf(
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page1")
                        .setTitle("Page 1")
                        .setFragment("Content 1")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page2")
                        .setTitle("Page 2")
                        .setFragment("Content 2")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page3")
                        .setTitle("Page 3")
                        .setFragment("Content 3")
                        .build()
                )

                coEvery { apiClient.searchContent("test") } returns results

                // Mock page responses - small pages within budget
                val pageResponse1 = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# Page 1")
                    .setFrontMatterToml("title = \"Page 1\"")
                    .setRenderedContentMarkdown("# Page 1\n\nContent")
                    .build()

                val pageResponse2 = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# Page 2")
                    .setFrontMatterToml("title = \"Page 2\"")
                    .setRenderedContentMarkdown("# Page 2\n\nContent")
                    .build()

                val pageResponse3 = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# Page 3")
                    .setFrontMatterToml("title = \"Page 3\"")
                    .setRenderedContentMarkdown("# Page 3\n\nContent")
                    .build()

                coEvery { apiClient.readPage("page1") } returns pageResponse1
                coEvery { apiClient.readPage("page2") } returns pageResponse2
                coEvery { apiClient.readPage("page3") } returns pageResponse3

                context = orchestrator.search("test")
            }

            @Test
            fun `should fetch all pages in parallel`() = runTest {
                coVerify(exactly = 1) { apiClient.readPage("page1") }
                coVerify(exactly = 1) { apiClient.readPage("page2") }
                coVerify(exactly = 1) { apiClient.readPage("page3") }
            }

            @Test
            fun `should include all pages in context`() {
                assertEquals(3, context.pages.size)
            }

            @Test
            fun `should maintain search result order`() {
                assertEquals("page1", context.pages[0].identifier)
                assertEquals("page2", context.pages[1].identifier)
                assertEquals("page3", context.pages[2].identifier)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(context.truncated)
            }
        }

        @Nested
        @DisplayName("when pages exceed token budget")
        inner class WhenPagesExceedTokenBudget {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                // Use a smaller budget for testing
                budgetManager = TokenBudgetManager(maxTokens = 1000) // 1000 - 200 - 500 = 300 available
                orchestrator = SearchOrchestrator(apiClient, budgetManager)

                val results: List<Search.SearchResult> = listOf(
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page1")
                        .setTitle("Page 1")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page2")
                        .setTitle("Page 2")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page3")
                        .setTitle("Page 3")
                        .build()
                )

                coEvery { apiClient.searchContent("test") } returns results

                // Each page has 150 tokens, so only 2 will fit in 300 token budget
                val hugePage1 = PageManagement.ReadPageResponse.newBuilder()
                    .setRenderedContentMarkdown("a".repeat(600)) // 150 tokens
                    .setFrontMatterToml("title = \"Page 1\"")
                    .build()

                val hugePage2 = PageManagement.ReadPageResponse.newBuilder()
                    .setRenderedContentMarkdown("b".repeat(600)) // 150 tokens
                    .setFrontMatterToml("title = \"Page 2\"")
                    .build()

                val hugePage3 = PageManagement.ReadPageResponse.newBuilder()
                    .setRenderedContentMarkdown("c".repeat(600)) // 150 tokens
                    .setFrontMatterToml("title = \"Page 3\"")
                    .build()

                coEvery { apiClient.readPage("page1") } returns hugePage1
                coEvery { apiClient.readPage("page2") } returns hugePage2
                coEvery { apiClient.readPage("page3") } returns hugePage3

                context = orchestrator.search("test")
            }

            @Test
            fun `should include first two pages`() {
                assertEquals(2, context.pages.size)
            }

            @Test
            fun `should indicate truncation occurred`() {
                assertTrue(context.truncated)
            }

            @Test
            fun `should not exceed token budget`() {
                assertTrue(context.totalTokens <= budgetManager.availableTokens)
            }
        }

        @Nested
        @DisplayName("when individual page fetch fails")
        inner class WhenIndividualPageFetchFails {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                val results: List<Search.SearchResult> = listOf(
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page1")
                        .setTitle("Page 1")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page2")
                        .setTitle("Page 2")
                        .build(),
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page3")
                        .setTitle("Page 3")
                        .build()
                )

                coEvery { apiClient.searchContent("test") } returns results

                // page1 succeeds
                val page1 = PageManagement.ReadPageResponse.newBuilder()
                    .setRenderedContentMarkdown("Content 1")
                    .setFrontMatterToml("title = \"Page 1\"")
                    .build()
                coEvery { apiClient.readPage("page1") } returns page1

                // page2 fails with PageNotFoundException
                coEvery { apiClient.readPage("page2") } throws PageNotFoundException("Not found")

                // page3 succeeds
                val page3 = PageManagement.ReadPageResponse.newBuilder()
                    .setRenderedContentMarkdown("Content 3")
                    .setFrontMatterToml("title = \"Page 3\"")
                    .build()
                coEvery { apiClient.readPage("page3") } returns page3

                context = orchestrator.search("test")
            }

            @Test
            fun `should include successful pages`() {
                assertEquals(2, context.pages.size)
            }

            @Test
            fun `should include page1`() {
                assertTrue(context.pages.any { it.identifier == "page1" })
            }

            @Test
            fun `should include page3`() {
                assertTrue(context.pages.any { it.identifier == "page3" })
            }

            @Test
            fun `should not include failed page2`() {
                assertFalse(context.pages.any { it.identifier == "page2" })
            }
        }

        @Nested
        @DisplayName("when all page fetches fail")
        inner class WhenAllPageFetchesFail {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                val results: List<Search.SearchResult> = listOf(
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page1")
                        .setTitle("Page 1")
                        .build()
                )

                coEvery { apiClient.searchContent("test") } returns results
                coEvery { apiClient.readPage("page1") } throws PageNotFoundException("Not found")

                context = orchestrator.search("test")
            }

            @Test
            fun `should return empty context`() {
                assertTrue(context.pages.isEmpty())
            }

            @Test
            fun `should have zero tokens`() {
                assertEquals(0, context.totalTokens)
            }
        }

        @Nested
        @DisplayName("when search itself fails")
        inner class WhenSearchItselfFails {

            @BeforeEach
            fun arrange() {
                coEvery { apiClient.searchContent("test") } throws ApiUnavailableException("Service down")
            }

            @Test
            fun `should throw exception`() {
                assertThrows(ApiUnavailableException::class.java) {
                    runTest {
                        orchestrator.search("test")
                    }
                }
            }
        }

        @Nested
        @DisplayName("when respecting maxResults limit")
        inner class WhenRespectingMaxResultsLimit {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                // Return 5 search results
                val results: List<Search.SearchResult> = (1..5).map { i ->
                    Search.SearchResult.newBuilder()
                        .setIdentifier("page$i")
                        .setTitle("Page $i")
                        .build()
                }

                coEvery { apiClient.searchContent("test") } returns results

                // Mock all page responses
                for (i in 1..5) {
                    val page = PageManagement.ReadPageResponse.newBuilder()
                        .setRenderedContentMarkdown("Content $i")
                        .setFrontMatterToml("title = \"Page $i\"")
                        .build()
                    coEvery { apiClient.readPage("page$i") } returns page
                }

                // Request only top 3 results
                context = orchestrator.search("test", maxResults = 3)
            }

            @Test
            fun `should limit to maxResults pages`() {
                assertTrue(context.pages.size <= 3)
            }

            @Test
            fun `should fetch top results first`() {
                assertEquals("page1", context.pages[0].identifier)
            }
        }

        @Nested
        @DisplayName("when parsing complex TOML frontmatter")
        inner class WhenParsingComplexTomlFrontmatter {

            private lateinit var context: WikiContext

            @BeforeEach
            fun arrange() = runTest {
                val searchResult = Search.SearchResult.newBuilder()
                    .setIdentifier("battery-page")
                    .setTitle("")  // Empty title to test fallback to frontmatter
                    .setFragment("CR2032 batteries")
                    .build()

                coEvery { apiClient.searchContent("batteries") } returns listOf(searchResult)

                // Realistic TOML with arrays, dates, and complex structures
                val complexToml = """
                    title = "CR2032 Battery Locations"
                    tags = ["inventory", "electronics", "batteries"]
                    created = "2025-01-10T14:30:00Z"
                    modified = "2025-01-12T09:15:00Z"
                    location = "Lab Wall Bins"
                """.trimIndent()

                val pageResponse = PageManagement.ReadPageResponse.newBuilder()
                    .setContentMarkdown("# CR2032 Battery Locations\n\nBatteries are stored in bin F1")
                    .setFrontMatterToml(complexToml)
                    .setRenderedContentMarkdown("# CR2032 Battery Locations\n\nBatteries are stored in bin F1")
                    .build()

                coEvery { apiClient.readPage("battery-page") } returns pageResponse

                context = orchestrator.search("batteries")
            }

            @Test
            fun `should parse title from TOML frontmatter`() {
                assertEquals("CR2032 Battery Locations", context.pages[0].title)
            }

            @Test
            fun `should parse tags array from TOML`() {
                @Suppress("UNCHECKED_CAST")
                val tags = context.pages[0].frontmatter["tags"] as List<String>
                assertEquals(3, tags.size)
                assertTrue(tags.contains("inventory"))
                assertTrue(tags.contains("electronics"))
                assertTrue(tags.contains("batteries"))
            }

            @Test
            fun `should parse created timestamp`() {
                assertEquals("2025-01-10T14:30:00Z", context.pages[0].frontmatter["created"])
            }

            @Test
            fun `should parse modified timestamp`() {
                assertEquals("2025-01-12T09:15:00Z", context.pages[0].frontmatter["modified"])
            }

            @Test
            fun `should include extra fields in frontmatter map`() {
                assertEquals("Lab Wall Bins", context.pages[0].frontmatter["location"])
            }
        }
    }
}

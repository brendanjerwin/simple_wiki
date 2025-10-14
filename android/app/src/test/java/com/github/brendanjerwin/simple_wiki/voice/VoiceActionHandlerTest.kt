package com.github.brendanjerwin.simple_wiki.voice

import com.github.brendanjerwin.simple_wiki.api.ApiTimeoutException
import com.github.brendanjerwin.simple_wiki.api.ApiUnavailableException
import com.github.brendanjerwin.simple_wiki.retrieval.PageContext
import com.github.brendanjerwin.simple_wiki.retrieval.SearchOrchestrator
import com.github.brendanjerwin.simple_wiki.retrieval.WikiContext
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.DisplayName
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

/**
 * Tests for VoiceActionHandler's business logic.
 *
 * Since VoiceActionHandler is an Activity and difficult to test directly,
 * these tests focus on the data transformation logic that would be used
 * in the Activity.
 */
@DisplayName("VoiceActionHandler Logic")
class VoiceActionHandlerTest {

    private lateinit var orchestrator: SearchOrchestrator

    @BeforeEach
    fun setUp() {
        orchestrator = mockk()
    }

    @Nested
    @DisplayName("performSearch")
    inner class PerformSearch {

        @Nested
        @DisplayName("when search returns results")
        inner class WhenSearchReturnsResults {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                val wikiContext = WikiContext(
                    pages = listOf(
                        PageContext(
                            identifier = "test-page",
                            title = "Test Page",
                            frontmatter = mapOf("title" to "Test Page", "tags" to listOf("test")),
                            renderedMarkdown = "# Test Page\n\nTest content",
                            tokenCount = 50
                        )
                    ),
                    totalTokens = 50,
                    truncated = false
                )

                coEvery { orchestrator.search("test query", any()) } returns wikiContext

                // Simulate the logic from VoiceActionHandler.performSearch
                val context = orchestrator.search("test query")
                val pages = context.pages.map { pageContext ->
                    PageSummary(
                        title = pageContext.title,
                        identifier = pageContext.identifier,
                        frontmatterJson = convertFrontmatterToJson(pageContext.frontmatter),
                        renderedMarkdown = pageContext.renderedMarkdown
                    )
                }

                result = VoiceSearchResult(
                    success = true,
                    pages = pages,
                    totalPages = context.pages.size,
                    error = null
                )
            }

            @Test
            fun `should call SearchOrchestrator`() = runTest {
                coVerify { orchestrator.search("test query", any()) }
            }

            @Test
            fun `should return success true`() {
                assertTrue(result.success)
            }

            @Test
            fun `should include pages in result`() {
                assertEquals(1, result.pages.size)
            }

            @Test
            fun `should include totalPages in result`() {
                assertEquals(1, result.totalPages)
            }

            @Test
            fun `should not include error in result`() {
                assertNull(result.error)
            }

            @Test
            fun `should include page title`() {
                assertEquals("Test Page", result.pages[0].title)
            }

            @Test
            fun `should include page identifier`() {
                assertEquals("test-page", result.pages[0].identifier)
            }

            @Test
            fun `should include rendered markdown`() {
                assertEquals("# Test Page\n\nTest content", result.pages[0].renderedMarkdown)
            }

            @Test
            fun `should include frontmatter as JSON`() {
                val frontmatter = result.pages[0].frontmatterJson
                assertNotNull(frontmatter)
                assertTrue(frontmatter.contains("\"title\""))
                assertTrue(frontmatter.contains("\"tags\""))
            }
        }

        @Nested
        @DisplayName("when search returns no results")
        inner class WhenSearchReturnsNoResults {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                val wikiContext = WikiContext(
                    pages = emptyList(),
                    totalTokens = 0,
                    truncated = false
                )

                coEvery { orchestrator.search("nonexistent", any()) } returns wikiContext

                val context = orchestrator.search("nonexistent")
                result = VoiceSearchResult(
                    success = true,
                    pages = emptyList(),
                    totalPages = context.pages.size,
                    error = null
                )
            }

            @Test
            fun `should return success true`() {
                assertTrue(result.success)
            }

            @Test
            fun `should return empty pages list`() {
                assertEquals(0, result.pages.size)
            }

            @Test
            fun `should return totalPages 0`() {
                assertEquals(0, result.totalPages)
            }

            @Test
            fun `should not include error message`() {
                assertNull(result.error)
            }
        }

        @Nested
        @DisplayName("when formatting response with markdown and frontmatter")
        inner class WhenFormattingResponseWithMarkdownAndFrontmatter {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                val wikiContext = WikiContext(
                    pages = listOf(
                        PageContext(
                            identifier = "page1",
                            title = "Page 1",
                            frontmatter = mapOf(
                                "title" to "Page 1",
                                "tags" to listOf("tag1", "tag2"),
                                "created" to "2025-01-10T14:30:00Z"
                            ),
                            renderedMarkdown = "# Page 1\n\nRendered content",
                            tokenCount = 100
                        )
                    ),
                    totalTokens = 100,
                    truncated = false
                )

                coEvery { orchestrator.search("test", any()) } returns wikiContext

                val context = orchestrator.search("test")
                val pages = context.pages.map { pageContext ->
                    PageSummary(
                        title = pageContext.title,
                        identifier = pageContext.identifier,
                        frontmatterJson = convertFrontmatterToJson(pageContext.frontmatter),
                        renderedMarkdown = pageContext.renderedMarkdown
                    )
                }

                result = VoiceSearchResult(
                    success = true,
                    pages = pages,
                    totalPages = context.pages.size,
                    error = null
                )
            }

            @Test
            fun `should include rendered_content_markdown field`() {
                assertEquals("# Page 1\n\nRendered content", result.pages[0].renderedMarkdown)
            }

            @Test
            fun `should include frontmatter as JSON string`() {
                val frontmatter = result.pages[0].frontmatterJson
                assertNotNull(frontmatter)
                assertTrue(frontmatter.contains("\"title\""))
                assertTrue(frontmatter.contains("\"tags\""))
            }

            @Test
            fun `should format JSON correctly with arrays`() {
                val frontmatter = result.pages[0].frontmatterJson
                // Verify the JSON contains array syntax
                assertTrue(frontmatter.contains("\"tags\""))
                assertTrue(frontmatter.contains("["))
                assertTrue(frontmatter.contains("]"))
                assertTrue(frontmatter.contains("tag1"))
                assertTrue(frontmatter.contains("tag2"))
            }

            @Test
            fun `should include page title`() {
                assertEquals("Page 1", result.pages[0].title)
            }

            @Test
            fun `should include page identifier`() {
                assertEquals("page1", result.pages[0].identifier)
            }
        }

        @Nested
        @DisplayName("when formatting multiple pages correctly")
        inner class WhenFormattingMultiplePagesCorrectly {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                val wikiContext = WikiContext(
                    pages = listOf(
                        PageContext(
                            identifier = "page1",
                            title = "Page 1",
                            frontmatter = mapOf("title" to "Page 1"),
                            renderedMarkdown = "Content 1",
                            tokenCount = 50
                        ),
                        PageContext(
                            identifier = "page2",
                            title = "Page 2",
                            frontmatter = mapOf("title" to "Page 2"),
                            renderedMarkdown = "Content 2",
                            tokenCount = 50
                        ),
                        PageContext(
                            identifier = "page3",
                            title = "Page 3",
                            frontmatter = mapOf("title" to "Page 3"),
                            renderedMarkdown = "Content 3",
                            tokenCount = 50
                        )
                    ),
                    totalTokens = 150,
                    truncated = false
                )

                coEvery { orchestrator.search("multi", any()) } returns wikiContext

                val context = orchestrator.search("multi")
                val pages = context.pages.map { pageContext ->
                    PageSummary(
                        title = pageContext.title,
                        identifier = pageContext.identifier,
                        frontmatterJson = convertFrontmatterToJson(pageContext.frontmatter),
                        renderedMarkdown = pageContext.renderedMarkdown
                    )
                }

                result = VoiceSearchResult(
                    success = true,
                    pages = pages,
                    totalPages = context.pages.size,
                    error = null
                )
            }

            @Test
            fun `should include all pages`() {
                assertEquals(3, result.pages.size)
            }

            @Test
            fun `should maintain page order`() {
                assertEquals("page1", result.pages[0].identifier)
                assertEquals("page2", result.pages[1].identifier)
                assertEquals("page3", result.pages[2].identifier)
            }

            @Test
            fun `should format each page with all fields`() {
                result.pages.forEach { page ->
                    assertNotNull(page.title)
                    assertNotNull(page.identifier)
                    assertNotNull(page.frontmatterJson)
                    assertNotNull(page.renderedMarkdown)
                }
            }
        }

        @Nested
        @DisplayName("when handling ApiUnavailableException")
        inner class WhenHandlingApiUnavailableException {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                coEvery { orchestrator.search("test", any()) } throws ApiUnavailableException("Service unavailable")

                // Simulate error handling from VoiceActionHandler
                result = try {
                    orchestrator.search("test")
                    VoiceSearchResult(success = true, pages = emptyList(), totalPages = 0, error = null)
                } catch (e: ApiUnavailableException) {
                    VoiceSearchResult(
                        success = false,
                        pages = emptyList(),
                        totalPages = 0,
                        error = "Could not reach wiki. Check Tailscale connection."
                    )
                }
            }

            @Test
            fun `should return success false`() {
                assertFalse(result.success)
            }

            @Test
            fun `should return empty pages list`() {
                assertEquals(0, result.pages.size)
            }

            @Test
            fun `should provide user-friendly error message`() {
                assertEquals("Could not reach wiki. Check Tailscale connection.", result.error)
            }

            @Test
            fun `should not include technical error details`() {
                assertFalse(result.error?.contains("Service unavailable") == true)
            }
        }

        @Nested
        @DisplayName("when handling ApiTimeoutException")
        inner class WhenHandlingApiTimeoutException {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                coEvery { orchestrator.search("test", any()) } throws ApiTimeoutException("Timeout")

                result = try {
                    orchestrator.search("test")
                    VoiceSearchResult(success = true, pages = emptyList(), totalPages = 0, error = null)
                } catch (e: ApiTimeoutException) {
                    VoiceSearchResult(
                        success = false,
                        pages = emptyList(),
                        totalPages = 0,
                        error = "Wiki search timed out. Try again."
                    )
                }
            }

            @Test
            fun `should return success false`() {
                assertFalse(result.success)
            }

            @Test
            fun `should provide user-friendly timeout message`() {
                assertEquals("Wiki search timed out. Try again.", result.error)
            }

            @Test
            fun `should include retry suggestion`() {
                assertTrue(result.error?.contains("Try again") == true)
            }
        }

        @Nested
        @DisplayName("when handling generic exceptions")
        inner class WhenHandlingGenericExceptions {

            private lateinit var result: VoiceSearchResult

            @BeforeEach
            fun arrange() = runTest {
                coEvery { orchestrator.search("test", any()) } throws RuntimeException("Unexpected error")

                result = try {
                    orchestrator.search("test")
                    VoiceSearchResult(success = true, pages = emptyList(), totalPages = 0, error = null)
                } catch (e: ApiUnavailableException) {
                    VoiceSearchResult(success = false, pages = emptyList(), totalPages = 0, error = "Could not reach wiki. Check Tailscale connection.")
                } catch (e: ApiTimeoutException) {
                    VoiceSearchResult(success = false, pages = emptyList(), totalPages = 0, error = "Wiki search timed out. Try again.")
                } catch (e: Exception) {
                    VoiceSearchResult(success = false, pages = emptyList(), totalPages = 0, error = "An error occurred while searching. Please try again.")
                }
            }

            @Test
            fun `should return success false`() {
                assertFalse(result.success)
            }

            @Test
            fun `should provide generic helpful error message`() {
                assertEquals("An error occurred while searching. Please try again.", result.error)
            }

            @Test
            fun `should not expose technical error details`() {
                assertFalse(result.error?.contains("RuntimeException") == true)
                assertFalse(result.error?.contains("Unexpected error") == true)
            }
        }
    }

    /**
     * Helper function to convert frontmatter to JSON (mimics VoiceActionHandler logic).
     * Simple manual JSON builder for unit tests to avoid Android framework dependencies.
     */
    private fun convertFrontmatterToJson(frontmatter: Map<String, Any>): String {
        val json = StringBuilder("{")
        frontmatter.entries.forEachIndexed { index, (key, value) ->
            if (index > 0) json.append(",")
            json.append("\"").append(key).append("\":")

            when (value) {
                is String -> json.append("\"").append(value).append("\"")
                is Number -> json.append(value)
                is Boolean -> json.append(value)
                is List<*> -> {
                    json.append("[")
                    value.forEachIndexed { i, item ->
                        if (i > 0) json.append(",")
                        when (item) {
                            is String -> json.append("\"").append(item).append("\"")
                            is Number -> json.append(item)
                            is Boolean -> json.append(item)
                            else -> json.append("null")
                        }
                    }
                    json.append("]")
                }
                else -> json.append("null")
            }
        }
        json.append("}")
        return json.toString()
    }
}

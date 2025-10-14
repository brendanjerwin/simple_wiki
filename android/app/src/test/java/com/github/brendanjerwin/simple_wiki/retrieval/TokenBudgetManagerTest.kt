package com.github.brendanjerwin.simple_wiki.retrieval

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.DisplayName
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

@DisplayName("TokenBudgetManager")
class TokenBudgetManagerTest {

    @Nested
    @DisplayName("estimateTokenCount")
    inner class EstimateTokenCount {

        private lateinit var budgetManager: TokenBudgetManager

        @BeforeEach
        fun setUp() {
            budgetManager = TokenBudgetManager()
        }

        @Nested
        @DisplayName("when given empty string")
        inner class WhenGivenEmptyString {

            private var tokenCount: Int = -1

            @BeforeEach
            fun arrange() {
                tokenCount = budgetManager.estimateTokenCount("")
            }

            @Test
            fun `should return zero tokens`() {
                assertEquals(0, tokenCount)
            }
        }

        @Nested
        @DisplayName("when given 4 character string")
        inner class WhenGiven4CharacterString {

            private var tokenCount: Int = -1

            @BeforeEach
            fun arrange() {
                tokenCount = budgetManager.estimateTokenCount("test")
            }

            @Test
            fun `should return 1 token`() {
                assertEquals(1, tokenCount)
            }
        }

        @Nested
        @DisplayName("when given 100 character string")
        inner class WhenGiven100CharacterString {

            private var tokenCount: Int = -1

            @BeforeEach
            fun arrange() {
                val text = "a".repeat(100)
                tokenCount = budgetManager.estimateTokenCount(text)
            }

            @Test
            fun `should return 25 tokens`() {
                assertEquals(25, tokenCount)
            }
        }

        @Nested
        @DisplayName("when given 400 character string")
        inner class WhenGiven400CharacterString {

            private var tokenCount: Int = -1

            @BeforeEach
            fun arrange() {
                val text = "a".repeat(400)
                tokenCount = budgetManager.estimateTokenCount(text)
            }

            @Test
            fun `should return 100 tokens`() {
                assertEquals(100, tokenCount)
            }
        }
    }

    @Nested
    @DisplayName("availableTokens")
    inner class AvailableTokens {

        @Nested
        @DisplayName("when using default budget (4000 tokens)")
        inner class WhenUsingDefaultBudget {

            private lateinit var budgetManager: TokenBudgetManager
            private var available: Int = -1

            @BeforeEach
            fun arrange() {
                budgetManager = TokenBudgetManager()
                available = budgetManager.availableTokens
            }

            @Test
            fun `should reserve 200 for prompt`() {
                // Available = 4000 - 200 - 500 = 3300
                assertEquals(3300, available)
            }
        }

        @Nested
        @DisplayName("when using custom budget (8000 tokens)")
        inner class WhenUsingCustomBudget {

            private lateinit var budgetManager: TokenBudgetManager
            private var available: Int = -1

            @BeforeEach
            fun arrange() {
                budgetManager = TokenBudgetManager(maxTokens = 8000)
                available = budgetManager.availableTokens
            }

            @Test
            fun `should have more available tokens`() {
                // Available = 8000 - 200 - 500 = 7300
                assertEquals(7300, available)
            }
        }
    }

    @Nested
    @DisplayName("filterPagesWithinBudget")
    inner class FilterPagesWithinBudget {

        private lateinit var budgetManager: TokenBudgetManager

        @BeforeEach
        fun setUp() {
            budgetManager = TokenBudgetManager(maxTokens = 4000)
        }

        @Nested
        @DisplayName("when given empty list")
        inner class WhenGivenEmptyList {

            private lateinit var result: Pair<List<PageContext>, Boolean>

            @BeforeEach
            fun arrange() {
                result = budgetManager.filterPagesWithinBudget(emptyList())
            }

            @Test
            fun `should return empty list`() {
                assertTrue(result.first.isEmpty())
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(result.second)
            }
        }

        @Nested
        @DisplayName("when given single page within budget")
        inner class WhenGivenSinglePageWithinBudget {

            private lateinit var result: Pair<List<PageContext>, Boolean>
            private lateinit var page: PageContext

            @BeforeEach
            fun arrange() {
                page = PageContext(
                    identifier = "page1",
                    title = "Test Page",
                    frontmatter = mapOf("title" to "Test"),
                    renderedMarkdown = "# Test\n\nContent here",
                    tokenCount = 100
                )
                result = budgetManager.filterPagesWithinBudget(listOf(page))
            }

            @Test
            fun `should include the page`() {
                assertEquals(1, result.first.size)
                assertEquals("page1", result.first[0].identifier)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(result.second)
            }
        }

        @Nested
        @DisplayName("when given multiple pages all within budget")
        inner class WhenGivenMultiplePagesAllWithinBudget {

            private lateinit var result: Pair<List<PageContext>, Boolean>
            private lateinit var pages: List<PageContext>

            @BeforeEach
            fun arrange() {
                pages = listOf(
                    PageContext("page1", "Page 1", emptyMap(), "Content 1", 100),
                    PageContext("page2", "Page 2", emptyMap(), "Content 2", 100),
                    PageContext("page3", "Page 3", emptyMap(), "Content 3", 100)
                )
                result = budgetManager.filterPagesWithinBudget(pages)
            }

            @Test
            fun `should include all pages`() {
                assertEquals(3, result.first.size)
            }

            @Test
            fun `should maintain order`() {
                assertEquals("page1", result.first[0].identifier)
                assertEquals("page2", result.first[1].identifier)
                assertEquals("page3", result.first[2].identifier)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(result.second)
            }
        }

        @Nested
        @DisplayName("when pages exceed budget")
        inner class WhenPagesExceedBudget {

            private lateinit var result: Pair<List<PageContext>, Boolean>
            private lateinit var pages: List<PageContext>

            @BeforeEach
            fun arrange() {
                // Available budget is 3300 tokens
                pages = listOf(
                    PageContext("page1", "Page 1", emptyMap(), "Content 1", 2000),
                    PageContext("page2", "Page 2", emptyMap(), "Content 2", 2000),
                    PageContext("page3", "Page 3", emptyMap(), "Content 3", 2000)
                )
                result = budgetManager.filterPagesWithinBudget(pages)
            }

            @Test
            fun `should include first page`() {
                assertTrue(result.first.size >= 1)
                assertEquals("page1", result.first[0].identifier)
            }

            @Test
            fun `should not include all pages`() {
                assertTrue(result.first.size < pages.size)
            }

            @Test
            fun `should indicate truncation occurred`() {
                assertTrue(result.second)
            }
        }

        @Nested
        @DisplayName("when single page exceeds budget")
        inner class WhenSinglePageExceedsBudget {

            private lateinit var result: Pair<List<PageContext>, Boolean>
            private lateinit var page: PageContext

            @BeforeEach
            fun arrange() {
                // Available budget is 3300 tokens
                page = PageContext(
                    identifier = "huge-page",
                    title = "Huge Page",
                    frontmatter = emptyMap(),
                    renderedMarkdown = "a".repeat(50000),
                    tokenCount = 5000
                )
                result = budgetManager.filterPagesWithinBudget(listOf(page))
            }

            @Test
            fun `should return empty list`() {
                assertTrue(result.first.isEmpty())
            }

            @Test
            fun `should indicate truncation occurred`() {
                assertTrue(result.second)
            }
        }

        @Nested
        @DisplayName("when pages fit exactly within budget")
        inner class WhenPagesFitExactlyWithinBudget {

            private lateinit var result: Pair<List<PageContext>, Boolean>
            private lateinit var pages: List<PageContext>

            @BeforeEach
            fun arrange() {
                // Available budget is 3300 tokens
                // Create pages that sum to exactly 3300
                pages = listOf(
                    PageContext("page1", "Page 1", emptyMap(), "Content 1", 1650),
                    PageContext("page2", "Page 2", emptyMap(), "Content 2", 1650)
                )
                result = budgetManager.filterPagesWithinBudget(pages)
            }

            @Test
            fun `should include all pages`() {
                assertEquals(2, result.first.size)
            }

            @Test
            fun `should not indicate truncation`() {
                assertFalse(result.second)
            }
        }
    }
}

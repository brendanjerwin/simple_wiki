package com.github.brendanjerwin.simple_wiki.voice

/**
 * Represents the result of a voice search query.
 *
 * This data class encapsulates the outcome of a voice-triggered wiki search,
 * including either successful results or error information. It's designed to
 * be passed back through an Intent to Google Assistant/Gemini.
 *
 * Success state: success=true, pages populated, error=null
 * Empty state: success=true, pages empty, error=null
 * Error state: success=false, pages empty, error populated
 *
 * @property success Whether the search completed successfully
 * @property pages List of matching wiki pages (empty if no results or error)
 * @property totalPages Total number of pages found (may be > pages.size if truncated)
 * @property error User-friendly error message (null on success)
 */
data class VoiceSearchResult(
    val success: Boolean,
    val pages: List<PageSummary>,
    val totalPages: Int,
    val error: String?
)

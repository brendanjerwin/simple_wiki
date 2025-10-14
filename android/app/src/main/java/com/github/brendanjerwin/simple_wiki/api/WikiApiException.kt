package com.github.brendanjerwin.simple_wiki.api

/**
 * Base exception for all Wiki API errors.
 *
 * @param message The error message
 * @param cause The underlying cause, if any
 */
open class WikiApiException(message: String, cause: Throwable? = null) : Exception(message, cause)

/**
 * Exception thrown when the API server is unavailable or unreachable.
 */
class ApiUnavailableException(message: String, cause: Throwable? = null) : WikiApiException(message, cause)

/**
 * Exception thrown when a requested page is not found.
 */
class PageNotFoundException(message: String, cause: Throwable? = null) : WikiApiException(message, cause)

/**
 * Exception thrown when an API request times out.
 */
class ApiTimeoutException(message: String, cause: Throwable? = null) : WikiApiException(message, cause)

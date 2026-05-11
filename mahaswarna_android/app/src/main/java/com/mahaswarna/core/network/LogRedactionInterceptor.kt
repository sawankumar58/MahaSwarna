package com.mahaswarna.core.network

import okhttp3.Interceptor
import okhttp3.Response

/**
 * Interceptor order position: 4.
 * Strips sensitive headers from request/response before they reach
 * HttpLoggingInterceptor (position 5, debug-only).
 * Redacted headers: Authorization, X-Firebase-Id-Token, X-Integrity-Token.
 */
class LogRedactionInterceptor : Interceptor {
    private val sensitiveHeaders = setOf(
        "authorization",
        "x-firebase-id-token",
        "x-integrity-token",
    )

    override fun intercept(chain: Interceptor.Chain): Response {
        val sanitised = chain.request().newBuilder().apply {
            chain.request().headers.names()
                .filter { it.lowercase() in sensitiveHeaders }
                .forEach { header("$it", "***REDACTED***") }
        }.build()
        return chain.proceed(sanitised)
    }
}

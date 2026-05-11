package com.mahaswarna.core.network

import com.mahaswarna.core.storage.PreferenceStore
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Interceptor order position: 3.
 * Reads AI quota response headers and writes them to [PreferenceStore].
 * Headers: X-Ai-Quota-Used, X-Ai-Quota-Limit, X-Ai-Quota-Reset-At
 * Pass-through: does not modify the request or response.
 */
class AiQuotaInterceptor(
    private val preferenceStore: PreferenceStore,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val response = chain.proceed(chain.request())
        val used    = response.header("X-Ai-Quota-Used")?.toIntOrNull()
        val limit   = response.header("X-Ai-Quota-Limit")?.toIntOrNull()
        val resetAt = response.header("X-Ai-Quota-Reset-At")?.toLongOrNull()
        if (used != null && limit != null && resetAt != null) {
            preferenceStore.setAiQuota(used, limit, resetAt)
        }
        return response
    }
}

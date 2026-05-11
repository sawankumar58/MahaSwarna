package com.mahaswarna.core.network

import okhttp3.Interceptor
import okhttp3.Response

/**
 * Interceptor order position: 1 (outermost).
 * Adds "Accept-Version: v1" to every request.
 * On HTTP 410: throws [ApiError.VersionDeprecated] and emits on [ApiErrorBus]
 * so MainActivity can navigate to UpdateRequiredScreen. Never retried.
 */
class VersionInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request().newBuilder()
            .header("Accept-Version", ApiConstants.API_VERSION)
            .build()
        val response = chain.proceed(request)
        if (response.code == 410) {
            response.close()
            ApiErrorBus.emit(ApiError.VersionDeprecated)
            throw ApiError.VersionDeprecated
        }
        return response
    }
}

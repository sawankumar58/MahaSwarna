package com.mahaswarna.core.network

import retrofit2.HttpException

/** Typed API errors surfaced across the app. */
sealed class ApiError : Exception() {
    /** HTTP 410 — server has deprecated this API version; show UpdateRequiredScreen. */
    data object VersionDeprecated : ApiError()
    /** HTTP 401 after refresh attempt failed — session is fully expired. */
    data object Unauthorized : ApiError()
    /** HTTP 403 body contains "device_not_trusted" — Play Integrity check failed. */
    data object DeviceNotTrusted : ApiError()
    /** HTTP 429 — AI image-search quota exhausted or general rate limit. */
    data object RateLimited : ApiError()
    /** Backend responded but no live rate is available. */
    data object RateUnavailable : ApiError()
    /** 5xx or unexpected server error. */
    data class ServerError(val code: Int) : ApiError()
    /** Any other error (network, parse, etc). */
    data class Unknown(override val cause: Throwable?) : ApiError()
}

object ApiErrorMapper {
    /**
     * Maps a [Throwable] from a Retrofit/OkHttp call to a typed [ApiError].
     * VersionInterceptor throws [ApiError.VersionDeprecated] directly on 410 —
     * this mapper catches it here for consumers that use [mapOrThrow].
     */
    fun map(t: Throwable): ApiError = when {
        t is ApiError -> t
        t is HttpException -> when (t.code()) {
            401 -> ApiError.Unauthorized
            403 -> {
                val body = runCatching { t.response()?.errorBody()?.string() }.getOrNull()
                if (body?.contains("device_not_trusted") == true) ApiError.DeviceNotTrusted
                else if (body?.contains("integrity_token_expired") == true) ApiError.DeviceNotTrusted
                else ApiError.Unknown(t)
            }
            410 -> ApiError.VersionDeprecated
            429 -> ApiError.RateLimited
            503 -> ApiError.RateUnavailable
            in 500..599 -> ApiError.ServerError(t.code())
            else -> ApiError.Unknown(t)
        }
        else -> ApiError.Unknown(t)
    }
}

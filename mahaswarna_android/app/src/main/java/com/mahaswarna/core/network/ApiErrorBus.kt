package com.mahaswarna.core.network

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow

/**
 * Process-wide bus for non-recoverable API errors that require navigation.
 * VersionDeprecated is emitted here by VersionInterceptor (or ApiErrorMapper callers)
 * and observed in MainActivity to navigate to UpdateRequiredScreen.
 *
 * Use tryEmit() from interceptor/coroutine context — no suspension needed.
 */
object ApiErrorBus {
    private val _events = MutableSharedFlow<ApiError>(extraBufferCapacity = 1)
    val events: SharedFlow<ApiError> = _events

    fun emit(error: ApiError) {
        _events.tryEmit(error)
    }
}

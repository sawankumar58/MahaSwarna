package com.mahaswarna.core.network

import com.mahaswarna.core.auth.SessionManager
import com.mahaswarna.core.auth.TokenStore
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Interceptor order position: 2.
 * Attaches Bearer token. On 401: refreshes once (synchronized) then retries.
 * First access to TokenStore triggers Keystore unsealing (50–200ms on budget
 * devices) — absorbed in the background, never on the main thread.
 */
class AuthInterceptor(
    private val tokenStore: TokenStore,
    private val sessionManager: SessionManager,
) : Interceptor {

    private val refreshLock = Any()

    override fun intercept(chain: Interceptor.Chain): Response {
        val token = tokenStore.getAccessToken()
        val request = chain.request().newBuilder()
            .apply { if (token != null) header("Authorization", "Bearer $token") }
            .build()

        val response = chain.proceed(request)
        if (response.code != 401) return response

        // 401 — attempt one refresh under a lock to avoid stampede
        response.close()
        return synchronized(refreshLock) {
            // Re-check: another thread may have refreshed while we waited
            val refreshed = tokenStore.getAccessToken()
            if (refreshed != null && refreshed != token) {
                // Token was refreshed by another thread; retry with new token
                chain.proceed(
                    chain.request().newBuilder()
                        .header("Authorization", "Bearer $refreshed")
                        .build()
                )
            } else {
                val newToken = runBlocking { sessionManager.refresh() }
                if (newToken == null) {
                    // Refresh failed — propagate 401 so callers emit SessionEvent.LoggedOut
                    chain.proceed(chain.request())
                } else {
                    chain.proceed(
                        chain.request().newBuilder()
                            .header("Authorization", "Bearer $newToken")
                            .build()
                    )
                }
            }
        }
    }
}

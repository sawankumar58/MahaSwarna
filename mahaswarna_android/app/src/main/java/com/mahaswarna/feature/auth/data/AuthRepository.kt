package com.mahaswarna.feature.auth.data

import com.mahaswarna.feature.auth.data.remote.AuthApi
import com.mahaswarna.feature.auth.data.remote.TokenResponse
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Stub repository providing the token-refresh contract required by
 * [SessionManager] in Phase 1.
 *
 * Full implementation (OTP login, consent, logout, account delete)
 * lives in Phase 2 (feature/auth/).
 */
@Singleton
class AuthRepository @Inject constructor(
    private val authApi: AuthApi,
) {
    /**
     * Exchanges a refresh token for a new access/refresh token pair.
     * Called by [SessionManager.refresh] on 401 responses.
     *
     * @throws retrofit2.HttpException on HTTP errors (caller handles 401 → logout)
     */
    suspend fun refreshToken(refreshToken: String): TokenResponse =
        authApi.refreshToken(RefreshRequest(refreshToken))
}

data class RefreshRequest(val refreshToken: String)

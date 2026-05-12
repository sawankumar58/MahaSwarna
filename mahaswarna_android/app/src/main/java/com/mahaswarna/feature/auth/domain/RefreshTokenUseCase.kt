package com.mahaswarna.feature.auth.domain

import com.mahaswarna.core.auth.SessionManager
import javax.inject.Inject

/**
 * Triggers a token refresh via [SessionManager].
 *
 * SessionManager is the authoritative owner of the refresh cycle — it handles
 * the synchronized lock, TokenStore persistence, and [SessionEvent.LoggedOut]
 * emission on failure. This use case is a thin domain wrapper for callers that
 * need an explicit refresh trigger outside of the AuthInterceptor's automatic path.
 *
 * Returns the new access token, or null if refresh failed (LoggedOut already emitted).
 */
class RefreshTokenUseCase @Inject constructor(
    private val sessionManager: SessionManager,
) {
    suspend operator fun invoke(): String? = sessionManager.refresh()
}

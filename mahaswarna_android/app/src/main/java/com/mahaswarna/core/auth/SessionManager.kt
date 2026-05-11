package com.mahaswarna.core.auth

import com.google.firebase.crashlytics.FirebaseCrashlytics
import com.mahaswarna.feature.auth.data.AuthRepository
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import javax.inject.Inject
import javax.inject.Singleton

/** Events emitted by SessionManager; observed in MainActivity. */
sealed class SessionEvent {
    data object LoggedOut : SessionEvent()
}

/**
 * Token lifecycle management.
 * - isExpired() / shouldRefresh(): client-side JWT claims check (no network).
 * - refresh(): calls AuthRepository.refresh(); on failure emits LoggedOut.
 * - emitLoggedOut(): clears tokens + emits the event → MainActivity
 *   calls AppDatabase.clearSessionData() then navigates to Login.
 *
 * AuthRepository is injected lazily to break the DI cycle:
 *   NetworkModule → AuthInterceptor → SessionManager → AuthRepository
 *                                                       → NetworkModule (retrofit)
 * The dagger.Lazy wrapper defers repository construction until first use.
 */
@Singleton
class SessionManager @Inject constructor(
    private val tokenStore: TokenStore,
    private val authRepository: dagger.Lazy<AuthRepository>,
) {
    private val _events = MutableSharedFlow<SessionEvent>(extraBufferCapacity = 1)
    val events: SharedFlow<SessionEvent> = _events

    fun isExpired(): Boolean {
        val token = tokenStore.getAccessToken() ?: return true
        return JwtParser.isExpired(token)
    }

    fun shouldRefresh(): Boolean {
        val token = tokenStore.getAccessToken() ?: return true
        return JwtParser.shouldRefresh(token)
    }

    /**
     * Attempts to refresh the access token using the stored refresh token.
     * Returns the new access token on success, null on failure.
     * On failure: emits [SessionEvent.LoggedOut].
     */
    suspend fun refresh(): String? {
        val refreshToken = tokenStore.getRefreshToken() ?: run {
            emitLoggedOut()
            return null
        }
        return runCatching {
            val tokens = authRepository.get().refreshToken(refreshToken)
            tokenStore.saveTokens(tokens.accessToken, tokens.refreshToken)
            tokens.accessToken
        }.getOrElse { e ->
            FirebaseCrashlytics.getInstance().log("Token refresh failed: ${e.message}")
            emitLoggedOut()
            null
        }
    }

    fun emitLoggedOut() {
        tokenStore.clearAll()
        _events.tryEmit(SessionEvent.LoggedOut)
    }
}

package com.mahaswarna.feature.billing.integrity

import android.content.Context
import com.google.android.play.core.integrity.IntegrityManagerFactory
import com.google.android.play.core.integrity.IntegrityTokenRequest
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.tasks.await
import javax.inject.Inject
import javax.inject.Singleton

/**
 * PlayIntegrityVerifier — obtains a Play Integrity token for device attestation.
 *
 * Called in two mandatory contexts:
 *  1. Before [POST /auth/login] — see LoginViewModel.
 *  2. Before any purchase flow — see PaywallViewModel.
 *
 * On HTTP 403 `device_not_trusted` from the backend: show non-dismissible
 * "This device is not supported" screen (do NOT navigate to Home).
 * On [IntegrityManager] failure (Play Services unavailable): surface as a
 * login or purchase error — do NOT silently proceed without a token.
 *
 * CLOUD_PROJECT_NUMBER must be the Google Cloud project number linked to the
 * app's Play Console account. Set via BuildConfig or remote config; never hardcode
 * as a string literal (it differs between debug/staging/production projects).
 */
@Singleton
class PlayIntegrityVerifier @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    /**
     * Requests a Play Integrity token.
     *
     * @param nonce A base64url-encoded nonce generated server-side. For login flows,
     *              this is a random UUID from the backend's /auth/send-otp response.
     *              For billing flows, use a UUID generated client-side (no server round-trip needed).
     * @return The integrity token string to send to the backend.
     * @throws com.google.android.play.core.integrity.IntegrityServiceException when Play
     *         Services are unavailable or attestation fails.
     */
    suspend fun requestToken(nonce: String): String {
        val integrityManager = IntegrityManagerFactory.create(context)
        val tokenResponse = integrityManager.requestIntegrityToken(
            IntegrityTokenRequest.builder()
                .setNonce(nonce)
                .build()
        ).await()
        return tokenResponse.token()
    }

    /**
     * Generates a fresh nonce for billing flows (client-generated; no server round-trip).
     * Base64url encoding satisfies the Play Integrity SDK nonce requirements.
     */
    fun generateNonce(): String {
        val bytes = ByteArray(32).also { java.security.SecureRandom().nextBytes(it) }
        return android.util.Base64.encodeToString(
            bytes,
            android.util.Base64.URL_SAFE or android.util.Base64.NO_WRAP or android.util.Base64.NO_PADDING
        )
    }
}

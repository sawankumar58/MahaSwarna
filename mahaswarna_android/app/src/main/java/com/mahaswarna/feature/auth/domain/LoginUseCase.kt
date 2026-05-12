package com.mahaswarna.feature.auth.domain

import com.mahaswarna.feature.auth.data.AuthRepository
import com.mahaswarna.feature.auth.data.remote.AuthTokenResponse
import javax.inject.Inject

/**
 * Thin use-case wrapper over [AuthRepository] login paths.
 *
 * Callers (LoginViewModel) use this for a clean domain boundary. The actual
 * HTTP and token-persistence logic lives in [AuthRepository].
 *
 * Two invoke paths mirror the two OTP providers:
 *   - Firebase: [invokeFirebase] — requires a firebaseIdToken from PhoneAuthCredential.
 *   - Msg91:    [invokeMsg91]   — requires the raw OTP from the SMS.
 *
 * Both paths require a Play Integrity [integrityToken] and a [cityID] for first-time users.
 */
class LoginUseCase @Inject constructor(
    private val authRepository: AuthRepository,
) {
    suspend fun invokeFirebase(
        phone: String,
        firebaseIdToken: String,
        integrityToken: String,
        cityID: String?,
    ): AuthTokenResponse = authRepository.loginFirebase(
        phone           = phone,
        firebaseIdToken = firebaseIdToken,
        integrityToken  = integrityToken,
        cityID          = cityID,
    )

    suspend fun invokeMsg91(
        phone: String,
        otp: String,
        integrityToken: String,
        cityID: String?,
    ): AuthTokenResponse = authRepository.loginMsg91(
        phone          = phone,
        otp            = otp,
        integrityToken = integrityToken,
        cityID         = cityID,
    )
}

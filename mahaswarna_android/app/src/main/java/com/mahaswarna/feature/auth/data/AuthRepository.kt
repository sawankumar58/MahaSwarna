package com.mahaswarna.feature.auth.data

import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.feature.auth.data.remote.AuthApi
import com.mahaswarna.feature.auth.data.remote.AuthTokenResponse
import com.mahaswarna.feature.auth.data.remote.ConsentRequest
import com.mahaswarna.feature.auth.data.remote.LoginRequest
import com.mahaswarna.feature.auth.data.remote.SendOtpResponse
import javax.inject.Inject
import javax.inject.Singleton

// Keep original RefreshRequest for SessionManager compat
data class RefreshRequest(val refreshToken: String)

/**
 * Full Phase 2 AuthRepository.
 *
 * Responsibilities:
 *   sendOtp()         → POST /auth/send-otp; returns provider ("firebase"|"msg91")
 *   loginFirebase()   → POST /auth/login with firebaseIdToken; saves tokens to TokenStore
 *   loginMsg91()      → POST /auth/login with otp; saves tokens to TokenStore
 *   logConsent()      → POST /user/consent — called EXACTLY TWICE per accept event
 *   refreshToken()    → POST /auth/refresh; called by SessionManager on 401
 *
 * City handling: cityID is passed in login calls for first-time users only.
 * The backend writes city ONLY on xmax == 0 (fresh insert) — never overwrites returning users.
 *
 * INVARIANT: logConsent() MUST be called with "privacy_policy" then "tos" in order.
 * "ai_disclaimer" must never be passed — enforced by ConsentType enum.
 */
@Singleton
class AuthRepository @Inject constructor(
    private val authApi: AuthApi,
    private val tokenStore: TokenStore,
) {
    /** Trigger OTP delivery. Provider in response drives client-side OTP flow. */
    suspend fun sendOtp(phone: String): SendOtpResponse =
        authApi.sendOtp(com.mahaswarna.feature.auth.data.remote.SendOtpRequest(phone))

    /**
     * Firebase login path.
     * @param phone      E.164 phone number
     * @param firebaseIdToken  ID token from PhoneAuthCredential
     * @param integrityToken   Play Integrity API token (obtained before sendOtp)
     * @param cityID           city slug — written only for new users
     */
    suspend fun loginFirebase(
        phone: String,
        firebaseIdToken: String,
        integrityToken: String,
        cityID: String?,
    ): AuthTokenResponse {
        val response = authApi.login(
            LoginRequest(
                phone            = phone,
                provider         = "firebase",
                integrityToken   = integrityToken,
                firebaseIdToken  = firebaseIdToken,
                cityID           = cityID,
            )
        )
        saveTokens(response)
        return response
    }

    /**
     * MSG91 login path.
     * @param phone          E.164 phone number
     * @param otp            6-digit code from MSG91 SMS
     * @param integrityToken Play Integrity API token
     * @param cityID         city slug — written only for new users
     */
    suspend fun loginMsg91(
        phone: String,
        otp: String,
        integrityToken: String,
        cityID: String?,
    ): AuthTokenResponse {
        val response = authApi.login(
            LoginRequest(
                phone          = phone,
                provider       = "msg91",
                integrityToken = integrityToken,
                otp            = otp,
                cityID         = cityID,
            )
        )
        saveTokens(response)
        return response
    }

    /**
     * Log a single consent record.
     * Callers MUST call this twice: once with ConsentType.PRIVACY_POLICY, once with ConsentType.TOS.
     * "ai_disclaimer" is NEVER a valid type — enforced by the sealed class below.
     */
    suspend fun logConsent(type: ConsentType, version: String = "1.0") {
        authApi.logConsent(ConsentRequest(consentType = type.wireValue, version = version))
    }

    /** Called by SessionManager on 401 responses. */
    suspend fun refreshToken(refreshToken: String): AuthTokenResponse =
        authApi.refreshToken(RefreshRequest(refreshToken)).also { resp ->
            saveTokens(resp)
        }

    // ── Private ───────────────────────────────────────────────────────────────

    private fun saveTokens(response: AuthTokenResponse) {
        tokenStore.saveTokens(response.accessToken, response.refreshToken)
    }
}

/**
 * Strongly-typed consent type enum.
 * "ai_disclaimer" is intentionally absent — it is display-only and must never be POSTed.
 */
enum class ConsentType(val wireValue: String) {
    PRIVACY_POLICY("privacy_policy"),
    TOS("tos"),
}

package com.mahaswarna.feature.auth.data.remote

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.POST

// ── Request DTOs ──────────────────────────────────────────────────────────────

@Serializable
data class SendOtpRequest(val phone: String)

@Serializable
data class SendOtpResponse(val provider: String)  // "firebase" | "msg91"

@Serializable
data class LoginRequest(
    val phone: String,
    val provider: String,                       // "firebase" | "msg91"
    val integrityToken: String,
    val firebaseIdToken: String? = null,        // set when provider == "firebase"
    val otp: String? = null,                    // set when provider == "msg91"
    val cityID: String? = null,                 // written only on fresh user insert
)

@Serializable
data class ConsentRequest(
    val consentType: String,    // ALLOWLIST: "privacy_policy" | "tos" — never "ai_disclaimer"
    val version: String = "1.0",
)

// TokenResponse already declared in this package via original AuthApi.kt stub.
// It is redeclared here as the authoritative version; the stub can be deleted.
@Serializable
data class AuthTokenResponse(
    val accessToken: String,
    val refreshToken: String,
    val tier: String,           // "FREE" | "PREMIUM" | "ADMIN"
)

// ── Retrofit interface ────────────────────────────────────────────────────────

interface AuthApi {
    /** Trigger OTP delivery. Returns which OTP provider the client should use. */
    @POST("auth/send-otp")
    suspend fun sendOtp(@Body request: SendOtpRequest): SendOtpResponse

    /**
     * Verify OTP and issue JWT pair.
     * Firebase path: include firebaseIdToken; otp must be null.
     * MSG91 path:    include otp; firebaseIdToken must be null.
     * Never include both in the same request.
     */
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): AuthTokenResponse

    /**
     * Log user consent.
     * Called EXACTLY TWICE on first accept: once with "privacy_policy", once with "tos".
     * "ai_disclaimer" is NEVER sent to this endpoint — it is display-only.
     */
    @POST("user/consent")
    suspend fun logConsent(@Body request: ConsentRequest)

    /**
     * Refresh access token using the stored refresh token.
     * Also present in the Phase 1 stub; this interface supersedes it.
     */
    @POST("auth/refresh")
    suspend fun refreshToken(@Body request: com.mahaswarna.feature.auth.data.RefreshRequest): TokenResponse
}

// Legacy alias — kept for SessionManager / AuthRepository back-compat
typealias TokenResponse = AuthTokenResponse

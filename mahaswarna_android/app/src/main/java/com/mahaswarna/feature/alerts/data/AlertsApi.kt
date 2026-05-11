package com.mahaswarna.feature.alerts.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path

// ── Request DTOs ──────────────────────────────────────────────────────────────

@Serializable
data class RegisterTokenRequest(val fcmToken: String)

@Serializable
data class CreateAlertRequest(
    val cityId: String,
    val metal: String,       // "gold" | "silver"
    val threshold: Double,   // price per gram in INR
    val direction: String,   // "above" | "below"
)

// ── Response DTOs ─────────────────────────────────────────────────────────────

@Serializable
data class AlertResponseDto(
    val id: String,
    val cityId: String,
    val metal: String,
    val threshold: Double,
    val direction: String,
    @SerialName("createdAt") val createdAt: String,
)

@Serializable
data class AlertListResponse(
    val alerts: List<AlertResponseDto>,
)

// ── Retrofit interface ────────────────────────────────────────────────────────

interface AlertsApi {

    /** GET /alerts — list all alerts for the authenticated user. */
    @GET("alerts")
    suspend fun listAlerts(): AlertListResponse

    /** POST /alerts — create a new price threshold alert. */
    @POST("alerts")
    suspend fun createAlert(@Body request: CreateAlertRequest): AlertResponseDto

    /** DELETE /alerts/{id} — delete an alert (returns 204 No Content). */
    @DELETE("alerts/{id}")
    suspend fun deleteAlert(@Path("id") alertId: String)

    /** POST /alerts/device-token — register or refresh FCM token. */
    @POST("alerts/device-token")
    suspend fun registerDeviceToken(@Body request: RegisterTokenRequest)
}

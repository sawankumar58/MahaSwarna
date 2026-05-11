package com.mahaswarna.feature.alerts.data

import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.POST

@Serializable
data class RegisterTokenRequest(val fcmToken: String)

interface AlertsApi {
    @POST("alerts/device-token")
    suspend fun registerDeviceToken(@Body request: RegisterTokenRequest)
}

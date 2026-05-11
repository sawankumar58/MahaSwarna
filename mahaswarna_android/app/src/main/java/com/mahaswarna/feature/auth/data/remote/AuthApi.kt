package com.mahaswarna.feature.auth.data.remote

import com.mahaswarna.feature.auth.data.RefreshRequest
import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.POST

@Serializable
data class TokenResponse(
    val accessToken: String,
    val refreshToken: String,
)

interface AuthApi {
    @POST("auth/refresh")
    suspend fun refreshToken(@Body request: RefreshRequest): TokenResponse
}

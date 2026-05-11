package com.mahaswarna.core.di

import com.mahaswarna.BuildConfig
import com.mahaswarna.core.auth.SessionManager
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.core.network.AiQuotaInterceptor
import com.mahaswarna.core.network.AuthInterceptor
import com.mahaswarna.core.network.LogRedactionInterceptor
import com.mahaswarna.core.network.VersionInterceptor
import com.mahaswarna.core.storage.PreferenceStore
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import java.util.concurrent.TimeUnit
import javax.inject.Named
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {

    // ── Shared Json instance ──────────────────────────────────────────────────

    @Provides
    @Singleton
    fun provideJson(): Json = Json {
        ignoreUnknownKeys = true
        isLenient = true
        coerceInputValues = true
    }

    // ── Interceptors ──────────────────────────────────────────────────────────

    @Provides
    @Singleton
    fun provideVersionInterceptor(): VersionInterceptor = VersionInterceptor()

    @Provides
    @Singleton
    fun provideAuthInterceptor(
        tokenStore: TokenStore,
        sessionManager: SessionManager,
    ): AuthInterceptor = AuthInterceptor(tokenStore, sessionManager)

    @Provides
    @Singleton
    fun provideAiQuotaInterceptor(preferenceStore: PreferenceStore): AiQuotaInterceptor =
        AiQuotaInterceptor(preferenceStore)

    @Provides
    @Singleton
    fun provideLogRedactionInterceptor(): LogRedactionInterceptor = LogRedactionInterceptor()

    // ── Primary OkHttpClient ──────────────────────────────────────────────────
    // Interceptor order is MANDATORY (canonical invariant):
    //   1. VersionInterceptor
    //   2. AuthInterceptor
    //   3. AiQuotaInterceptor
    //   4. LogRedactionInterceptor
    //   5. HttpLoggingInterceptor (debug only)

    @Provides
    @Singleton
    fun provideOkHttpClient(
        versionInterceptor: VersionInterceptor,
        authInterceptor: AuthInterceptor,
        aiQuotaInterceptor: AiQuotaInterceptor,
        logRedactionInterceptor: LogRedactionInterceptor,
    ): OkHttpClient = OkHttpClient.Builder()
        .addInterceptor(versionInterceptor)         // 1
        .addInterceptor(authInterceptor)             // 2
        .addInterceptor(aiQuotaInterceptor)          // 3
        .addInterceptor(logRedactionInterceptor)     // 4
        .apply {
            if (BuildConfig.DEBUG) {
                addInterceptor(                      // 5 — debug only
                    HttpLoggingInterceptor().apply {
                        level = HttpLoggingInterceptor.Level.BODY
                    }
                )
            }
        }
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .build()

    // ── S3 / CDN OkHttpClient — NO AuthInterceptor ────────────────────────────

    @Provides
    @Singleton
    @Named("s3")
    fun provideS3OkHttpClient(): OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .writeTimeout(60, TimeUnit.SECONDS)
        .build()

    // ── WebSocket OkHttpClient ────────────────────────────────────────────────

    @Provides
    @Singleton
    @Named("ws")
    fun provideWsOkHttpClient(): OkHttpClient = OkHttpClient.Builder()
        .pingInterval(30, TimeUnit.SECONDS)
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.SECONDS)   // no read timeout for long-lived WS
        .build()

    // ── Retrofit ─────────────────────────────────────────────────────────────
    // Retrofit 3.0.0 ships built-in kotlinx.serialization support via
    // retrofit2.converter.kotlinx.serialization.asConverterFactory — no
    // separate Jakewharton converter artifact required.

    @Provides
    @Singleton
    fun provideRetrofit(okHttpClient: OkHttpClient, json: Json): Retrofit =
        Retrofit.Builder()
            .baseUrl(BuildConfig.BASE_URL)
            .client(okHttpClient)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
}

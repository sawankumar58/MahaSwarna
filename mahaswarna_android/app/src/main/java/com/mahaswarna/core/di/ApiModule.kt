package com.mahaswarna.core.di

import com.mahaswarna.feature.auth.data.remote.AuthApi
import com.mahaswarna.feature.catalog.data.CatalogApi
import com.mahaswarna.feature.flags.data.FlagsApi
import com.mahaswarna.feature.home.data.BffApi
import com.mahaswarna.feature.marketplace.data.MarketplaceApi
import com.mahaswarna.feature.rates.data.remote.RatesApi
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import retrofit2.Retrofit
import javax.inject.Singleton

/**
 * All Retrofit API bindings.
 *
 * FIX (Phase 4): Added [CatalogApi] and [MarketplaceApi] @Provides.
 * Both were missing from the original ApiModule — Hilt would fail at compile time
 * with MissingBinding errors for CatalogRepository and MarketplaceRepository.
 *
 * All APIs share the primary [Retrofit] instance from [NetworkModule] (with AuthInterceptor).
 * The S3 client ([Named("s3")]) is injected directly into [ShopViewModel] — not here.
 */
@Module
@InstallIn(SingletonComponent::class)
object ApiModule {

    @Provides
    @Singleton
    fun provideAuthApi(retrofit: Retrofit): AuthApi =
        retrofit.create(AuthApi::class.java)

    @Provides
    @Singleton
    fun provideFlagsApi(retrofit: Retrofit): FlagsApi =
        retrofit.create(FlagsApi::class.java)

    @Provides
    @Singleton
    fun provideBffApi(retrofit: Retrofit): BffApi =
        retrofit.create(BffApi::class.java)

    @Provides
    @Singleton
    fun provideRatesApi(retrofit: Retrofit): RatesApi =
        retrofit.create(RatesApi::class.java)

    // ── Phase 4 additions ─────────────────────────────────────────────────────

    /**
     * Provides [CatalogApi] backed by the primary Retrofit instance.
     * Routes: GET /catalog/search, GET /catalog/designs/{id},
     *         GET /catalog/recommendations, POST /catalog/image-search (gated).
     */
    @Provides
    @Singleton
    fun provideCatalogApi(retrofit: Retrofit): CatalogApi =
        retrofit.create(CatalogApi::class.java)

    /**
     * Provides [MarketplaceApi] backed by the primary Retrofit instance.
     * Routes: POST /marketplace/shops, GET /marketplace/shops,
     *         POST /marketplace/shops/{id}/banner-upload-url,
     *         POST /marketplace/shops/{id}/banner-confirm,
     *         POST /marketplace/shops/{id}/invoices (@Streaming PDF).
     */
    @Provides
    @Singleton
    fun provideMarketplaceApi(retrofit: Retrofit): MarketplaceApi =
        retrofit.create(MarketplaceApi::class.java)
}

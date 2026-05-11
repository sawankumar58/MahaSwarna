package com.mahaswarna.core.di

import com.mahaswarna.feature.auth.data.remote.AuthApi
import com.mahaswarna.feature.flags.data.FlagsApi
import com.mahaswarna.feature.home.data.BffApi
import com.mahaswarna.feature.rates.data.remote.RatesApi
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import retrofit2.Retrofit
import javax.inject.Singleton

/**
 * Phase 2 API bindings.
 * Add these @Provides methods to the existing NetworkModule, or keep as a separate module.
 * All APIs use the shared Retrofit instance from NetworkModule.provideRetrofit().
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
}

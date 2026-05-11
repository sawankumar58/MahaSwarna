package com.mahaswarna.core.di

import com.mahaswarna.core.websocket.WsClient
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import javax.inject.Named
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object WsModule {
    @Provides
    @Singleton
    fun provideWsClient(
        @Named("ws") okHttpClient: OkHttpClient,
        json: Json,
    ): WsClient = WsClient(okHttpClient, json)
}

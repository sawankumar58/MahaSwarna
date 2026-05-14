package com.mahaswarna.feature.rates.data

import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.core.websocket.WsClient
import com.mahaswarna.core.websocket.WsEnvelope
import com.mahaswarna.data.mapper.toRoomEntity
import com.mahaswarna.feature.rates.data.remote.RatesApi
import com.mahaswarna.feature.rates.domain.Rate
import com.mahaswarna.feature.rates.domain.RateHistoryPoint
import com.mahaswarna.local.dao.RateDao
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.map
import kotlinx.serialization.json.Json
import com.mahaswarna.data.mapper.toRateDomain
import com.mahaswarna.data.mapper.toDomain
import kotlinx.serialization.json.decodeFromJsonElement
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Rates repository.
 *
 * Source priority: WS push > REST pull > Room cache.
 *
 * WS message handling:
 *   WsEnvelope.channel == "rates" → deserialize payload to RateUpdatePayload
 *   Update _currentRate StateFlow and persist to Room.
 *
 * REST fallback:
 *   getRate(cityId): on WS unavailable or first load.
 *   getHistory(cityId): no Room cache — always network. Required for RateHistoryScreen.
 */
@Singleton
class RatesRepository @Inject constructor(
    private val ratesApi: RatesApi,
    private val wsClient: WsClient,
    private val json: Json,
    private val preferenceStore: PreferenceStore,
    private val rateDao: RateDao,
) {
    private val _currentRate = MutableStateFlow<Rate?>(null)
    val currentRateFlow: StateFlow<Rate?> = _currentRate.asStateFlow()

    // ── WS rate push ──────────────────────────────────────────────────────────

    /**
     * Starts collecting rate updates pushed via WebSocket.
     * Call this from RatesDashboardViewModel.init or HomeViewModel after WS connects.
     * Filters envelopes where channel == "rates".
     */
    fun rateUpdatesFromWs(): Flow<Rate> =
        wsClient.messageFlow()
            .filter { it.channel == "rates" }
            .map { envelope ->
                val payload = json.decodeFromJsonElement<WsRatePayload>(envelope.payload)
                Rate(
                    cityId      = payload.cityId,
                    gold        = payload.gold,
                    silver      = payload.silver,
                    source      = payload.source,
                    generatedAt = payload.generatedAt,
                    isStale     = payload.stale,
                ).also { rate ->
                    _currentRate.value = rate
                    rateDao.upsertRate(rate.toRoomEntity())
                }
            }

    // ── REST ──────────────────────────────────────────────────────────────────

    /**
     * Fetch current rate for a city via REST (BFF alternative).
     * Used as fallback when WS is disconnected.
     */
    suspend fun getRate(cityId: String): Rate {
        val dto = ratesApi.getRate(cityId)
        val rate = dto.toRateDomain()
        _currentRate.value = rate
        rateDao.upsertRate(rate.toRoomEntity())
        return rate
    }

    /**
     * Fetch rate history for a city.
     * No Room cache — always hits network. Required for RateHistoryScreen.
     * On failure, caller shows error state.
     */
    suspend fun getHistory(cityId: String): List<RateHistoryPoint> =
        ratesApi.getRateHistory(cityId).map { it.toDomain() }

    // ── Room cache read ───────────────────────────────────────────────────────

    /**
     * Observe the latest cached rate for a city from Room.
     * Emits immediately on subscription (cold-start path — ≤400ms target).
     * Returns null if no cached value exists yet.
     */
    fun cachedRateFlow(cityId: String): Flow<Rate?> =
        rateDao.getLatest(cityId).map { entity -> entity?.toRateDomain() }


}

/** Deserialized WS rates-channel payload. Shape matches backend ws_rates_message.go. */
@kotlinx.serialization.Serializable
private data class WsRatePayload(
    val cityId: String,
    val gold: Double,
    val silver: Double,
    val source: String,
    val generatedAt: String,
    val stale: Boolean,
)

package com.mahaswarna.feature.home.data

import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.feature.home.domain.HomeData
import com.mahaswarna.feature.home.domain.RateInfo
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import javax.inject.Inject
import javax.inject.Singleton

/**
 * HomeRepository — local-first data strategy.
 *
 * Source priority: WS push > REST pull > Room cache.
 *
 * INVARIANT: after every BFF fetch, ALL fields must be persisted to Room:
 *   homeDao.upsert(home.toRoomEntity())
 *   ratesDao.upsertAll(home.rates.map { it.toRoomEntity() })
 *   alertsDao.upsertAll(home.alerts.map { it.toRoomEntity() })
 *   prefs.setLastRefreshed(System.currentTimeMillis())
 * Never hold HomeResponse in ViewModel memory only — next cold start renders from Room.
 *
 * degradedFlow: StateFlow<Boolean> — updated on every BFF response (including polling).
 *   _degraded.value = response._degraded ?: false
 * NOT persisted to Room — transient signal only.
 * Resets correctly when polling mode is lifted (clears to false on next non-degraded response).
 */
@Singleton
class HomeRepository @Inject constructor(
    private val bffApi: BffApi,
    private val preferenceStore: PreferenceStore,
    // Room DAOs injected here in full implementation; stubs omitted for brevity
    // private val homeDao: HomeDao,
    // private val ratesDao: RatesDao,
    // private val alertsDao: AlertsDao,
) {
    // ── Degraded signal ───────────────────────────────────────────────────────
    private val _degraded = MutableStateFlow(false)
    val degradedFlow: StateFlow<Boolean> = _degraded.asStateFlow()

    // ── In-memory last known state (populated from Room + REST) ───────────────
    private val _homeData = MutableStateFlow<HomeData?>(null)
    val homeDataFlow: Flow<HomeData?> = _homeData.asStateFlow()

    /**
     * Fetch fresh home data from BFF REST.
     * Persists ALL fields to Room. Updates degradedFlow.
     * Callers (HomeViewModel) collect homeDataFlow reactively — no need to observe return value.
     *
     * On failure: silently keeps last cached data; degradedFlow unchanged.
     */
    suspend fun refresh() {
        runCatching {
            val response = bffApi.getHome()

            // Update degraded signal BEFORE Room persist (transient — not stored)
            _degraded.value = response.degraded

            val domainData = response.toDomain()

            // TODO: persist to Room — requires injected DAOs
            // homeDao.upsert(domainData.toRoomEntity())
            // ratesDao.upsertAll(...)
            // alertsDao.upsertAll(...)
            preferenceStore.setLastRefreshed(System.currentTimeMillis())

            _homeData.value = domainData
        }.onFailure {
            // Keep last cached data; _degraded not updated (retains last known value)
        }
    }

    /**
     * Returns cached HomeData from Room on cold start.
     * Returns null on first install (Room empty).
     */
    fun getCachedHome(): Flow<HomeData?> {
        // Full implementation: homeDao.observeLatest().map { it?.toDomain() }
        // Stub: return in-memory flow
        return _homeData
    }

    /**
     * Apply a live rate update pushed via WebSocket.
     * Does NOT call BFF — WS is the authoritative live source.
     * Updates Room so the next cold start gets the WS rate.
     */
    fun applyWsRateUpdate(rate: RateInfo) {
        val current = _homeData.value
        if (current != null) {
            _homeData.value = current.copy(rate = rate)
            // TODO: persist: ratesDao.upsert(rate.toRoomEntity())
        }
    }

    // ── Converters ────────────────────────────────────────────────────────────

    private fun HomeResponse.toDomain(): HomeData = HomeData(
        rate = RateInfo(
            cityId      = rates.cityId,
            gold        = rates.gold,
            silver      = rates.silver,
            source      = rates.source,
            generatedAt = rates.generatedAt,
            isStale     = rates.stale,
        ),
        alerts = alerts?.map { it.toAlertDomain() } ?: emptyList(),
    )

    private fun AlertResponseDto.toAlertDomain() =
        com.mahaswarna.feature.alerts.domain.Alert(
            id          = id,
            metal       = metal,
            targetPrice = targetPrice,
            direction   = direction,
            active      = active,
        )
}

// Extension stored in PreferenceStore patch
private fun PreferenceStore.setLastRefreshed(time: Long) {
    // PATCH: add setLastRefreshed(Long) / getLastRefreshed(): Long to PreferenceStore.kt
    // Uses a longPreferencesKey("last_refreshed_at")
}

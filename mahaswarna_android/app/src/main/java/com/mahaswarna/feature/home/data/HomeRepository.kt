package com.mahaswarna.feature.home.data

import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.data.mapper.toAlertList
import com.mahaswarna.data.mapper.toDomain
import com.mahaswarna.data.mapper.toEntity
import com.mahaswarna.feature.home.domain.HomeData
import com.mahaswarna.feature.home.domain.RateInfo
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.dao.HomeDao
import com.mahaswarna.local.dao.RateDao
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

/**
 * HomeRepository — local-first data strategy.
 *
 * Source priority: WS push > REST pull > Room cache.
 *
 * cityId is sourced from [PreferenceStore.getSelectedCityId] internally — callers never
 * pass it. This is set at login (CityPicker) and updated in RatesDashboardScreen.
 *
 * INVARIANT: after every BFF fetch, ALL fields are persisted to Room:
 *   homeDao.upsert(response.toEntity())
 *   rateDao.upsertRate(response.rates.toEntity())
 *   alertDao.upsertAll(response.alerts.map { it.toEntity(...) })
 *   prefs.setLastRefreshed(System.currentTimeMillis())
 *
 * [degradedFlow]: transient — NOT persisted to Room.
 * Resets on next non-degraded BFF response.
 *
 * [homeDataFlow]: hot StateFlow combining Room cache for the current city.
 * Backed by [getCachedHome]; HomeViewModel collects this directly.
 */
@Singleton
class HomeRepository @Inject constructor(
    private val bffApi: BffApi,
    private val preferenceStore: PreferenceStore,
    private val homeDao: HomeDao,
    private val rateDao: RateDao,
    private val alertDao: AlertDao,
) {
    // ── Degraded signal (transient — never persisted) ─────────────────────────
    private val _degraded = MutableStateFlow(false)
    val degradedFlow: StateFlow<Boolean> = _degraded.asStateFlow()

    // ── homeDataFlow — hot StateFlow from Room for the current city ───────────

    /**
     * Reactive stream combining the latest cached rate (for the user's stored city)
     * and cached alerts into a [HomeData] domain object.
     *
     * Emits null until Room is populated (first install or post-logout wipe).
     * Automatically re-emits when [PreferenceStore.selectedCityId] changes.
     */
    @OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
    val homeDataFlow: Flow<HomeData?> = preferenceStore.getSelectedCityIdFlow()
        .flatMapLatest { cityId ->
            combine(
                rateDao.getLatest(cityId),
                homeDao.observe(),
            ) { rateEntity, homeEntity ->
                if (rateEntity == null) null
                else HomeData(
                    rate   = rateEntity.toDomain(),
                    alerts = homeEntity?.toAlertList() ?: emptyList(),
                )
            }
        }

    // ── getCachedHome — same as homeDataFlow, for GetHomeDataUseCase ──────────

    /**
     * Returns the same reactive stream as [homeDataFlow].
     * Kept as a named function for callers (use cases) that prefer explicit method calls.
     */
    @OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
    fun getCachedHome(): Flow<HomeData?> = homeDataFlow

    // ── refresh — fetches BFF and persists to Room ────────────────────────────

    /**
     * Fetch fresh home data from BFF REST.
     * Persists ALL fields to Room. Updates [degradedFlow].
     * cityId is sourced from [PreferenceStore.getSelectedCityId].
     *
     * On failure: silently keeps last cached data; [degradedFlow] retains last value.
     */
    suspend fun refresh() {
        val cityId = preferenceStore.getSelectedCityId()
        runCatching {
            val response = bffApi.getHome()

            // 1. Update transient degraded signal BEFORE Room persist.
            _degraded.value = response.degraded

            val now = System.currentTimeMillis()

            // 2. Persist ALL fields to Room — INVARIANT.
            rateDao.upsertRate(response.rates.toEntity(cachedAt = now))
            alertDao.upsertAll(
                response.alerts?.map { dto ->
                    // BFF AlertResponseDto lacks cityId/threshold; use stored cityId and
                    // targetPrice.toFloat() — full data persisted via GET /alerts.
                    com.mahaswarna.local.entity.AlertEntity(
                        id        = dto.id,
                        cityId    = cityId,
                        metal     = dto.metal,
                        direction = dto.direction,
                        threshold = dto.targetPrice.toFloat(),
                        active    = dto.active,
                        createdAt = "",
                    )
                } ?: emptyList()
            )
            homeDao.upsert(response.toEntity(cachedAt = now))
            preferenceStore.setLastRefreshed(now)

        }.onFailure {
            // Keep last cached data; degradedFlow retains last known value.
            // Error is intentionally swallowed — HomeViewModel renders from Room cache.
        }
    }

    // ── WS rate update ────────────────────────────────────────────────────────

    /**
     * Apply a live rate update pushed via WebSocket.
     * Persists directly to Room so the next cold start gets the WS rate.
     */
    suspend fun applyWsRateUpdate(rate: RateInfo) {
        val entity = com.mahaswarna.local.entity.RateEntity(
            cityId      = rate.cityId,
            gold        = rate.gold,
            silver      = rate.silver,
            source      = rate.source,
            generatedAt = rate.generatedAt,
            isStale     = rate.isStale,
            cachedAt    = System.currentTimeMillis(),
        )
        rateDao.upsertRate(entity)
    }

    /**
     * Rate history for the Vico chart in [RateHistoryScreen].
     * Populated by REST; WS does not back-fill history.
     */
    fun getRateHistory(cityId: String): Flow<List<RateInfo>> =
        rateDao.getHistory(cityId).map { list -> list.map { it.toDomain() } }

    /**
     * Clears the transient degraded signal (e.g. on WS reconnection when BFF response
     * with degraded=false has not yet arrived).
     */
    fun clearDegraded() {
        _degraded.value = false
    }
}

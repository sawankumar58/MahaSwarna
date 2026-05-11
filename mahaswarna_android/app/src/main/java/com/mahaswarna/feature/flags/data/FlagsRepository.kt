package com.mahaswarna.feature.flags.data

import androidx.datastore.preferences.core.stringPreferencesKey
import com.mahaswarna.feature.flags.domain.FeatureFlags
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

// DataStore key — stored as serialized JSON string
private val FLAGS_CACHE_KEY = stringPreferencesKey("feature_flags_cache")

/**
 * Feature flags repository.
 *
 * Contract:
 *   - getFlags() returns localCache ?: DEFAULT_FLAGS — always succeeds, never throws.
 *   - refresh() hits GET /config/feature-flags; on success writes to DataStore and
 *     updates the in-memory StateFlow; on failure (network, parse) returns DEFAULT_FLAGS
 *     silently (kills-switches must never crash the app startup path).
 *   - flagsFlow: StateFlow<FeatureFlags> — observed by HomeViewModel and MainActivity.
 *
 * DataStore is used (not EncryptedSharedPreferences) — flags are not sensitive.
 */
@Singleton
class FlagsRepository @Inject constructor(
    private val flagsApi: FlagsApi,
    private val json: Json,
    private val preferenceStore: com.mahaswarna.core.storage.PreferenceStore,
) {
    private val _flagsFlow = MutableStateFlow(FeatureFlags.DEFAULT)
    val flagsFlow: Flow<FeatureFlags> = _flagsFlow.asStateFlow()

    /** Returns cached flags (DataStore) or DEFAULT_FLAGS on first install / cache miss. */
    fun getFlags(): FeatureFlags = _flagsFlow.value

    /**
     * Fetches fresh flags from the backend.
     * Called on every app resume (MainActivity.onResume / HomeViewModel.init).
     * Failures are silenced — app continues with last known / default flags.
     *
     * @return the refreshed [FeatureFlags] (or the last good value on failure).
     */
    suspend fun refresh(): FeatureFlags {
        return runCatching {
            val dto   = flagsApi.getFeatureFlags()
            val flags = dto.toDomain()
            persistToCache(flags)
            _flagsFlow.value = flags
            flags
        }.getOrElse {
            // Network error / parse error — keep last known flags
            _flagsFlow.value
        }
    }

    /** Hydrates in-memory flow from DataStore on app start (call from Application or DI). */
    suspend fun loadFromCache() {
        val cached = readFromCache()
        if (cached != null) _flagsFlow.value = cached
    }

    // ── Private helpers ───────────────────────────────────────────────────────

    private fun FlagsDto.toDomain(): FeatureFlags = FeatureFlags(
        aiEnabled          = flags.aiEnabled,
        shopEnabled        = flags.shopEnabled,
        wsEnabled          = flags.wsEnabled,
        paymentsEnabled    = flags.paymentsEnabled,
        catalogEnabled     = flags.catalogEnabled,
        killSwitchAi          = killSwitch.ai,
        killSwitchWs          = killSwitch.ws,
        killSwitchPayments    = killSwitch.payments,
        killSwitchCatalog     = killSwitch.catalog,
        killSwitchImageSearch = killSwitch.imageSearch,
        params = params,
    )

    private suspend fun persistToCache(flags: FeatureFlags) {
        runCatching {
            val dto = FeatureFlagsCacheDto(
                aiEnabled          = flags.aiEnabled,
                shopEnabled        = flags.shopEnabled,
                wsEnabled          = flags.wsEnabled,
                paymentsEnabled    = flags.paymentsEnabled,
                catalogEnabled     = flags.catalogEnabled,
                killSwitchAi          = flags.killSwitchAi,
                killSwitchWs          = flags.killSwitchWs,
                killSwitchPayments    = flags.killSwitchPayments,
                killSwitchCatalog     = flags.killSwitchCatalog,
                killSwitchImageSearch = flags.killSwitchImageSearch,
                params = flags.params,
            )
            preferenceStore.setFlagsCache(json.encodeToString(dto))
        }
        // DataStore write failure is non-fatal — in-memory flow is already updated
    }

    private suspend fun readFromCache(): FeatureFlags? = runCatching {
        val raw = preferenceStore.getFlagsCache() ?: return@runCatching null
        val dto = json.decodeFromString<FeatureFlagsCacheDto>(raw)
        FeatureFlags(
            aiEnabled          = dto.aiEnabled,
            shopEnabled        = dto.shopEnabled,
            wsEnabled          = dto.wsEnabled,
            paymentsEnabled    = dto.paymentsEnabled,
            catalogEnabled     = dto.catalogEnabled,
            killSwitchAi          = dto.killSwitchAi,
            killSwitchWs          = dto.killSwitchWs,
            killSwitchPayments    = dto.killSwitchPayments,
            killSwitchCatalog     = dto.killSwitchCatalog,
            killSwitchImageSearch = dto.killSwitchImageSearch,
            params = dto.params,
        )
    }.getOrNull()
}

/** Serializable cache envelope stored in DataStore as a JSON string. */
@kotlinx.serialization.Serializable
private data class FeatureFlagsCacheDto(
    val aiEnabled: Boolean,
    val shopEnabled: Boolean,
    val wsEnabled: Boolean,
    val paymentsEnabled: Boolean,
    val catalogEnabled: Boolean,
    val killSwitchAi: Boolean,
    val killSwitchWs: Boolean,
    val killSwitchPayments: Boolean,
    val killSwitchCatalog: Boolean,
    val killSwitchImageSearch: Boolean,
    val params: Map<String, Double>,
)

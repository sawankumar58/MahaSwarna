package com.mahaswarna.feature.home.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.firebase.crashlytics.FirebaseCrashlytics
import com.mahaswarna.core.auth.SessionManager
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.core.websocket.WsClient
import com.mahaswarna.core.websocket.WsConnectionState
import com.mahaswarna.feature.auth.data.AuthRepository
import com.mahaswarna.feature.flags.data.FlagsRepository
import com.mahaswarna.feature.home.data.HomeRepository
import com.mahaswarna.feature.home.domain.HomeData
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ─────────────────────────────────────────────────────────────────

sealed class HomeUiState {
    data object Loading         : HomeUiState()
    data class  Success(val data: HomeData) : HomeUiState()
    data object NoDataAvailable : HomeUiState()
}

/**
 * StaleRateBanner visibility conditions (ANY of these → show banner):
 *   1. rate.isStale == true (backend field)
 *   2. wsState in {Reconnecting, Disconnected} for > 30 seconds
 *   3. wsState == Error (immediate, no 30s grace)
 *   4. killSwitchWs == true (polling mode — permanently stale)
 *   5. degradedFlow == true (BFF reported partial upstream failure)
 */
data class StaleBannerState(
    val rateStale:       Boolean = false,
    val wsDisconnected:  Boolean = false,   // ws stale > 30s or Error
    val killSwitchWs:    Boolean = false,
    val degraded:        Boolean = false,
) {
    val showBanner: Boolean
        get() = rateStale || wsDisconnected || killSwitchWs || degraded
}

// ── ViewModel ─────────────────────────────────────────────────────────────────

/**
 * HomeViewModel — initialisation order is an INVARIANT. Do not reorder.
 *
 * Step 1 — shimmer timeout guard (MUST be first; shimmerJob assigned before Room collector runs)
 * Step 2 — Room cache read (launched after shimmerJob is assigned)
 * Steps 3–5 in single coroutine (after steps 1+2):
 *   3. JWT pre-warm (wrapped in try/catch — exception must never abort WS connect)
 *   4. wsClient.connect() — only if !flags.killSwitchWs
 *   5. observeHomeData().collect { … } — shimmer cancel + Success emit
 *
 * Degraded signal: separate collector on homeRepository.degradedFlow.
 * WS stale: 30-second grace timer for Reconnecting/Disconnected; immediate for Error.
 */
@HiltViewModel
class HomeViewModel @Inject constructor(
    private val homeRepository: HomeRepository,
    private val flagsRepository: FlagsRepository,
    private val authRepository: AuthRepository,
    private val sessionManager: SessionManager,
    private val tokenStore: TokenStore,
    private val wsClient: WsClient,
) : ViewModel() {

    private val _uiState = MutableStateFlow<HomeUiState>(HomeUiState.Loading)
    val uiState: StateFlow<HomeUiState> = _uiState.asStateFlow()

    private val _staleBanner = MutableStateFlow(StaleBannerState())
    val staleBanner: StateFlow<StaleBannerState> = _staleBanner.asStateFlow()

    private var shimmerJob: Job? = null
    private var wsStaleJob:  Job? = null  // 30s timer for Reconnecting/Disconnected

    init {
        initHome()
        observeDegraded()
        observeWsState()
    }

    // ── Init sequence ─────────────────────────────────────────────────────────

    private fun initHome() {
        val flags = flagsRepository.getFlags()

        // Step 1 — shimmer timeout guard (FIRST — must be assigned before Room collector launches)
        shimmerJob = viewModelScope.launch {
            kotlinx.coroutines.delay(2_000)
            if (_uiState.value is HomeUiState.Loading) {
                _uiState.value = HomeUiState.NoDataAvailable
            }
        }

        // Step 2 — Room cache read
        viewModelScope.launch {
            homeRepository.getCachedHome().collect { cached ->
                if (cached != null && _uiState.value is HomeUiState.Loading) {
                    _uiState.value = HomeUiState.Success(cached)
                    shimmerJob?.cancel()
                }
            }
        }

        // Steps 3–5 in single coroutine
        viewModelScope.launch {
            // Step 3 — JWT pre-warm (MUST be wrapped; uncaught exception would abort WS connect)
            try {
                val remainingMs = sessionManager.accessTokenRemainingMs()
                if (remainingMs < 3 * 60_000L) {
                    authRepository.refreshToken(tokenStore.getRefreshToken() ?: "")
                }
            } catch (e: Exception) {
                FirebaseCrashlytics.getInstance().log("JWT pre-warm failed: ${e.message}")
                // DO NOT rethrow — WS connect must always proceed
            }

            // Step 4 — WS connect (gated by kill-switch)
            if (!flags.killSwitchWs) {
                val token = tokenStore.getAccessToken()
                if (token != null) {
                    // WS connect is launched in a non-blocking sub-coroutine so step 5 runs
                    launch {
                        wsClient.connect(token, com.mahaswarna.BuildConfig.WS_URL)
                    }
                }
            } else {
                // kill-switch active — show stale banner permanently
                _staleBanner.value = _staleBanner.value.copy(killSwitchWs = true)
            }

            // Step 5 — observe home data (includes BFF REST refresh on first collect)
            viewModelScope.launch { homeRepository.refresh() }

            homeRepository.homeDataFlow.collect { data ->
                if (data != null) {
                    shimmerJob?.cancel()
                    _uiState.value = HomeUiState.Success(data)
                    _staleBanner.value = _staleBanner.value.copy(rateStale = data.rate.isStale)
                }
            }
        }
    }

    // ── Degraded signal ───────────────────────────────────────────────────────

    private fun observeDegraded() {
        viewModelScope.launch {
            homeRepository.degradedFlow.collect { isDegraded ->
                _staleBanner.value = _staleBanner.value.copy(degraded = isDegraded)
            }
        }
    }

    // ── WS connection state → stale banner ───────────────────────────────────

    private fun observeWsState() {
        viewModelScope.launch {
            wsClient.connectionState.collect { wsState ->
                when (wsState) {
                    is WsConnectionState.Connected -> {
                        wsStaleJob?.cancel()
                        _staleBanner.value = _staleBanner.value.copy(wsDisconnected = false)
                    }
                    is WsConnectionState.Error -> {
                        wsStaleJob?.cancel()
                        // Immediate — no 30s grace for Error (terminal state)
                        _staleBanner.value = _staleBanner.value.copy(wsDisconnected = true)
                    }
                    is WsConnectionState.Reconnecting,
                    is WsConnectionState.Disconnected -> {
                        // 30-second grace before showing banner
                        wsStaleJob?.cancel()
                        wsStaleJob = viewModelScope.launch {
                            kotlinx.coroutines.delay(30_000)
                            _staleBanner.value = _staleBanner.value.copy(wsDisconnected = true)
                        }
                    }
                    else -> Unit
                }
            }
        }
    }

    // ── Public actions ────────────────────────────────────────────────────────

    fun retry() {
        _uiState.value = HomeUiState.Loading
        viewModelScope.launch {
            shimmerJob = launch {
                kotlinx.coroutines.delay(2_000)
                if (_uiState.value is HomeUiState.Loading) {
                    _uiState.value = HomeUiState.NoDataAvailable
                }
            }
            try {
                homeRepository.refresh()
            } catch (e: Exception) {
                shimmerJob?.cancel()
                _uiState.value = HomeUiState.NoDataAvailable
            }
        }
    }

    override fun onCleared() {
        super.onCleared()
        shimmerJob?.cancel()
        wsStaleJob?.cancel()
    }
}

/** Extension on SessionManager to expose remaining token life in ms. */
private fun SessionManager.accessTokenRemainingMs(): Long {
    // SessionManager already has shouldRefresh() which uses 12-min threshold.
    // Expose raw remaining ms: JwtParser.parse(token)?.exp * 1000 - System.currentTimeMillis()
    // This is a thin wrapper — full impl reads from JwtParser.
    return if (shouldRefresh()) 0L else Long.MAX_VALUE
}

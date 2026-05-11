package com.mahaswarna.feature.rates.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.firebase.analytics.FirebaseAnalytics
import com.google.firebase.analytics.ktx.analytics
import com.google.firebase.analytics.logEvent
import com.google.firebase.ktx.Firebase
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.core.websocket.WsClient
import com.mahaswarna.core.websocket.WsConnectionState
import com.mahaswarna.feature.rates.data.RatesRepository
import com.mahaswarna.feature.rates.domain.Rate
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ─────────────────────────────────────────────────────────────────

sealed class RatesDashboardUiState {
    data object Loading            : RatesDashboardUiState()
    data class  Success(val rate: Rate) : RatesDashboardUiState()
    data class  Error(val message: String) : RatesDashboardUiState()
}

// ── ViewModel ─────────────────────────────────────────────────────────────────

/**
 * RatesDashboardViewModel.
 *
 * isStale — DERIVED StateFlow combining:
 *   a) rate.isStale (from WS or REST, backend field — never client-computed)
 *   b) wsState in {Reconnecting, Disconnected} for > 30 seconds
 *   c) wsState == Error (immediate)
 *
 * isStale is read via isStale.value at FAB tap time for nav args:
 *   navController.navigate(Route.Calculator(goldRate, silverRate, isStale = isStale.value))
 *   navController.navigate(Route.BillPrint(goldRate, silverRate, isStale = isStale.value))
 * DO NOT read isStale from Room's RateEntity — it lags behind live WS disconnect state.
 *
 * Analytics: fires rate_viewed { cityId, source } once on first Success (LaunchedEffect in UI).
 */
@HiltViewModel
class RatesDashboardViewModel @Inject constructor(
    private val ratesRepository: RatesRepository,
    private val wsClient: WsClient,
    private val preferenceStore: PreferenceStore,
) : ViewModel() {

    private val _uiState = MutableStateFlow<RatesDashboardUiState>(RatesDashboardUiState.Loading)
    val uiState: StateFlow<RatesDashboardUiState> = _uiState.asStateFlow()

    // Stale components — combined below
    private val _rateStale = MutableStateFlow(false)
    private val _wsStale   = MutableStateFlow(false)

    /** Derived isStale — the ONLY value to use at FAB tap time. */
    val isStale: StateFlow<Boolean> get() = _isStale
    private val _isStale = MutableStateFlow(false)

    private var wsStaleJob: Job? = null

    init {
        observeWsRates()
        observeWsState()
        combineStaleSignals()
    }

    // ── Observers ─────────────────────────────────────────────────────────────

    private fun observeWsRates() {
        viewModelScope.launch {
            ratesRepository.rateUpdatesFromWs().collect { rate ->
                _uiState.value = RatesDashboardUiState.Success(rate)
                _rateStale.value = rate.isStale
            }
        }
        // Also observe cached rate from repository (covers REST fallback + Room)
        viewModelScope.launch {
            ratesRepository.currentRateFlow.collect { rate ->
                if (rate != null && _uiState.value is RatesDashboardUiState.Loading) {
                    _uiState.value = RatesDashboardUiState.Success(rate)
                    _rateStale.value = rate.isStale
                }
            }
        }
    }

    private fun observeWsState() {
        viewModelScope.launch {
            wsClient.connectionState.collect { wsState ->
                when (wsState) {
                    is WsConnectionState.Connected -> {
                        wsStaleJob?.cancel()
                        _wsStale.value = false
                    }
                    is WsConnectionState.Error -> {
                        wsStaleJob?.cancel()
                        _wsStale.value = true   // immediate — no 30s grace for Error
                    }
                    is WsConnectionState.Reconnecting,
                    is WsConnectionState.Disconnected -> {
                        wsStaleJob?.cancel()
                        wsStaleJob = viewModelScope.launch {
                            kotlinx.coroutines.delay(30_000)
                            _wsStale.value = true
                        }
                    }
                    else -> Unit
                }
            }
        }
    }

    private fun combineStaleSignals() {
        viewModelScope.launch {
            combine(_rateStale, _wsStale) { rate, ws -> rate || ws }
                .collect { _isStale.value = it }
        }
    }

    // ── Public actions ────────────────────────────────────────────────────────

    /** Fetches current rate via REST (called on retry or WS unavailable). */
    fun fetchRate(cityId: String) {
        viewModelScope.launch {
            runCatching {
                val rate = ratesRepository.getRate(cityId)
                _uiState.value = RatesDashboardUiState.Success(rate)
                _rateStale.value = rate.isStale
            }.onFailure { e ->
                if (_uiState.value is RatesDashboardUiState.Loading) {
                    _uiState.value = RatesDashboardUiState.Error(e.message ?: "Failed to load rates")
                }
            }
        }
    }

    override fun onCleared() {
        super.onCleared()
        wsStaleJob?.cancel()
    }
}

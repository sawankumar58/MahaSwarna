package com.mahaswarna.feature.alerts.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.feature.alerts.data.AlertsRepository
import com.mahaswarna.feature.alerts.domain.Alert
import com.google.firebase.analytics.FirebaseAnalytics
import com.google.firebase.analytics.ktx.analytics
import com.google.firebase.ktx.Firebase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class AlertsUiState {
    data object Loading : AlertsUiState()
    data class Success(val alerts: List<Alert>) : AlertsUiState()
    data class Error(val message: String) : AlertsUiState()
}

@HiltViewModel
class AlertsViewModel @Inject constructor(
    private val alertsRepository: AlertsRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow<AlertsUiState>(AlertsUiState.Loading)
    val uiState: StateFlow<AlertsUiState> = _uiState.asStateFlow()

    private val _createError = MutableStateFlow<String?>(null)
    val createError: StateFlow<String?> = _createError.asStateFlow()

    private val _deleteError = MutableStateFlow<String?>(null)
    val deleteError: StateFlow<String?> = _deleteError.asStateFlow()

    init {
        observeAlerts()
        syncAlerts()
    }

    private fun observeAlerts() {
        alertsRepository.observeAlerts()
            .onEach { alerts ->
                _uiState.value = AlertsUiState.Success(alerts)
            }
            .catch { e ->
                _uiState.value = AlertsUiState.Error(e.message ?: "Failed to load alerts")
            }
            .launchIn(viewModelScope)
    }

    /** Fetches alerts from backend and updates Room. Called on entry and after mutations. */
    fun syncAlerts() {
        viewModelScope.launch {
            runCatching { alertsRepository.syncAlerts() }
                .onFailure { e ->
                    // Non-fatal: keep Room cache displayed; show error only if Room is empty.
                    if (_uiState.value is AlertsUiState.Loading) {
                        _uiState.value = AlertsUiState.Error(e.message ?: "Sync failed")
                    }
                }
        }
    }

    /**
     * Creates a new price alert.
     * Fires `alert_created` analytics event on success.
     */
    fun createAlert(
        cityId: String,
        metal: String,
        threshold: Double,
        direction: String,
    ) {
        viewModelScope.launch {
            runCatching {
                alertsRepository.createAlert(
                    cityId    = cityId,
                    metal     = metal,
                    threshold = threshold,
                    direction = direction,
                )
            }.onSuccess {
                Firebase.analytics.logEvent("alert_created", android.os.Bundle().apply {
                    putString("metal", metal)
                    putString("direction", direction)
                })
                _createError.value = null
            }.onFailure { e ->
                _createError.value = e.message ?: "Failed to create alert"
            }
        }
    }

    /** Deletes an alert by ID. */
    fun deleteAlert(alertId: String) {
        viewModelScope.launch {
            runCatching { alertsRepository.deleteAlert(alertId) }
                .onFailure { e ->
                    _deleteError.value = e.message ?: "Failed to delete alert"
                }
        }
    }

    fun clearCreateError() { _createError.value = null }
    fun clearDeleteError() { _deleteError.value = null }
}

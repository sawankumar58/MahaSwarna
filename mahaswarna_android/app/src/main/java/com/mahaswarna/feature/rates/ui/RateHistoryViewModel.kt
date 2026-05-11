package com.mahaswarna.feature.rates.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.feature.rates.data.RatesRepository
import com.mahaswarna.feature.rates.domain.RateHistoryPoint
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

sealed class RateHistoryUiState {
    data object Loading : RateHistoryUiState()
    data class  Success(val points: List<RateHistoryPoint>) : RateHistoryUiState()
    data class  Error(val message: String) : RateHistoryUiState()
}

/**
 * RateHistoryViewModel — GAP-M4 fix.
 *
 * No Room cache for history — network-required on every open.
 * cityId sourced from current city preference / navigator argument.
 * Calls ratesRepository.getHistory(cityId) → GET /rates/:cityID/history.
 *
 * UiState drives Vico line chart in RateHistoryScreen.
 */
@HiltViewModel
class RateHistoryViewModel @Inject constructor(
    private val ratesRepository: RatesRepository,
    private val preferenceStore: PreferenceStore,
) : ViewModel() {

    private val _uiState = MutableStateFlow<RateHistoryUiState>(RateHistoryUiState.Loading)
    val uiState: StateFlow<RateHistoryUiState> = _uiState.asStateFlow()

    fun loadHistory(cityId: String) {
        _uiState.value = RateHistoryUiState.Loading
        viewModelScope.launch {
            runCatching {
                val points = ratesRepository.getHistory(cityId)
                _uiState.value = if (points.isEmpty())
                    RateHistoryUiState.Error("No history available for this city")
                else
                    RateHistoryUiState.Success(points)
            }.onFailure { e ->
                _uiState.value = RateHistoryUiState.Error(e.message ?: "Failed to load history")
            }
        }
    }

    fun retry(cityId: String) = loadHistory(cityId)
}

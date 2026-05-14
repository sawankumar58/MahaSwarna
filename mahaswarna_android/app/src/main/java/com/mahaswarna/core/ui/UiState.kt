package com.mahaswarna.core.ui

/**
 * Generic UI state sealed class for ViewModels.
 *
 * Replaces the eight per-ViewModel sealed classes
 * (AlertsUiState, RatesDashboardUiState, RateHistoryUiState,
 *  CatalogUiState, PaywallUiState, BillPrintUiState, ShopUiState, HomeUiState)
 * that each re-declared Loading / Success<T> / Error independently.
 *
 * ViewModels with extra states (e.g. CatalogUiState.Disabled, HomeUiState.NoDataAvailable)
 * can define service-specific sub-types that extend or wrap this class.
 *
 * Usage:
 * ```
 * private val _uiState = MutableStateFlow<UiState<List<Alert>>>(UiState.Loading)
 * val uiState: StateFlow<UiState<List<Alert>>> = _uiState.asStateFlow()
 * ```
 */
sealed class UiState<out T> {
    /** Initial/in-progress loading. Show shimmer or progress indicator. */
    data object Loading : UiState<Nothing>()

    /** Data successfully loaded. */
    data class Success<T>(val data: T) : UiState<T>()

    /**
     * Unrecoverable or recoverable error.
     * @param message User-facing error description.
     * @param isRetryable When true, show a Retry button. Defaults to true.
     */
    data class Error(
        val message: String,
        val isRetryable: Boolean = true,
    ) : UiState<Nothing>()
}

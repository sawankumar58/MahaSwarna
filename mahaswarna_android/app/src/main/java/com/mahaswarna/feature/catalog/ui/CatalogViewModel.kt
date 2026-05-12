package com.mahaswarna.feature.catalog.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.feature.catalog.data.CatalogRepository
import com.mahaswarna.feature.catalog.domain.Design
import com.mahaswarna.feature.catalog.domain.SearchDesignUseCase
import com.mahaswarna.feature.flags.data.FlagsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class CatalogUiState {
    data object Loading : CatalogUiState()
    data class Success(
        val designs: List<Design>,
        val page: Int,
        val totalPages: Int,
        val isLoadingMore: Boolean = false,
    ) : CatalogUiState()
    data class Error(val message: String) : CatalogUiState()
    /** killSwitchCatalog == true; caller should hide the tab. */
    data object Disabled : CatalogUiState()
}

@HiltViewModel
class CatalogViewModel @Inject constructor(
    private val searchUseCase: SearchDesignUseCase,
    private val catalogRepository: CatalogRepository,
    private val flagsRepository: FlagsRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow<CatalogUiState>(CatalogUiState.Loading)
    val uiState: StateFlow<CatalogUiState> = _uiState.asStateFlow()

    private val _query = MutableStateFlow("")
    val query: StateFlow<String> = _query.asStateFlow()

    /** Whether killSwitchImageSearch is false (image search route enabled). */
    val imageSearchEnabled: Boolean
        get() = !flagsRepository.getFlags().killSwitchImageSearch

    private var currentPage = 1
    private var currentQuery = ""
    private var currentMetal = ""

    init { loadRecommendations() }

    // ── Public API ────────────────────────────────────────────────────────────

    fun onQueryChange(q: String) { _query.value = q }

    fun search(query: String, metalType: String = "") {
        currentQuery  = query
        currentMetal  = metalType
        currentPage   = 1
        _uiState.value = CatalogUiState.Loading
        viewModelScope.launch {
            runSearch(query, metalType, page = 1)
        }
    }

    fun loadNextPage() {
        val current = _uiState.value as? CatalogUiState.Success ?: return
        if (current.page >= current.totalPages || current.isLoadingMore) return
        _uiState.value = current.copy(isLoadingMore = true)
        viewModelScope.launch {
            searchUseCase(
                query     = currentQuery,
                metalType = currentMetal,
                page      = current.page + 1,
                pageSize  = 20,
            ).onSuccess { result ->
                _uiState.value = CatalogUiState.Success(
                    designs    = current.designs + result.designs,
                    page       = result.page,
                    totalPages = result.totalPages,
                )
            }.onFailure {
                _uiState.value = current.copy(isLoadingMore = false)
            }
        }
    }

    fun retry() {
        _uiState.value = CatalogUiState.Loading
        if (currentQuery.isBlank()) loadRecommendations()
        else viewModelScope.launch { runSearch(currentQuery, currentMetal, currentPage) }
    }

    // ── Private helpers ───────────────────────────────────────────────────────

    private fun loadRecommendations() {
        if (flagsRepository.getFlags().killSwitchCatalog) {
            _uiState.value = CatalogUiState.Disabled
            return
        }
        viewModelScope.launch {
            runCatching {
                catalogRepository.getRecommendations(limit = 20)
            }.onSuccess { designs ->
                _uiState.value = CatalogUiState.Success(
                    designs    = designs,
                    page       = 1,
                    totalPages = 1,
                )
            }.onFailure { e ->
                // Degraded: serve cached designs
                runCatching {
                    kotlinx.coroutines.flow.first(catalogRepository.observeCache()) { true }
                }.onSuccess { cached ->
                    if (cached.isNotEmpty()) {
                        _uiState.value = CatalogUiState.Success(cached, 1, 1)
                    } else {
                        _uiState.value = CatalogUiState.Error(e.message ?: "Unable to load catalog")
                    }
                }.onFailure {
                    _uiState.value = CatalogUiState.Error(e.message ?: "Unable to load catalog")
                }
            }
        }
    }

    private suspend fun runSearch(query: String, metalType: String, page: Int) {
        searchUseCase(query = query, metalType = metalType, page = page, pageSize = 20)
            .onSuccess { result ->
                _uiState.value = CatalogUiState.Success(
                    designs    = result.designs,
                    page       = result.page,
                    totalPages = result.totalPages,
                )
            }
            .onFailure { e ->
                _uiState.value = CatalogUiState.Error(e.message ?: "Search failed")
            }
    }
}

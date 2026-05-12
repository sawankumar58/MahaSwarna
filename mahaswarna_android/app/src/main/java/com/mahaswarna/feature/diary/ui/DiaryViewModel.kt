package com.mahaswarna.feature.diary.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.feature.diary.data.DiaryRepository
import com.mahaswarna.feature.diary.domain.AddLedgerEntryUseCase
import com.mahaswarna.feature.diary.domain.Customer
import com.mahaswarna.feature.diary.domain.DiaryBill
import com.mahaswarna.feature.diary.domain.LedgerEntry
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import java.util.UUID
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class DiaryUiEvent {
    data class ShowError(val message: String) : DiaryUiEvent()
    data object CustomerSaved : DiaryUiEvent()
    data object LedgerEntrySaved : DiaryUiEvent()
    data object BillSaved : DiaryUiEvent()
}

data class CustomerWithBalance(
    val customer: Customer,
    val netBalanceInr: Double,
)

@OptIn(ExperimentalCoroutinesApi::class)
@HiltViewModel
class DiaryViewModel @Inject constructor(
    private val repository: DiaryRepository,
    private val addLedgerEntry: AddLedgerEntryUseCase,
) : ViewModel() {

    // ── Search query ──────────────────────────────────────────────────────────

    private val _searchQuery = MutableStateFlow("")
    val searchQuery: StateFlow<String> = _searchQuery.asStateFlow()

    fun onSearchQueryChange(query: String) { _searchQuery.value = query }

    // ── Customers with net balances ───────────────────────────────────────────
    //
    // FIX: previous version had a broken .let{} chain that:
    //   (a) double-evaluated stateIn with wrong generic type (Pair vs CustomerWithBalance)
    //   (b) created a second MutableStateFlow sink inside a .let on an already-stateIn'd flow
    //
    // Correct pattern: flatMapLatest → emit List<Pair>, map to CustomerWithBalance, stateIn once.

    val customersWithBalance: StateFlow<List<CustomerWithBalance>> =
        _searchQuery
            .flatMapLatest { q ->
                if (q.isBlank()) {
                    // All customers + their net balances from DiaryRepository
                    repository.observeCustomersWithBalance()
                } else {
                    // FTS search — then compute balance for each result
                    repository.searchCustomers(q).flatMapLatest { customers ->
                        flow {
                            emit(customers.map { c -> c to repository.getNetBalance(c.id) })
                        }
                    }
                }
            }
            .map { pairs -> pairs.map { (c, b) -> CustomerWithBalance(c, b) } }
            .stateIn(
                scope        = viewModelScope,
                started      = SharingStarted.WhileSubscribed(5_000),
                initialValue = emptyList(),
            )

    // ── Bills list ────────────────────────────────────────────────────────────

    val allBills: StateFlow<List<DiaryBill>> =
        _searchQuery
            .flatMapLatest { q ->
                if (q.isBlank()) repository.observeAllBills()
                else repository.searchBills(q)
            }
            .stateIn(
                scope        = viewModelScope,
                started      = SharingStarted.WhileSubscribed(5_000),
                initialValue = emptyList(),
            )

    // ── One-shot events ───────────────────────────────────────────────────────

    private val _events = MutableStateFlow<DiaryUiEvent?>(null)
    val events: StateFlow<DiaryUiEvent?> = _events.asStateFlow()
    fun eventConsumed() { _events.value = null }

    // ── Customer CRUD ─────────────────────────────────────────────────────────

    /**
     * Upserts a customer.
     * FIX: on edit (id != null), [createdAt] MUST be fetched from the existing entity,
     * not set to 0L (which incorrectly moves the record to epoch 0 in the timeline).
     * If the existing entity cannot be found (race condition), [System.currentTimeMillis()] is used.
     */
    fun saveCustomer(
        id: String? = null,
        name: String,
        phone: String,
        address: String,
        gstNumber: String,
        notes: String,
    ) {
        viewModelScope.launch {
            runCatching {
                val now = System.currentTimeMillis()
                val existingCreatedAt: Long = if (id != null) {
                    repository.getCustomerById(id)?.createdAt ?: now
                } else {
                    now
                }
                val customer = Customer(
                    id        = id ?: UUID.randomUUID().toString(),
                    name      = name.trim(),
                    phone     = phone.trim(),
                    address   = address.trim(),
                    gstNumber = gstNumber.trim().uppercase(),
                    notes     = notes.trim(),
                    createdAt = existingCreatedAt,  // FIX: was 0L on edit path
                    updatedAt = now,
                )
                repository.upsertCustomer(customer)
            }.onSuccess {
                _events.value = DiaryUiEvent.CustomerSaved
            }.onFailure { e ->
                _events.value = DiaryUiEvent.ShowError(e.message ?: "Failed to save customer")
            }
        }
    }

    fun deleteCustomer(id: String) {
        viewModelScope.launch {
            runCatching { repository.deleteCustomer(id) }.onFailure { e ->
                _events.value = DiaryUiEvent.ShowError(e.message ?: "Failed to delete customer")
            }
        }
    }

    // ── Ledger ────────────────────────────────────────────────────────────────

    fun addLedgerEntry(
        customerId: String,
        type: String,
        amountInr: Double,
        description: String,
    ) {
        viewModelScope.launch {
            runCatching {
                addLedgerEntry(
                    AddLedgerEntryUseCase.Input(
                        customerId  = customerId,
                        type        = type,
                        amountInr   = amountInr,
                        description = description,
                    )
                )
            }.onSuccess {
                _events.value = DiaryUiEvent.LedgerEntrySaved
            }.onFailure { e ->
                _events.value = DiaryUiEvent.ShowError(e.message ?: "Failed to add entry")
            }
        }
    }
}

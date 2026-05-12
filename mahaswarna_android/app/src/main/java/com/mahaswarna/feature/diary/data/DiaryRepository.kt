package com.mahaswarna.feature.diary.data

import com.mahaswarna.feature.diary.domain.Customer
import com.mahaswarna.feature.diary.domain.DiaryBill
import com.mahaswarna.feature.diary.domain.LedgerEntry
import com.mahaswarna.local.dao.BillDao
import com.mahaswarna.local.dao.CustomerDao
import com.mahaswarna.local.dao.LedgerDao
import com.mahaswarna.local.entity.BillEntity
import com.mahaswarna.local.entity.CustomerEntity
import com.mahaswarna.local.entity.LedgerEntryEntity
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

/**
 * DiaryRepository — local-only, Room-backed.
 *
 * INVARIANT: NEVER sync Diary data to the server in this class.
 * All writes are local-only. [syncedAt] on BillEntity is reserved for a future
 * optional backup feature and MUST NOT be set here.
 *
 * Mapper functions (Entity ↔ Domain) are private to this class. Generic mappers
 * in EntityMapper.kt are for cross-feature entities only (Rate, Alert, Home).
 */
@OptIn(ExperimentalCoroutinesApi::class)
@Singleton
class DiaryRepository @Inject constructor(
    private val billDao: BillDao,
    private val customerDao: CustomerDao,
    private val ledgerDao: LedgerDao,
) {

    // ── Customers ─────────────────────────────────────────────────────────────

    fun observeAllCustomers(): Flow<List<Customer>> =
        customerDao.observeAll().map { list -> list.map { it.toDomain() } }

    fun searchCustomers(query: String): Flow<List<Customer>> =
        customerDao.search("$query*").map { list -> list.map { it.toDomain() } }

    suspend fun upsertCustomer(customer: Customer) {
        customerDao.upsert(customer.toEntity())
    }

    suspend fun deleteCustomer(id: String) {
        customerDao.delete(id)
        // Cascades to ledger_entries via FK ON DELETE CASCADE (defined in LedgerEntryEntity).
    }

    suspend fun getCustomerById(id: String): Customer? =
        customerDao.getById(id)?.toDomain()

    // ── Bills ─────────────────────────────────────────────────────────────────

    fun observeAllBills(): Flow<List<DiaryBill>> =
        billDao.observeAll().map { list -> list.map { it.toDomain() } }

    fun observeBillsByCustomer(customerId: String): Flow<List<DiaryBill>> =
        billDao.observeByCustomer(customerId).map { list -> list.map { it.toDomain() } }

    fun searchBills(query: String): Flow<List<DiaryBill>> =
        billDao.search("$query*").map { list -> list.map { it.toDomain() } }

    suspend fun upsertBill(bill: DiaryBill) {
        billDao.upsert(bill.toEntity())
    }

    suspend fun deleteBill(id: String) {
        billDao.delete(id)
    }

    // ── Ledger ────────────────────────────────────────────────────────────────

    fun observeLedgerByCustomer(customerId: String): Flow<List<LedgerEntry>> =
        ledgerDao.observeByCustomer(customerId).map { list -> list.map { it.toDomain() } }

    suspend fun getNetBalance(customerId: String): Double =
        ledgerDao.getNetBalance(customerId)

    suspend fun upsertLedgerEntry(entry: LedgerEntry) {
        ledgerDao.upsert(entry.toEntity())
    }

    suspend fun deleteLedgerEntry(id: String) {
        ledgerDao.delete(id)
    }

    /**
     * Returns a [Flow] of each customer paired with their net balance.
     *
     * FIX: [ledgerDao.getNetBalance] is a `suspend` function and CANNOT be called
     * inside a plain `.map {}` lambda on a Flow (not a coroutine context).
     *
     * Correct pattern: `.flatMapLatest { customers -> flow { emit(...suspend call...) } }`
     * The `flow {}` builder IS a coroutine scope, so suspend calls are valid inside it.
     *
     * This re-evaluates all balances whenever the customers list changes (insert/delete).
     * Individual ledger balance changes do NOT re-trigger this; observe per-customer
     * ledger separately via [observeLedgerByCustomer] + [getNetBalance] in the UI layer.
     */
    fun observeCustomersWithBalance(): Flow<List<Pair<Customer, Double>>> =
        customerDao.observeAll().flatMapLatest { customers ->
            flow {
                // suspend calls allowed inside flow {} builder
                emit(
                    customers.map { entity ->
                        val balance = ledgerDao.getNetBalance(entity.id)
                        entity.toDomain() to balance
                    }
                )
            }
        }

    // ── Entity ↔ Domain mappers ───────────────────────────────────────────────

    private fun CustomerEntity.toDomain() = Customer(
        id        = id,
        name      = name,
        phone     = phone,
        address   = address,
        gstNumber = gstNumber,
        notes     = notes,
        createdAt = createdAt,
        updatedAt = updatedAt,
    )

    private fun Customer.toEntity() = CustomerEntity(
        id        = id,
        name      = name,
        phone     = phone,
        address   = address,
        gstNumber = gstNumber,
        notes     = notes,
        createdAt = createdAt,
        updatedAt = updatedAt,
    )

    private fun BillEntity.toDomain() = DiaryBill(
        id             = id,
        customerId     = customerId,
        customerName   = customerName,
        customerPhone  = customerPhone,
        itemsSummary   = itemsSummary,
        metalType      = metalType,
        totalWeightG   = totalWeightG,
        metalValueInr  = metalValueInr,
        makingCharges  = makingCharges,
        totalInr       = totalInr,
        paymentMode    = paymentMode,
        notes          = notes,
        goldRateUsed   = goldRateUsed,
        silverRateUsed = silverRateUsed,
        rateSource     = rateSource,
        createdAt      = createdAt,
        syncedAt       = syncedAt,
    )

    private fun DiaryBill.toEntity() = BillEntity(
        id             = id,
        customerId     = customerId,
        customerName   = customerName,
        customerPhone  = customerPhone,
        itemsSummary   = itemsSummary,
        metalType      = metalType,
        totalWeightG   = totalWeightG,
        metalValueInr  = metalValueInr,
        makingCharges  = makingCharges,
        totalInr       = totalInr,
        paymentMode    = paymentMode,
        notes          = notes,
        goldRateUsed   = goldRateUsed,
        silverRateUsed = silverRateUsed,
        rateSource     = rateSource,
        createdAt      = createdAt,
        syncedAt       = null,   // INVARIANT: never set syncedAt locally
    )

    private fun LedgerEntryEntity.toDomain() = LedgerEntry(
        id          = id,
        customerId  = customerId,
        billId      = billId,
        type        = type,
        amountInr   = amountInr,
        description = description,
        createdAt   = createdAt,
    )

    private fun LedgerEntry.toEntity() = LedgerEntryEntity(
        id          = id,
        customerId  = customerId,
        billId      = billId,
        type        = type,
        amountInr   = amountInr,
        description = description,
        createdAt   = createdAt,
    )
}

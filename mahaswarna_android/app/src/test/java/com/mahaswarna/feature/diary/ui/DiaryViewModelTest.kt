package com.mahaswarna.feature.diary.ui

import app.cash.turbine.test
import com.mahaswarna.feature.diary.data.DiaryRepository
import com.mahaswarna.feature.diary.domain.AddLedgerEntryUseCase
import com.mahaswarna.feature.diary.domain.Customer
import com.mahaswarna.feature.diary.domain.GetCustomerLedgerUseCase
import com.mahaswarna.feature.diary.domain.LedgerEntry
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Unit tests for [DiaryViewModel].
 *
 * Architecture invariants under test:
 *   - Diary reads go through repository → ViewModel state (no direct DB access)
 *   - Adding a ledger entry calls AddLedgerEntryUseCase exactly once
 *   - Customers list is derived from the repository Flow
 *   - Search query filters customers by name (case-insensitive)
 */
@OptIn(ExperimentalCoroutinesApi::class)
class DiaryViewModelTest {

    private val testDispatcher   = StandardTestDispatcher()
    private lateinit var repository: DiaryRepository
    private lateinit var addLedgerEntry: AddLedgerEntryUseCase
    private lateinit var getCustomerLedger: GetCustomerLedgerUseCase
    private lateinit var viewModel: DiaryViewModel

    private val sampleCustomers = listOf(
        Customer(id = "c1", name = "Ramesh Jewellers", phone = "9876543210",
                 address = "Mumbai", gstNumber = "", notes = "", createdAt = 1000L, updatedAt = 1000L),
        Customer(id = "c2", name = "Suresh Gold House", phone = "9123456789",
                 address = "Pune",   gstNumber = "", notes = "", createdAt = 2000L, updatedAt = 2000L),
    )

    @Before
    fun setup() {
        Dispatchers.setMain(testDispatcher)
        repository       = mockk(relaxed = true)
        addLedgerEntry   = mockk(relaxed = true)
        getCustomerLedger = mockk(relaxed = true)

        every { repository.observeCustomers() } returns flowOf(sampleCustomers)

        viewModel = DiaryViewModel(repository, addLedgerEntry, getCustomerLedger)
    }

    @After
    fun teardown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `customers list populated from repository on init`() = runTest {
        testDispatcher.scheduler.advanceUntilIdle()

        val customers = viewModel.uiState.value.customers
        assertEquals(2, customers.size)
        assertEquals("Ramesh Jewellers", customers[0].name)
    }

    @Test
    fun `search query filters customers by name case-insensitively`() = runTest {
        val allCustomers = listOf(
            Customer("c1","Ramesh Jewellers","","","","",1000L,1000L),
            Customer("c2","Suresh Gold House","","","","",2000L,2000L),
            Customer("c3","Rajiv Ornaments","","","","",3000L,3000L),
        )
        every { repository.observeCustomers() } returns flowOf(allCustomers)
        viewModel = DiaryViewModel(repository, addLedgerEntry, getCustomerLedger)

        viewModel.onSearchQueryChange("ra")
        testDispatcher.scheduler.advanceUntilIdle()

        val filtered = viewModel.uiState.value.filteredCustomers
        assertEquals(2, filtered.size)
        assertTrue(filtered.all { it.name.contains("ra", ignoreCase = true) })
    }

    @Test
    fun `empty search shows all customers`() = runTest {
        testDispatcher.scheduler.advanceUntilIdle()
        viewModel.onSearchQueryChange("")
        testDispatcher.scheduler.advanceUntilIdle()

        assertEquals(2, viewModel.uiState.value.filteredCustomers.size)
    }

    @Test
    fun `addLedgerEntry calls use case exactly once`() = runTest {
        coEvery { addLedgerEntry(any()) } returns Result.success(Unit)

        viewModel.addLedgerEntry(
            customerId  = "c1",
            amountInr   = 5000.0,
            type        = "credit",
            description = "advance",
        )
        testDispatcher.scheduler.advanceUntilIdle()

        coVerify(exactly = 1) { addLedgerEntry(any()) }
    }

    @Test
    fun `addLedgerEntry sets error state on failure`() = runTest {
        coEvery { addLedgerEntry(any()) } returns Result.failure(RuntimeException("DB error"))

        viewModel.uiState.test {
            skipItems(1) // initial
            viewModel.addLedgerEntry("c1", 1000.0, "debit", "purchase")
            val loading = awaitItem()
            assertTrue("Expected loading state", loading.isLoading)
            val error = awaitItem()
            assertTrue("Expected error state", error.errorMessage != null)
        }
    }
}

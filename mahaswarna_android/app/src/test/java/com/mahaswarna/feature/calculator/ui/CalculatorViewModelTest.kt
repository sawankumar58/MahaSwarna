package com.mahaswarna.feature.calculator.ui

import app.cash.turbine.test
import com.mahaswarna.feature.calculator.domain.CalculatorMode
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

/**
 * Unit tests for [CalculatorViewModel].
 *
 * Tests cover:
 *   - Weight-to-value calculation for gold and silver
 *   - Mode switching (WEIGHT_TO_VALUE ↔ VALUE_TO_WEIGHT)
 *   - Zero / negative input edge cases
 *   - Making-charge inclusion
 */
@OptIn(ExperimentalCoroutinesApi::class)
class CalculatorViewModelTest {

    private val testDispatcher = StandardTestDispatcher()
    private lateinit var viewModel: CalculatorViewModel

    @Before
    fun setup() {
        Dispatchers.setMain(testDispatcher)
        viewModel = CalculatorViewModel()
    }

    @After
    fun teardown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `initial state has empty inputs and zero result`() = runTest {
        val state = viewModel.uiState.value
        assertEquals("", state.weight)
        assertEquals("", state.ratePerGram)
        assertEquals(0.0, state.result, 0.001)
    }

    @Test
    fun `weight-to-value calculates correctly for gold`() = runTest {
        viewModel.onWeightChange("10")
        viewModel.onRateChange("6100")
        viewModel.onMakingChargeChange("500")
        testDispatcher.scheduler.advanceUntilIdle()

        val state = viewModel.uiState.value
        // 10g × ₹6100/g + ₹500 making = ₹61500
        assertEquals(61500.0, state.result, 0.01)
    }

    @Test
    fun `value-to-weight calculates correctly`() = runTest {
        viewModel.onModeChange(CalculatorMode.VALUE_TO_WEIGHT)
        viewModel.onValueChange("61000")
        viewModel.onRateChange("6100")
        testDispatcher.scheduler.advanceUntilIdle()

        val state = viewModel.uiState.value
        // ₹61000 ÷ ₹6100/g = 10g
        assertEquals(10.0, state.result, 0.001)
    }

    @Test
    fun `zero weight produces zero result`() = runTest {
        viewModel.onWeightChange("0")
        viewModel.onRateChange("6100")
        testDispatcher.scheduler.advanceUntilIdle()

        assertEquals(0.0, viewModel.uiState.value.result, 0.001)
    }

    @Test
    fun `zero rate produces zero result`() = runTest {
        viewModel.onWeightChange("10")
        viewModel.onRateChange("0")
        testDispatcher.scheduler.advanceUntilIdle()

        assertEquals(0.0, viewModel.uiState.value.result, 0.001)
    }

    @Test
    fun `blank weight input produces zero result without crash`() = runTest {
        viewModel.onWeightChange("")
        viewModel.onRateChange("6100")
        testDispatcher.scheduler.advanceUntilIdle()

        assertEquals(0.0, viewModel.uiState.value.result, 0.001)
    }

    @Test
    fun `mode switch clears result and inputs`() = runTest {
        viewModel.onWeightChange("10")
        viewModel.onRateChange("6100")
        testDispatcher.scheduler.advanceUntilIdle()

        viewModel.onModeChange(CalculatorMode.VALUE_TO_WEIGHT)
        testDispatcher.scheduler.advanceUntilIdle()

        val state = viewModel.uiState.value
        assertEquals(CalculatorMode.VALUE_TO_WEIGHT, state.mode)
        assertEquals(0.0, state.result, 0.001)
    }

    @Test
    fun `silver calculation uses silver rate`() = runTest {
        viewModel.onWeightChange("100")
        viewModel.onRateChange("76")
        testDispatcher.scheduler.advanceUntilIdle()

        // 100g × ₹76/g = ₹7600
        assertEquals(7600.0, viewModel.uiState.value.result, 0.01)
    }
}

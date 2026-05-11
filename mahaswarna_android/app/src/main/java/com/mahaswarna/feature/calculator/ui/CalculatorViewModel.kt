package com.mahaswarna.feature.calculator.ui

import androidx.lifecycle.ViewModel
import com.mahaswarna.feature.calculator.domain.CalculatorMode
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.map
import javax.inject.Inject

/**
 * CalculatorViewModel — pure local computation, no repository, no network.
 *
 * Rate values are injected from the caller (HomeViewModel / RatesViewModel)
 * and kept in [goldRatePerGram] / [silverRatePerGram].
 *
 * All calculation results are derived synchronously from current StateFlow values.
 */
@HiltViewModel
class CalculatorViewModel @Inject constructor() : ViewModel() {

    // ── Selected metal ────────────────────────────────────────────────────────
    private val _selectedMetal = MutableStateFlow("gold") // "gold" | "silver"
    val selectedMetal: StateFlow<String> = _selectedMetal.asStateFlow()

    // ── Live rates (injected from parent screen) ──────────────────────────────
    private val _goldRatePerGram = MutableStateFlow(0.0)
    private val _silverRatePerGram = MutableStateFlow(0.0)

    /** Call from the parent composable when rates update from Room/WS. */
    fun updateRates(goldPerGram: Double, silverPerGram: Double) {
        _goldRatePerGram.value = goldPerGram
        _silverRatePerGram.value = silverPerGram
        recalculate()
    }

    // ── Mode ──────────────────────────────────────────────────────────────────
    private val _mode = MutableStateFlow<CalculatorMode>(CalculatorMode.WeightToPrice)
    val mode: StateFlow<CalculatorMode> = _mode.asStateFlow()

    // ── Inputs ────────────────────────────────────────────────────────────────
    private val _weightInput = MutableStateFlow("")
    val weightInput: StateFlow<String> = _weightInput.asStateFlow()

    private val _priceInput = MutableStateFlow("")
    val priceInput: StateFlow<String> = _priceInput.asStateFlow()

    private val _makingChargesPercent = MutableStateFlow("")
    val makingChargesPercent: StateFlow<String> = _makingChargesPercent.asStateFlow()

    // ── Results ───────────────────────────────────────────────────────────────

    data class CalculatorResult(
        val metalValue: Double? = null,       // price of metal alone
        val makingChargesValue: Double? = null,
        val totalValue: Double? = null,        // metal + making charges (MakingCharges mode)
        val weightGrams: Double? = null,       // PriceToWeight mode result
        val errorMessage: String? = null,
    )

    private val _result = MutableStateFlow(CalculatorResult())
    val result: StateFlow<CalculatorResult> = _result.asStateFlow()

    // ── Actions ───────────────────────────────────────────────────────────────

    fun selectMetal(metal: String) {
        _selectedMetal.value = metal
        recalculate()
    }

    fun selectMode(mode: CalculatorMode) {
        _mode.value = mode
        _weightInput.value = ""
        _priceInput.value = ""
        _makingChargesPercent.value = ""
        _result.value = CalculatorResult()
    }

    fun onWeightInputChange(value: String) {
        _weightInput.value = value
        recalculate()
    }

    fun onPriceInputChange(value: String) {
        _priceInput.value = value
        recalculate()
    }

    fun onMakingChargesChange(value: String) {
        _makingChargesPercent.value = value
        recalculate()
    }

    fun clearInputs() {
        _weightInput.value = ""
        _priceInput.value = ""
        _makingChargesPercent.value = ""
        _result.value = CalculatorResult()
    }

    // ── Computation ───────────────────────────────────────────────────────────

    private fun recalculate() {
        val ratePerGram = if (_selectedMetal.value == "gold") _goldRatePerGram.value
                          else _silverRatePerGram.value

        if (ratePerGram <= 0.0) {
            _result.value = CalculatorResult(errorMessage = "Rate not available — check your connection")
            return
        }

        _result.value = when (_mode.value) {
            CalculatorMode.WeightToPrice  -> calculateWeightToPrice(ratePerGram)
            CalculatorMode.PriceToWeight  -> calculatePriceToWeight(ratePerGram)
            CalculatorMode.MakingCharges  -> calculateMakingCharges(ratePerGram)
        }
    }

    private fun calculateWeightToPrice(ratePerGram: Double): CalculatorResult {
        val weight = _weightInput.value.toDoubleOrNull()
            ?: return CalculatorResult()
        if (weight <= 0) return CalculatorResult(errorMessage = "Weight must be greater than 0")
        val total = weight * ratePerGram
        return CalculatorResult(metalValue = total, totalValue = total)
    }

    private fun calculatePriceToWeight(ratePerGram: Double): CalculatorResult {
        val budget = _priceInput.value.toDoubleOrNull()
            ?: return CalculatorResult()
        if (budget <= 0) return CalculatorResult(errorMessage = "Budget must be greater than 0")
        val weight = budget / ratePerGram
        return CalculatorResult(weightGrams = weight)
    }

    private fun calculateMakingCharges(ratePerGram: Double): CalculatorResult {
        val weight = _weightInput.value.toDoubleOrNull()
            ?: return CalculatorResult()
        if (weight <= 0) return CalculatorResult(errorMessage = "Weight must be greater than 0")

        val metalValue = weight * ratePerGram

        val makingPct = _makingChargesPercent.value.toDoubleOrNull()
        val makingValue = if (makingPct != null && makingPct >= 0) {
            metalValue * (makingPct / 100.0)
        } else null

        val total = if (makingValue != null) metalValue + makingValue else metalValue

        return CalculatorResult(
            metalValue         = metalValue,
            makingChargesValue = makingValue,
            totalValue         = total,
        )
    }
}

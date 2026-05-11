package com.mahaswarna.feature.calculator.domain

/**
 * Calculator operating modes.
 *
 * Calculator is pure local computation — no repository, no network.
 * All rates are passed in from the current [RateInfo] observed by the parent screen.
 *
 * [WeightToPrice] — given weight in grams, compute total INR value.
 * [PriceToWeight] — given total INR budget, compute equivalent weight in grams.
 * [MakingCharges] — compute making charges as a percentage of metal value,
 *                   then add to get a final jewellery price.
 */
sealed class CalculatorMode {

    /** Input: weight (g). Output: total value in INR. */
    data object WeightToPrice : CalculatorMode()

    /** Input: budget in INR. Output: weight in grams. */
    data object PriceToWeight : CalculatorMode()

    /**
     * Input: weight (g) + making-charges percentage.
     * Output: metal value + making charges = total jewellery price.
     */
    data object MakingCharges : CalculatorMode()

    fun displayName(): String = when (this) {
        WeightToPrice  -> "Weight → Price"
        PriceToWeight  -> "Budget → Weight"
        MakingCharges  -> "Making Charges"
    }
}

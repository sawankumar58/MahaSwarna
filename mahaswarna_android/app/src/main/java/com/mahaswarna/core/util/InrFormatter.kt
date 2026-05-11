package com.mahaswarna.core.util

import java.text.NumberFormat
import java.util.Locale

/**
 * Formats monetary values using Indian lakh/crore grouping (en_IN locale).
 *
 * All price fields from the backend are Double (price per gram in INR).
 * Use [formatRate] for rate display and [formatPrice] for invoice/calculator amounts.
 *
 * IMPORTANT: Always use Locale("en", "IN") — never Locale.getDefault() or Locale.US.
 * Indian number grouping (1,00,000 vs 100,000) is locale-specific and must be explicit.
 */
object InrFormatter {

    private val INR_LOCALE = Locale("en", "IN")

    /**
     * Formats a per-gram gold/silver rate.
     * e.g. 6150.5 → "₹6,150.50/g"
     */
    fun formatRate(pricePerGram: Double): String {
        val nf = NumberFormat.getNumberInstance(INR_LOCALE).apply {
            minimumFractionDigits = 2
            maximumFractionDigits = 2
        }
        return "₹${nf.format(pricePerGram)}/g"
    }

    /**
     * Formats a general INR amount with 2 decimal places.
     * e.g. 125000.0 → "₹1,25,000.00"
     */
    fun formatPrice(amount: Double): String {
        val nf = NumberFormat.getNumberInstance(INR_LOCALE).apply {
            minimumFractionDigits = 2
            maximumFractionDigits = 2
        }
        return "₹${nf.format(amount)}"
    }

    /**
     * Formats a weight in grams.
     * e.g. 22.5 → "22.500 g"
     * Uses 3 decimal places to match jewellery precision conventions.
     */
    fun formatWeight(grams: Double): String {
        val nf = NumberFormat.getNumberInstance(INR_LOCALE).apply {
            minimumFractionDigits = 3
            maximumFractionDigits = 3
        }
        return "${nf.format(grams)} g"
    }

    /**
     * Formats a short rate label without the /g suffix (e.g. for chart axes).
     * e.g. 6150.5 → "₹6,150.50"
     */
    fun formatRateShort(pricePerGram: Double): String {
        val nf = NumberFormat.getNumberInstance(INR_LOCALE).apply {
            minimumFractionDigits = 2
            maximumFractionDigits = 2
        }
        return "₹${nf.format(pricePerGram)}"
    }

    /**
     * Formats a threshold value for display in alert rows and bottom sheets.
     * Strips trailing zeroes for cleaner display: 6200.00 → "₹6,200", 6200.50 → "₹6,200.50"
     */
    fun formatThreshold(threshold: Double): String {
        val nf = NumberFormat.getNumberInstance(INR_LOCALE).apply {
            minimumFractionDigits = 0
            maximumFractionDigits = 2
        }
        return "₹${nf.format(threshold)}"
    }
}

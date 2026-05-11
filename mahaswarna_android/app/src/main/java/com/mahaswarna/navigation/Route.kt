package com.mahaswarna.navigation

import kotlinx.serialization.Serializable

/**
 * Typed route definitions for Compose Navigation.
 * All routes with nav args use @Serializable for type-safe nav.
 */
sealed class Route {
    @Serializable data object Splash          : Route()
    @Serializable data object Login           : Route()
    @Serializable data object Consent         : Route()
    @Serializable data object Home            : Route()
    @Serializable data object Rates           : Route()
    @Serializable data object Catalog         : Route()
    @Serializable data object Diary           : Route()
    @Serializable data object Profile         : Route()
    @Serializable data object ShopSettings    : Route()
    @Serializable data object RegisterShop    : Route()
    @Serializable data object UpdateRequired  : Route()  // non-dismissible on 410

    @Serializable
    data class Calculator(
        val goldRate: Double,
        val silverRate: Double,
        val isStale: Boolean,
    ) : Route()

    @Serializable
    data class BillPrint(
        val goldRate: Double?,
        val silverRate: Double?,
        val isStale: Boolean,
    ) : Route()

    @Serializable
    data class CustomerLedgerDetail(val customerId: String) : Route()

    @Serializable
    data class CustomerDetail(val customerId: String) : Route()

    @Serializable
    data class EditShop(val shopId: String) : Route()

    @Serializable
    data class BannerPicker(val shopId: String) : Route()

    // Route.ImageSearch is intentionally ABSENT until killSwitchImageSearch == false.
    // Add this composable block in NavHost only in the same release that enables
    // the backend endpoint and sets killSwitchImageSearch = false.
}

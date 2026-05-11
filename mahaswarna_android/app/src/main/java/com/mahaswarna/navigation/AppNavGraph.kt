package com.mahaswarna.navigation

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import com.mahaswarna.feature.auth.ui.ConsentScreen
import com.mahaswarna.feature.auth.ui.SplashScreen
import com.mahaswarna.feature.auth.ui.UpdateRequiredScreen

/**
 * NavHost with all routes.
 * navController is hoisted in MainActivity.setContent — NOT inside AppNavGraph.
 *
 * Back-stack rules:
 *   Calculator → back → RatesDashboardScreen (NOT Home)
 *   BillPrint  → back → RatesDashboard or Calculator
 *   SessionEvent.LoggedOut → navigate Login (observed in MainActivity)
 *   ApiError.VersionDeprecated →
 *     navController.navigate(Route.UpdateRequired) {
 *         popUpTo(Route.Home) { inclusive = true }
 *     }
 *     Required: popUpTo clears the entire back stack so the user cannot
 *     back-navigate out of UpdateRequiredScreen.
 *
 * Route.ImageSearch is absent until killSwitchImageSearch == false.
 *
 * Phase 2 screens (currently stubs below):
 *   ProfileScreen, ShopSettingsScreen, EditShopScreen, BannerPickerScreen
 */
@Composable
fun AppNavGraph(
    navController: NavHostController,
) {
    NavHost(
        navController = navController,
        startDestination = Route.Splash,
    ) {
        composable<Route.Splash> {
            SplashScreen(navController = navController)
        }

        composable<Route.Login> {
            com.mahaswarna.feature.auth.ui.PhoneEntryScreen(navController = navController)
        }

        composable<Route.OtpEntry> {
            com.mahaswarna.feature.auth.ui.OtpScreen(navController = navController)
        }

        composable<Route.Consent> {
            // Back navigation disabled — see ConsentScreen
            ConsentScreen(navController = navController)
        }

        composable<Route.Home> {
            com.mahaswarna.feature.home.ui.HomeScreen(navController = navController)
        }

        composable<Route.Rates> {
            com.mahaswarna.feature.rates.ui.RatesDashboardScreen(navController = navController)
        }

        composable<Route.RateHistory> {
            com.mahaswarna.feature.rates.ui.RateHistoryScreen(
                cityId = "mumbai",
                navController = navController,
            )
        }

        composable<Route.Calculator> {
            com.mahaswarna.feature.calculator.ui.CalculatorScreen(navController = navController)
        }

        composable<Route.BillPrint> {
            com.mahaswarna.feature.marketplace.ui.BillPrintScreen(navController = navController)
        }

        composable<Route.Catalog> {
            com.mahaswarna.feature.catalog.ui.CatalogScreen(navController = navController)
        }

        composable<Route.Diary> {
            com.mahaswarna.feature.diary.ui.DiaryScreen(navController = navController)
        }

        composable<Route.CustomerLedgerDetail> {
            com.mahaswarna.feature.diary.ui.CustomerLedgerDetailScreen(navController = navController)
        }

        composable<Route.CustomerDetail> {
            com.mahaswarna.feature.diary.ui.CustomerDetailScreen(navController = navController)
        }

        composable<Route.RegisterShop> {
            com.mahaswarna.feature.marketplace.ui.RegisterShopScreen(navController = navController)
        }

        composable<Route.ShopSettings> {
            // Phase 2 — ShopSettingsScreen (marketplace feature)
            // Replace with: com.mahaswarna.feature.marketplace.ui.ShopSettingsScreen(navController)
            Text("Shop Settings — coming in Phase 2")
        }

        composable<Route.EditShop> {
            // Phase 2 — EditShopScreen (marketplace feature)
            // Replace with: com.mahaswarna.feature.marketplace.ui.EditShopScreen(navController)
            Text("Edit Shop — coming in Phase 2")
        }

        composable<Route.BannerPicker> {
            // Phase 2 — BannerPickerScreen (marketplace feature)
            // Replace with: com.mahaswarna.feature.marketplace.ui.ShopBannerScreen(navController)
            Text("Banner Picker — coming in Phase 2")
        }

        composable<Route.Profile> {
            // Phase 2 — ProfileScreen
            // Replace with: com.mahaswarna.feature.profile.ui.ProfileScreen(navController)
            Text("Profile — coming in Phase 2")
        }

        composable<Route.UpdateRequired> {
            // Non-dismissible — BackHandler + popUpTo(Home) { inclusive = true }
            // ensures user cannot back-navigate out of this screen.
            UpdateRequiredScreen()
        }

        // Route.ImageSearch — intentionally ABSENT while killSwitchImageSearch == true.
        // Add this composable block only in the release that enables the endpoint.
    }
}

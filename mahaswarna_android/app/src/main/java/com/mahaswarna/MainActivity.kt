package com.mahaswarna

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.LaunchedEffect
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.navigation.compose.rememberNavController
import com.google.firebase.crashlytics.FirebaseCrashlytics
import com.mahaswarna.core.auth.SessionEvent
import com.mahaswarna.core.auth.SessionManager
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.core.network.ApiError
import com.mahaswarna.core.network.ApiErrorBus
import com.mahaswarna.core.websocket.WsClient
import com.mahaswarna.local.AppDatabase
import com.mahaswarna.navigation.AppNavGraph
import com.mahaswarna.navigation.Route
import com.mahaswarna.theme.MahaSwarnTheme
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Single-activity. SplashScreen API (OS-level — zero Compose frames on cold start).
 *
 * Responsibilities:
 *   1. Observe SessionEvent.LoggedOut → clearSessionData() + navigate Login.
 *   2. Observe ApiError.VersionDeprecated → navigate UpdateRequired (back stack cleared).
 *   3. Read deep_link_screen FCM Intent extra → navigate(Route.Rates) when "rates".
 *   4. WS lifecycle: connect at T+80ms after content is set (JWT pre-warmed in background).
 *
 * TokenStore is NOT accessed in onCreate() — lazy access in AuthInterceptor
 * absorbs the 50–200ms TEE overhead on budget devices without blocking the first frame.
 */
@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    @Inject lateinit var sessionManager: SessionManager
    @Inject lateinit var tokenStore: TokenStore
    @Inject lateinit var database: AppDatabase
    @Inject lateinit var wsClient: WsClient

    private val activityScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    override fun onCreate(savedInstanceState: Bundle?) {
        installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            MahaSwarnTheme {
                val navController = rememberNavController()

                // Observe session events (LoggedOut → Login)
                LaunchedEffect(Unit) {
                    sessionManager.events.collect { event ->
                        when (event) {
                            is SessionEvent.LoggedOut -> {
                                database.clearSessionData()
                                navController.navigate(Route.Login) {
                                    popUpTo(Route.Home) { inclusive = true }
                                }
                            }
                        }
                    }
                }

                // Observe VersionDeprecated → UpdateRequired (non-dismissible)
                LaunchedEffect(Unit) {
                    ApiErrorBus.events.collect { error ->
                        if (error is ApiError.VersionDeprecated) {
                            navController.navigate(Route.UpdateRequired) {
                                popUpTo(Route.Home) { inclusive = true }
                            }
                        }
                    }
                }

                // Handle FCM deep-link from Intent
                LaunchedEffect(Unit) {
                    handleDeepLink(intent, navController)
                }

                AppNavGraph(navController = navController)
            }
        }

        // WS connect at T+80ms: JWT pre-warm runs in background; WS follows regardless.
        activityScope.launch {
            delay(80)
            val token = tokenStore.getAccessToken()
            if (token != null) {
                // Pre-warm: refresh if expiry within 12 min
                runCatching {
                    if (sessionManager.shouldRefresh()) sessionManager.refresh()
                }.onFailure {
                    // Pre-warm failure is non-fatal — WS connect proceeds
                    FirebaseCrashlytics.getInstance().log("JWT pre-warm failed: ${it.message}")
                }
                val freshToken = tokenStore.getAccessToken() ?: token
                wsClient.connect(freshToken, BuildConfig.WS_URL)
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        // FCM tap while app is foregrounded — re-handle deep link.
        // In production, route through a SharedFlow in a deep-link ViewModel
        // and collect it inside the Compose content block.
    }

    override fun onDestroy() {
        super.onDestroy()
        wsClient.disconnect()
        activityScope.cancel()
    }

    private fun handleDeepLink(
        intent: Intent?,
        navController: androidx.navigation.NavController,
    ) {
        val screen = intent?.getStringExtra("deep_link_screen") ?: return
        if (screen == "rates") {
            navController.navigate(Route.Rates)
        }
    }
}

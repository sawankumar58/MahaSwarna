package com.mahaswarna.feature.auth.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.platform.LocalContext
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.navigation.NavController
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.navigation.Route
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import java.io.File
import javax.inject.Inject

/**
 * Routing logic:
 *   1. Check token_exists_marker FILE (not TokenStore — avoids 50–200ms Keystore TEE hit).
 *      If absent → navigate Login.
 *   2. Read PreferenceStore.getConsentAccepted() (DataStore — async, fast local file).
 *      If false → navigate Consent.
 *   3. Otherwise → navigate Home.
 *
 * NO network call is made before routing. Feature flags are refreshed by HomeViewModel.
 */
@HiltViewModel
class SplashViewModel @Inject constructor(
    private val preferenceStore: PreferenceStore,
) : ViewModel() {

    sealed class SplashDestination {
        data object Pending  : SplashDestination()
        data object Login    : SplashDestination()
        data object Consent  : SplashDestination()
        data object Home     : SplashDestination()
    }

    private val _destination = MutableStateFlow<SplashDestination>(SplashDestination.Pending)
    val destination: StateFlow<SplashDestination> = _destination.asStateFlow()

    fun resolve(filesDir: File) {
        viewModelScope.launch {
            // Step 1 — marker file check (no Keystore access)
            val hasToken = File(filesDir, "token_exists_marker").exists()
            if (!hasToken) {
                _destination.value = SplashDestination.Login
                return@launch
            }
            // Step 2 — consent check (async DataStore read, non-blocking)
            val consentAccepted = preferenceStore.getConsentAccepted().first()
            _destination.value = if (consentAccepted) SplashDestination.Home
                                  else SplashDestination.Consent
        }
    }
}

/**
 * SplashScreen composable.
 *
 * The OS SplashScreen API (installSplashScreen() in MainActivity) shows the branded launch
 * image for zero Compose frames. This composable contains no visible UI — it performs the
 * routing decision and immediately navigates.
 *
 * Never use runBlocking on the main thread. Never make a network call here.
 */
@Composable
fun SplashScreen(
    navController: NavController,
    viewModel: SplashViewModel = hiltViewModel(),
) {
    val context = LocalContext.current

    LaunchedEffect(Unit) {
        viewModel.resolve(context.filesDir)
        viewModel.destination.collect { dest ->
            when (dest) {
                SplashViewModel.SplashDestination.Pending -> return@collect
                SplashViewModel.SplashDestination.Login -> {
                    navController.navigate(Route.Login) {
                        popUpTo(Route.Splash) { inclusive = true }
                    }
                }
                SplashViewModel.SplashDestination.Consent -> {
                    navController.navigate(Route.Consent) {
                        popUpTo(Route.Splash) { inclusive = true }
                    }
                }
                SplashViewModel.SplashDestination.Home -> {
                    navController.navigate(Route.Home) {
                        popUpTo(Route.Splash) { inclusive = true }
                    }
                }
            }
        }
    }
}

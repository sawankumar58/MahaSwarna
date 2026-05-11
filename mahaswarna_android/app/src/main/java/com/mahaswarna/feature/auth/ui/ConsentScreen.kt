package com.mahaswarna.feature.auth.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.feature.auth.data.AuthRepository
import com.mahaswarna.feature.auth.data.ConsentType
import com.mahaswarna.navigation.Route
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── ViewModel ─────────────────────────────────────────────────────────────────

sealed class ConsentUiState {
    data object Idle       : ConsentUiState()
    data object Submitting : ConsentUiState()
    data object Done       : ConsentUiState()
    data class  Error(val message: String) : ConsentUiState()
}

/**
 * ConsentViewModel.
 *
 * "I Agree" triggers EXACTLY TWO sequential POST /user/consent calls:
 *   1. consentType: "privacy_policy"
 *   2. consentType: "tos"
 * Both must succeed before PreferenceStore.setConsentAccepted(true) is written.
 *
 * "ai_disclaimer" is NEVER posted — display-only.
 * UNIT TEST: ConsentViewModelTest must assert exactly 2 calls and that "ai_disclaimer"
 * is never passed to logConsent(). See ConsentViewModelTest.kt.
 */
@HiltViewModel
class ConsentViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val preferenceStore: PreferenceStore,
) : ViewModel() {

    private val _uiState = MutableStateFlow<ConsentUiState>(ConsentUiState.Idle)
    val uiState: StateFlow<ConsentUiState> = _uiState.asStateFlow()

    fun acceptConsent() {
        _uiState.value = ConsentUiState.Submitting
        viewModelScope.launch {
            runCatching {
                // Call 1: privacy_policy
                authRepository.logConsent(ConsentType.PRIVACY_POLICY)
                // Call 2: tos — only after call 1 succeeds
                authRepository.logConsent(ConsentType.TOS)
                // Only write after BOTH calls succeed
                preferenceStore.setConsentAccepted(true)
            }.onSuccess {
                _uiState.value = ConsentUiState.Done
            }.onFailure { e ->
                _uiState.value = ConsentUiState.Error(e.message ?: "Failed to record consent")
            }
        }
    }
}

// ── Screen ────────────────────────────────────────────────────────────────────

/**
 * Full-screen consent route (Route.Consent) — NOT a dialog.
 *
 * Back navigation is disabled. User must tap "I Agree" to proceed.
 * Shows Privacy Policy, Terms of Service, and AI Disclaimer text.
 * AI Disclaimer is DISPLAY-ONLY — generates no backend call.
 */
@Composable
fun ConsentScreen(
    navController: NavController,
    viewModel: ConsentViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()

    // Back navigation disabled — user must consent or close the app
    BackHandler(enabled = true) { /* blocked */ }

    LaunchedEffect(uiState) {
        when (val s = uiState) {
            is ConsentUiState.Done ->
                navController.navigate(Route.Home) {
                    popUpTo(Route.Consent) { inclusive = true }
                }
            is ConsentUiState.Error ->
                scope.launch { snackbarHostState.showSnackbar(s.message) }
            else -> Unit
        }
    }

    Scaffold(snackbarHost = { SnackbarHost(snackbarHostState) }) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 24.dp)
                .verticalScroll(rememberScrollState()),
        ) {
            Spacer(Modifier.height(32.dp))
            Text("Before you continue", style = MaterialTheme.typography.headlineMedium)
            Spacer(Modifier.height(16.dp))

            // Privacy Policy
            Text("Privacy Policy", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(8.dp))
            Text(
                "MahaSwarna collects your phone number and usage data to provide gold and " +
                "silver rate information and jewellery market tools. Your data is not sold " +
                "to third parties. See our full Privacy Policy for details.",
                style = MaterialTheme.typography.bodyMedium,
            )

            Spacer(Modifier.height(24.dp))

            // Terms of Service
            Text("Terms of Service", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(8.dp))
            Text(
                "By using MahaSwarna, you agree to use the platform only for lawful purposes " +
                "related to gold and silver trade. Rate information is indicative only — " +
                "always verify with your local sarafa market before transacting.",
                style = MaterialTheme.typography.bodyMedium,
            )

            Spacer(Modifier.height(24.dp))

            // AI Disclaimer — DISPLAY ONLY — no backend call
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)) {
                Column(Modifier.padding(16.dp)) {
                    Text("AI Rate Generation Disclaimer", style = MaterialTheme.typography.titleSmall)
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "Gold and silver rates in MahaSwarna are generated using AI (Gemini). " +
                        "These rates are indicative estimates based on available market data and " +
                        "may differ from actual sarafa market rates. Always confirm rates " +
                        "directly with your local market before transacting. MahaSwarna accepts " +
                        "no liability for rate discrepancies.",
                        style = MaterialTheme.typography.bodySmall,
                    )
                }
            }

            Spacer(Modifier.height(32.dp))

            val isSubmitting = uiState is ConsentUiState.Submitting
            Button(
                onClick = { viewModel.acceptConsent() },
                enabled = !isSubmitting,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (isSubmitting) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                else Text("I Agree")
            }

            Spacer(Modifier.height(16.dp))
        }
    }
}

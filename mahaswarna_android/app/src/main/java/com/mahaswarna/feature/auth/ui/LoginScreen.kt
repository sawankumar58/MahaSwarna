package com.mahaswarna.feature.auth.ui

import android.app.Activity
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.mahaswarna.core.network.ApiConstants
import com.mahaswarna.navigation.Route
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ── PhoneEntryScreen ─────────────────────────────────────────────────────────

/**
 * Phone number entry. On "Send OTP" tapped: LoginViewModel.sendOtp(phone, activity).
 * Routes to OtpScreen when state transitions to OtpEntry.
 * Shows error snackbar on LoginState.Error.
 * Shows blocking screen on HTTP 403 device_not_trusted (never navigates to Home).
 */
@Composable
fun PhoneEntryScreen(
    navController: NavController,
    viewModel: LoginViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current as Activity
    val snackbarHostState = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()

    LaunchedEffect(state) {
        when (val s = state) {
            is LoginState.OtpEntry -> { /* NavHost observes this — OtpScreen reads same VM */ }
            is LoginState.Error    -> scope.launch { snackbarHostState.showSnackbar(s.message) }
            else -> Unit
        }
    }

    Scaffold(snackbarHost = { SnackbarHost(snackbarHostState) }) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 24.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("MahaSwarna", style = MaterialTheme.typography.headlineMedium)
            Spacer(Modifier.height(8.dp))
            Text("Enter your mobile number to continue", style = MaterialTheme.typography.bodyMedium)
            Spacer(Modifier.height(32.dp))

            var phone by remember { mutableStateOf("") }

            OutlinedTextField(
                value = phone,
                onValueChange = { if (it.length <= 10 && it.all(Char::isDigit)) phone = it },
                label = { Text("Mobile number") },
                prefix = { Text("+91 ") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(Modifier.height(24.dp))

            val isLoading = state is LoginState.SendingOtp
            Button(
                onClick = { viewModel.sendOtp(phone, context) },
                enabled = phone.length == 10 && !isLoading,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (isLoading) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                else Text("Send OTP")
            }

            // Route to OTP screen when provider is known
            if (state is LoginState.OtpEntry) {
                LaunchedEffect(Unit) {
                    navController.navigate(Route.OtpEntry)
                }
            }

            if (state is LoginState.Success) {
                LaunchedEffect(Unit) {
                    navController.navigate(Route.Home) {
                        popUpTo(Route.Login) { inclusive = true }
                    }
                }
            }
        }
    }
}

// ── OtpScreen ────────────────────────────────────────────────────────────────

/**
 * OTP entry screen — shared for both Firebase and MSG91 flows.
 *
 * Firebase path: SMS delivered by Firebase; user types 6-digit code → verifyFirebaseOtp().
 * MSG91 path:    SMS delivered by MSG91; user types 6-digit code → verifyMsg91Otp().
 *
 * City selection:
 *   CityPickerBottomSheet shown inline. Defaults to "Mumbai" if dismissed.
 *
 * Resend OTP:
 *   60-second countdown. Button disabled until countdown ends.
 *   On 429 from server: "Too many attempts — try again in 1 hour".
 */
@Composable
fun OtpScreen(
    navController: NavController,
    viewModel: LoginViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val selectedCity by viewModel.selectedCityId.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()

    var otpCode by remember { mutableStateOf("") }
    var showCityPicker by remember { mutableStateOf(false) }

    // Resend countdown
    var resendCountdown by remember { mutableIntStateOf(60) }
    LaunchedEffect(Unit) {
        while (resendCountdown > 0) {
            delay(1_000L)
            resendCountdown--
        }
    }

    LaunchedEffect(state) {
        when (val s = state) {
            is LoginState.Error -> scope.launch { snackbarHostState.showSnackbar(s.message) }
            is LoginState.Success -> navController.navigate(Route.Home) {
                popUpTo(Route.Login) { inclusive = true }
            }
            is LoginState.PhoneEntry -> navController.popBackStack()  // integrity_token_expired
            else -> Unit
        }
    }

    val context = LocalContext.current as Activity
    val provider = (state as? LoginState.OtpEntry)?.provider ?: OtpProvider.Firebase

    Scaffold(snackbarHost = { SnackbarHost(snackbarHostState) }) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 24.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("Enter OTP", style = MaterialTheme.typography.headlineSmall)
            Spacer(Modifier.height(8.dp))
            Text(
                text = if (provider == OtpProvider.Firebase)
                    "OTP sent via Firebase SMS"
                else
                    "OTP sent via SMS",
                style = MaterialTheme.typography.bodyMedium,
            )

            Spacer(Modifier.height(24.dp))

            // OTP input
            OutlinedTextField(
                value = otpCode,
                onValueChange = { if (it.length <= 6 && it.all(Char::isDigit)) otpCode = it },
                label = { Text("6-digit OTP") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(Modifier.height(16.dp))

            // City picker row
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text("City: ${ApiConstants.CITY_LIST.find { it.id == selectedCity }?.displayName ?: selectedCity}")
                TextButton(onClick = { showCityPicker = true }) { Text("Change") }
            }

            Spacer(Modifier.height(24.dp))

            val isVerifying = state is LoginState.Verifying
            Button(
                onClick = {
                    if (provider == OtpProvider.Firebase) viewModel.verifyFirebaseOtp(otpCode)
                    else viewModel.verifyMsg91Otp(otpCode)
                },
                enabled = otpCode.length == 6 && !isVerifying,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (isVerifying) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                else Text("Verify")
            }

            Spacer(Modifier.height(16.dp))

            TextButton(
                onClick = { viewModel.resendOtp(context); resendCountdown = 60 },
                enabled = resendCountdown == 0 && !isVerifying,
            ) {
                if (resendCountdown > 0) Text("Resend OTP in ${resendCountdown}s")
                else Text("Resend OTP")
            }
        }
    }

    // City picker bottom sheet
    if (showCityPicker) {
        CityPickerBottomSheet(
            currentCityId = selectedCity,
            onCitySelected = { cityId ->
                viewModel.onCitySelected(cityId)
                showCityPicker = false
            },
            onDismiss = { showCityPicker = false },
        )
    }
}

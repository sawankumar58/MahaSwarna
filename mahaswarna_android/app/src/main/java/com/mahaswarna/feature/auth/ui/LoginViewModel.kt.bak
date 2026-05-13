package com.mahaswarna.feature.auth.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.android.play.core.integrity.IntegrityManagerFactory
import com.google.android.play.core.integrity.IntegrityTokenRequest
import com.google.firebase.FirebaseException
import com.google.firebase.FirebaseNetworkException
import com.google.firebase.auth.FirebaseAuth
import com.google.firebase.auth.FirebaseTooManyRequestsException
import com.google.firebase.auth.PhoneAuthCredential
import com.google.firebase.auth.PhoneAuthOptions
import com.google.firebase.auth.PhoneAuthProvider
import com.mahaswarna.core.network.ApiConstants
import com.mahaswarna.feature.auth.data.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.tasks.await
import java.util.concurrent.TimeUnit
import javax.inject.Inject

/** OTP provider received from the backend. */
enum class OtpProvider { Firebase, Msg91 }

/** Full OTP login state machine. */
sealed class LoginState {
    data object Idle         : LoginState()
    data object PhoneEntry   : LoginState()
    data object SendingOtp   : LoginState()
    data class  OtpEntry(val provider: OtpProvider) : LoginState()
    data object Verifying    : LoginState()
    data object Success      : LoginState()
    data class  Error(val message: String) : LoginState()
}

/**
 * OTP login ViewModel.
 *
 * Flow (Firebase path):
 *   1. User enters phone → sendOtp(phone)
 *   2. State → SendingOtp
 *   3. Play Integrity token obtained, POST /auth/send-otp called
 *   4. If response.provider == "firebase":
 *      startFirebaseVerification() → Firebase SMS triggers
 *      State → OtpEntry(Firebase)
 *   5. Auto-verify: onVerificationCompleted() fires immediately → loginWithFirebase()
 *      Manual: user enters code → verifyFirebaseOtp()
 *
 * Flow (MSG91 path — fallback for network failures):
 *   1. sendOtp() → response.provider == "msg91" or FirebaseNetworkException triggers switchToMsg91()
 *   2. State → OtpEntry(Msg91)
 *   3. User enters code → verifyMsg91Otp()
 *
 * FirebaseTooManyRequestsException → Error (never switch to MSG91 — would bypass rate limit).
 *
 * City selection:
 *   CityPickerBottomSheet shown on OtpEntry before user taps Verify.
 *   Exposed via selectedCityId; defaults to "mumbai" if user dismisses.
 *   cityID is forwarded to every login() call.
 */
@HiltViewModel
class LoginViewModel @Inject constructor(
    private val authRepository: AuthRepository,
) : ViewModel() {

    private val _state = MutableStateFlow<LoginState>(LoginState.PhoneEntry)
    val state: StateFlow<LoginState> = _state.asStateFlow()

    // City picker state — always has a value (default: mumbai)
    private val _selectedCityId = MutableStateFlow(ApiConstants.DEFAULT_CITY.id)
    val selectedCityId: StateFlow<String> = _selectedCityId.asStateFlow()

    // Internal OTP flow state
    private var pendingIntegrityToken: String? = null
    private var verificationId: String? = null
    private var phoneNumber: String = ""

    // Firebase auth instance — injected via constructor in production; used directly here for brevity
    private val firebaseAuth: FirebaseAuth get() = FirebaseAuth.getInstance()

    // ── Public API ────────────────────────────────────────────────────────────

    fun onCitySelected(cityId: String) {
        _selectedCityId.value = cityId
    }

    /**
     * Entry point: user taps "Send OTP".
     * 1. Obtains Play Integrity token.
     * 2. POST /auth/send-otp.
     * 3. Routes to Firebase or MSG91 path based on response.
     */
    fun sendOtp(phone: String, activity: android.app.Activity) {
        phoneNumber = phone
        _state.value = LoginState.SendingOtp
        viewModelScope.launch {
            // Step 1 — Play Integrity token
            val integrityToken = runCatching {
                obtainIntegrityToken(activity)
            }.getOrElse { e ->
                _state.value = LoginState.Error("Integrity check failed: ${e.message}")
                return@launch
            }
            pendingIntegrityToken = integrityToken

            // Step 2 — POST /auth/send-otp
            val providerResponse = runCatching {
                authRepository.sendOtp(phone)
            }.getOrElse { e ->
                _state.value = LoginState.Error("Could not send OTP: ${e.message}")
                return@launch
            }

            // Step 3 — route
            when (providerResponse.provider) {
                "firebase" -> startFirebaseVerification(phone, activity)
                "msg91"    -> _state.value = LoginState.OtpEntry(OtpProvider.Msg91)
                else       -> _state.value = LoginState.OtpEntry(OtpProvider.Msg91)
            }
        }
    }

    /**
     * MSG91 path: user enters 6-digit code and taps Verify.
     */
    fun verifyMsg91Otp(otp: String) {
        _state.value = LoginState.Verifying
        viewModelScope.launch {
            runCatching {
                authRepository.loginMsg91(
                    phone          = phoneNumber,
                    otp            = otp,
                    integrityToken = pendingIntegrityToken ?: "",
                    cityID         = _selectedCityId.value,
                )
            }.onSuccess {
                _state.value = LoginState.Success
            }.onFailure { e ->
                val msg = when {
                    e.isHttpCode(403) -> {
                        _state.value = LoginState.PhoneEntry   // restart for integrity_token_expired
                        "Session expired — please try again"
                    }
                    else -> e.message ?: "Verification failed"
                }
                _state.value = LoginState.Error(msg)
            }
        }
    }

    /**
     * Firebase path: user manually enters 6-digit SMS code.
     */
    fun verifyFirebaseOtp(code: String) {
        val vId = verificationId ?: run {
            _state.value = LoginState.Error("Verification session expired — please resend OTP")
            return
        }
        _state.value = LoginState.Verifying
        viewModelScope.launch {
            runCatching {
                val credential = PhoneAuthProvider.getCredential(vId, code)
                signInWithFirebaseCredential(credential)
            }.onFailure { e ->
                _state.value = LoginState.Error(e.message ?: "Verification failed")
            }
        }
    }

    /** Resend OTP — restarts the whole flow from sendOtp. */
    fun resendOtp(activity: android.app.Activity) {
        sendOtp(phoneNumber, activity)
    }

    /**
     * Triggered by Firebase SDK when it auto-verifies the number (instant-verify on same device).
     * Called from PhoneAuthProvider callbacks — no UI interaction required.
     */
    internal fun onFirebaseAutoVerified(credential: PhoneAuthCredential) {
        _state.value = LoginState.Verifying
        viewModelScope.launch {
            runCatching { signInWithFirebaseCredential(credential) }
                .onFailure { e -> _state.value = LoginState.Error(e.message ?: "Auto-verify failed") }
        }
    }

    // ── Private helpers ───────────────────────────────────────────────────────

    private fun startFirebaseVerification(phone: String, activity: android.app.Activity) {
        val callbacks = object : PhoneAuthProvider.OnVerificationStateChangedCallbacks() {
            override fun onVerificationCompleted(credential: PhoneAuthCredential) {
                // Auto-verification (instant verify or Google Play services resolution)
                onFirebaseAutoVerified(credential)
            }

            override fun onVerificationFailed(e: FirebaseException) {
                _state.value = when (e) {
                    is FirebaseTooManyRequestsException ->
                        // MUST NOT switch to MSG91 — rate limit is an abuse-prevention signal
                        LoginState.Error("Too many attempts — please wait before retrying")
                    is FirebaseNetworkException ->
                        // Network failure only → switch to MSG91
                        LoginState.OtpEntry(OtpProvider.Msg91).also { switchToMsg91(activity) }
                    else ->
                        LoginState.Error(e.message ?: "Verification failed")
                }
            }

            override fun onCodeSent(vId: String, token: PhoneAuthProvider.ForceResendingToken) {
                verificationId = vId
                _state.value = LoginState.OtpEntry(OtpProvider.Firebase)
            }
        }

        val e164 = if (phone.startsWith("+")) phone else "+91$phone"
        val options = PhoneAuthOptions.newBuilder(firebaseAuth)
            .setPhoneNumber(e164)
            .setTimeout(60L, TimeUnit.SECONDS)
            .setActivity(activity)
            .setCallbacks(callbacks)
            .build()
        PhoneAuthProvider.verifyPhoneNumber(options)
    }

    private fun switchToMsg91(activity: android.app.Activity) {
        viewModelScope.launch {
            runCatching { authRepository.sendOtp(phoneNumber) }
            _state.value = LoginState.OtpEntry(OtpProvider.Msg91)
        }
    }

    private suspend fun signInWithFirebaseCredential(credential: PhoneAuthCredential) {
        val result = firebaseAuth.signInWithCredential(credential).await()
        val idToken = result.user!!.getIdToken(false).await().token!!
        authRepository.loginFirebase(
            phone            = phoneNumber,
            firebaseIdToken  = idToken,
            integrityToken   = pendingIntegrityToken ?: "",
            cityID           = _selectedCityId.value,
        )
        _state.value = LoginState.Success
    }

    private suspend fun obtainIntegrityToken(activity: android.app.Activity): String {
        val manager = IntegrityManagerFactory.create(activity)
        val tokenResponse = manager.requestIntegrityToken(
            IntegrityTokenRequest.builder()
                .setNonce("mahaswarna-${System.currentTimeMillis()}")
                .build()
        ).await()
        return tokenResponse.token()
    }

    private fun Throwable.isHttpCode(code: Int): Boolean =
        this is retrofit2.HttpException && this.code() == code
}

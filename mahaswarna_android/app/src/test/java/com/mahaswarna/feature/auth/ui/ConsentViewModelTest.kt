package com.mahaswarna.feature.auth.ui

import app.cash.turbine.test
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.feature.auth.data.AuthRepository
import com.mahaswarna.feature.auth.data.ConsentType
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Before
import org.junit.Test

/**
 * Unit test for ConsentViewModel.
 *
 * Required by architecture spec:
 *   "UNIT TEST REQUIRED: ConsentViewModelTest must assert that exactly 2 calls are made
 *    on 'I Agree' — one per consent type — and that 'ai_disclaimer' is never passed
 *    to logConsent()."
 */
@OptIn(ExperimentalCoroutinesApi::class)
class ConsentViewModelTest {

    private val testDispatcher = StandardTestDispatcher()
    private lateinit var authRepository: AuthRepository
    private lateinit var preferenceStore: PreferenceStore
    private lateinit var viewModel: ConsentViewModel

    @Before
    fun setup() {
        Dispatchers.setMain(testDispatcher)
        authRepository  = mockk(relaxed = true)
        preferenceStore = mockk(relaxed = true)
        viewModel = ConsentViewModel(authRepository, preferenceStore)
    }

    @After
    fun teardown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `acceptConsent makes exactly 2 logConsent calls`() = runTest {
        coEvery { authRepository.logConsent(any(), any()) } returns Unit

        viewModel.acceptConsent()
        testDispatcher.scheduler.advanceUntilIdle()

        coVerify(exactly = 2) { authRepository.logConsent(any(), any()) }
    }

    @Test
    fun `acceptConsent calls privacy_policy first then tos`() = runTest {
        val callOrder = mutableListOf<ConsentType>()
        coEvery { authRepository.logConsent(any(), any()) } coAnswers {
            callOrder.add(firstArg())
        }

        viewModel.acceptConsent()
        testDispatcher.scheduler.advanceUntilIdle()

        assert(callOrder == listOf(ConsentType.PRIVACY_POLICY, ConsentType.TOS)) {
            "Expected [PRIVACY_POLICY, TOS] but got $callOrder"
        }
    }

    @Test
    fun `acceptConsent never sends ai_disclaimer`() = runTest {
        val calledTypes = mutableListOf<ConsentType>()
        coEvery { authRepository.logConsent(any(), any()) } coAnswers {
            calledTypes.add(firstArg())
        }

        viewModel.acceptConsent()
        testDispatcher.scheduler.advanceUntilIdle()

        // ai_disclaimer is not a valid ConsentType — this assertion uses the enum's
        // compile-time completeness: if someone adds AI_DISCLAIMER to ConsentType,
        // the test below would catch any logConsent call using it.
        val hasAiDisclaimerWireValue = calledTypes.any { it.wireValue == "ai_disclaimer" }
        assertFalse("ai_disclaimer must never be sent to logConsent()", hasAiDisclaimerWireValue)
    }

    @Test
    fun `acceptConsent writes consent to PreferenceStore only after both calls succeed`() = runTest {
        coEvery { authRepository.logConsent(any(), any()) } returns Unit

        viewModel.acceptConsent()
        testDispatcher.scheduler.advanceUntilIdle()

        // Preference must be set AFTER both calls succeed
        coVerify(ordering = io.mockk.Ordering.SEQUENCE) {
            authRepository.logConsent(ConsentType.PRIVACY_POLICY, any())
            authRepository.logConsent(ConsentType.TOS, any())
            preferenceStore.setConsentAccepted(true)
        }
    }

    @Test
    fun `acceptConsent emits Done state on success`() = runTest {
        coEvery { authRepository.logConsent(any(), any()) } returns Unit

        viewModel.uiState.test {
            skipItems(1)   // initial Idle
            viewModel.acceptConsent()
            assert(awaitItem() is ConsentUiState.Submitting)
            assert(awaitItem() is ConsentUiState.Done)
        }
    }

    @Test
    fun `acceptConsent emits Error state when first call fails`() = runTest {
        coEvery { authRepository.logConsent(any(), any()) } throws RuntimeException("Network error")

        viewModel.uiState.test {
            skipItems(1)
            viewModel.acceptConsent()
            assert(awaitItem() is ConsentUiState.Submitting)
            val error = awaitItem()
            assert(error is ConsentUiState.Error) { "Expected Error but got $error" }
        }

        // Preference store must NOT be written on failure
        coVerify(exactly = 0) { preferenceStore.setConsentAccepted(any()) }
    }

    @Test
    fun `acceptConsent does not write preference when second call fails`() = runTest {
        var callCount = 0
        coEvery { authRepository.logConsent(any(), any()) } coAnswers {
            callCount++
            if (callCount == 2) throw RuntimeException("TOS call failed")
        }

        viewModel.acceptConsent()
        testDispatcher.scheduler.advanceUntilIdle()

        coVerify(exactly = 0) { preferenceStore.setConsentAccepted(any()) }
    }
}

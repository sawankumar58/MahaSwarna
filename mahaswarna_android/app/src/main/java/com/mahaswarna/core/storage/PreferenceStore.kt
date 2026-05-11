package com.mahaswarna.core.storage

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.runBlocking
import javax.inject.Inject
import javax.inject.Singleton

private val Context.dataStore by preferencesDataStore(name = "mahaswarna_prefs")

@Singleton
class PreferenceStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    // ── Keys ─────────────────────────────────────────────────────────────────
    private val AI_QUOTA_USED      = intPreferencesKey("ai_quota_used")
    private val AI_QUOTA_LIMIT     = intPreferencesKey("ai_quota_limit")
    private val AI_QUOTA_RESET_AT  = longPreferencesKey("ai_quota_reset_at")
    private val PENDING_BILL_QUEUE = stringPreferencesKey("pending_bill_queue")
    private val PENDING_FCM_TOKEN  = stringPreferencesKey("pending_fcm_token")
    private val CONSENT_ACCEPTED   = booleanPreferencesKey("consent_accepted")

    // ── AI Quota ─────────────────────────────────────────────────────────────

    data class AiQuotaState(
        val used: Int,
        val limit: Int,
        val resetAt: Long,
    ) {
        val isExhausted: Boolean get() = limit > 0 && used >= limit
    }

    /** Written by AiQuotaInterceptor on every response that carries quota headers. */
    fun setAiQuota(used: Int, limit: Int, resetAt: Long) {
        runBlocking {
            context.dataStore.edit { prefs ->
                prefs[AI_QUOTA_USED]     = used
                prefs[AI_QUOTA_LIMIT]    = limit
                prefs[AI_QUOTA_RESET_AT] = resetAt
            }
        }
    }

    fun getAiQuotaFlow(): Flow<AiQuotaState> = context.dataStore.data.map { prefs ->
        AiQuotaState(
            used    = prefs[AI_QUOTA_USED]     ?: 0,
            limit   = prefs[AI_QUOTA_LIMIT]    ?: 0,
            resetAt = prefs[AI_QUOTA_RESET_AT] ?: 0L,
        )
    }

    // ── Pending Bill Queue ────────────────────────────────────────────────────

    fun setPendingBillQueue(json: String) = runBlocking {
        context.dataStore.edit { it[PENDING_BILL_QUEUE] = json }
    }

    fun getPendingBillQueue(): String? = runBlocking {
        context.dataStore.data.first()[PENDING_BILL_QUEUE]
    }

    fun clearPendingBillQueue() = runBlocking {
        context.dataStore.edit { it.remove(PENDING_BILL_QUEUE) }
    }

    // ── Pending FCM Token (pre-login) ─────────────────────────────────────────
    // The token is NOT sensitive — FCM registration tokens are not secret —
    // so DataStore (not EncryptedSharedPreferences) is correct here.

    fun setPendingFcmToken(token: String) = runBlocking {
        context.dataStore.edit { it[PENDING_FCM_TOKEN] = token }
    }

    fun getPendingFcmToken(): String? = runBlocking {
        context.dataStore.data.first()[PENDING_FCM_TOKEN]
    }

    fun clearPendingFcmToken() = runBlocking {
        context.dataStore.edit { it.remove(PENDING_FCM_TOKEN) }
    }

    // ── Consent ───────────────────────────────────────────────────────────────

    fun setConsentAccepted(value: Boolean) = runBlocking {
        context.dataStore.edit { it[CONSENT_ACCEPTED] = value }
    }

    fun getConsentAccepted(): Flow<Boolean> = context.dataStore.data.map { prefs ->
        prefs[CONSENT_ACCEPTED] ?: false
    }
}

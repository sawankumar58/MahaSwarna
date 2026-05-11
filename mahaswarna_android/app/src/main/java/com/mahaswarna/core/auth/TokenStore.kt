package com.mahaswarna.core.auth

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

/**
 * AES-256 EncryptedSharedPreferences wrapper for the JWT access/refresh tokens.
 *
 * WRITE ORDER INVARIANT — saveAccessToken():
 *   Step 1: prefs.edit().putString("access_token", token).commit()  ← commit(), NOT apply()
 *   Step 2: File(filesDir, "token_exists_marker").createNewFile()
 *
 * apply() is async — the marker can be written before the token is flushed.
 * If the process is killed between writes, next cold start routes to Home
 * with no token → 401 → force-logout. Reversed order has the same consequence.
 *
 * clearAll(): deletes token_exists_marker + all ESP keys.
 *
 * TokenStore.init() / Keystore access is NOT called from Application.onCreate().
 * First access happens lazily in AuthInterceptor on the first background REST
 * call — the 50–200ms TEE overhead on budget devices is absorbed there.
 */
@Singleton
class TokenStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    companion object {
        private const val PREFS_FILE = "mahaswarna_secure_prefs"
        private const val KEY_ACCESS_TOKEN  = "access_token"
        private const val KEY_REFRESH_TOKEN = "refresh_token"
        private const val MARKER_FILE = "token_exists_marker"
    }

    private val prefs by lazy {
        val masterKey = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        EncryptedSharedPreferences.create(
            context,
            PREFS_FILE,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    fun getAccessToken(): String? = prefs.getString(KEY_ACCESS_TOKEN, null)

    fun getRefreshToken(): String? = prefs.getString(KEY_REFRESH_TOKEN, null)

    fun hasToken(): Boolean =
        File(context.filesDir, MARKER_FILE).exists()

    /**
     * Saves access token.
     * INVARIANT: commit() (synchronous flush) MUST come before createNewFile().
     */
    fun saveAccessToken(token: String) {
        prefs.edit().putString(KEY_ACCESS_TOKEN, token).commit()          // Step 1
        File(context.filesDir, MARKER_FILE).createNewFile()               // Step 2
    }

    fun saveTokens(accessToken: String, refreshToken: String) {
        prefs.edit()
            .putString(KEY_ACCESS_TOKEN, accessToken)
            .putString(KEY_REFRESH_TOKEN, refreshToken)
            .commit()                                                      // Step 1
        File(context.filesDir, MARKER_FILE).createNewFile()               // Step 2
    }

    /**
     * Removes all stored tokens and the existence marker.
     * Called from SessionManager.emitLoggedOut() / DeleteAccountUseCase.
     */
    fun clearAll() {
        prefs.edit().clear().commit()
        File(context.filesDir, MARKER_FILE).delete()
    }
}

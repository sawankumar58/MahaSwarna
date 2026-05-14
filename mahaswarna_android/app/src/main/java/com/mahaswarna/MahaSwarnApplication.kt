package com.mahaswarna

import android.app.Application
import com.mahaswarna.core.util.NotificationChannelSetup
import dagger.hilt.android.HiltAndroidApp

/**
 * Application class.
 *
 * Init order (INVARIANT — do not reorder):
 *   Step 1 — NOTIFICATION CHANNELS (MUST be first, before super.onCreate() / Firebase init).
 *             If Firebase initialises first and an FCM message arrives immediately
 *             (edge case on API 26+), the notification is silently dropped because
 *             the channel does not exist yet. Channel creation is idempotent.
 *   Step 2 — super.onCreate() → Hilt builds the app component:
 *               NetworkModule  → OkHttpClient with interceptors in required order
 *               DatabaseModule → Room (async open, FK enforcement enabled)
 *               WsModule       → WsClient singleton (not connected yet)
 *   Step 3 — Firebase auto-init (google-services plugin handles this automatically)
 *   Step 4 — Sentry init (after super so Hilt graph + app context are fully ready)
 *
 * Sentry DSN: references BuildConfig.SENTRY_DSN, declared via buildConfigField
 * in build.gradle.kts for each build type:
 *   debug   → "" (Sentry no-ops; avoids polluting prod project with dev noise)
 *   staging → overridden in CI from SENTRY_DSN_STAGING secret
 *   release → overridden in CI from SENTRY_DSN_RELEASE secret
 * Sentry gracefully no-ops when DSN is blank.
 *
 * TokenStore is NOT accessed here. On post-reboot cold start, first Keystore TEE
 * access takes 50–200ms on budget devices. Calling it in onCreate() consumes the
 * 400ms budget margin before Room even opens. Token is accessed lazily by
 * AuthInterceptor on the first background REST call.
 *
 * WS lifecycle is started from MainActivity, not here.
 */
@HiltAndroidApp
class MahaSwarnApplication : Application() {

    override fun onCreate() {
        // Step 1 — channels before super (and therefore before Firebase)
        NotificationChannelSetup.createChannels(this)

        // Step 2 — Hilt + Firebase auto-init
        super.onCreate()

        // Step 3/4 — Sentry (after super so app context is ready)
        // BuildConfig.SENTRY_DSN is declared per build type in build.gradle.kts.
        // Sentry silently skips init when DSN is blank — no crash, no noise.
        io.sentry.android.core.SentryAndroid.init(this) { options ->
            options.dsn = BuildConfig.SENTRY_DSN
            options.isEnableAutoSessionTracking = true
            options.environment = BuildConfig.BUILD_TYPE        // "debug" / "staging" / "release"
            options.sampleRate = if (BuildConfig.DEBUG) 1.0 else 0.2
            options.tracesSampleRate = if (BuildConfig.DEBUG) 1.0 else 0.05
        }
    }
}

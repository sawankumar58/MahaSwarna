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
 *               DatabaseModule → Room (async open)
 *               WsModule       → WsClient singleton (not connected yet)
 *   Step 3 — Firebase auto-init (google-services plugin handles this automatically)
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

        // Step 3 — Sentry (after super so app context is ready)
        io.sentry.android.core.SentryAndroid.init(this) { options ->
            options.dsn = BuildConfig.SENTRY_DSN_PLACEHOLDER
            options.isEnableAutoSessionTracking = true
        }
    }
}

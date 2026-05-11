package com.mahaswarna.notification

import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import androidx.core.app.NotificationCompat
import com.google.firebase.crashlytics.FirebaseCrashlytics
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import com.mahaswarna.MainActivity
import com.mahaswarna.R
import com.mahaswarna.core.auth.TokenStore
import com.mahaswarna.core.storage.PreferenceStore
import com.mahaswarna.core.util.NotificationChannelSetup.CHANNEL_ID_PRICE_ALERTS
import com.mahaswarna.feature.alerts.data.AlertsRepository
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.launch
import javax.inject.Inject

@AndroidEntryPoint
class MahaSwarnMessagingService : FirebaseMessagingService() {

    @Inject lateinit var tokenStore: TokenStore
    @Inject lateinit var preferenceStore: PreferenceStore
    // AlertsRepository injected lazily to avoid circular dep at startup
    @Inject lateinit var alertsRepository: AlertsRepository

    /**
     * Called when FCM assigns a new registration token.
     * If user is authenticated → register immediately.
     * If not authenticated → defer to AuthRepository.login().
     */
    override fun onNewToken(token: String) {
        super.onNewToken(token)
        if (tokenStore.hasToken()) {
            // User is authenticated — register device token immediately
            // (fire-and-forget; failure is acceptable — FCM will retry)
            MainScope().launch {
                runCatching { alertsRepository.registerDeviceToken(token) }
                    .onFailure {
                        FirebaseCrashlytics.getInstance()
                            .log("FCM token register failed: ${it.message}")
                    }
            }
        } else {
            // Not authenticated — defer until next successful login
            preferenceStore.setPendingFcmToken(token)
        }
    }

    /**
     * FCM data payload contract (set by deliver_alert_usecase.go):
     *   type, metal, direction, threshold, city_id, screen — ALL required.
     * Missing fields → non-fatal Crashlytics log (backend contract regression).
     */
    override fun onMessageReceived(message: RemoteMessage) {
        super.onMessageReceived(message)
        val data      = message.data
        val type      = data["type"]
        val screen    = data["screen"]
        val metal     = data["metal"]
        val direction = data["direction"]
        val cityId    = data["city_id"]
        val threshold = data["threshold"]

        if (type == null || metal == null || direction == null ||
            threshold == null || cityId == null || screen == null
        ) {
            FirebaseCrashlytics.getInstance()
                .log("FCM payload missing required field(s): $data")
        }

        val notifId = System.currentTimeMillis().toInt()
        val intent = Intent(this, MainActivity::class.java).apply {
            putExtra("deep_link_screen", screen)
            putExtra("deep_link_metal", metal)
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this, notifId, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val directionLabel = if (direction == "above") "above" else "below"
        val notification = NotificationCompat.Builder(this, CHANNEL_ID_PRICE_ALERTS)
            .setContentTitle("${metal?.replaceFirstChar { it.uppercase() }} Rate Alert")
            .setContentText(
                "Rate crossed ₹$threshold ($directionLabel) in ${cityId?.replaceFirstChar { it.uppercase() }}"
            )
            .setSmallIcon(R.drawable.ic_notification)
            .setContentIntent(pendingIntent)
            .setAutoCancel(true)
            .build()

        val nm = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        nm.notify(notifId, notification)
    }
}

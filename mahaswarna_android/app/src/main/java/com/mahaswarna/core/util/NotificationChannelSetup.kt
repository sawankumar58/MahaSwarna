package com.mahaswarna.core.util

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build

/**
 * Creates notification channels on app start.
 * REQUIRED — Android 8+ (API 26+) silently drops all notifications
 * if the channel does not exist.
 *
 * Called from MahaSwarnApplication.onCreate() BEFORE super.onCreate()
 * (and therefore before Firebase auto-init). Channel creation is idempotent
 * — calling it on every start is safe.
 */
object NotificationChannelSetup {

    fun createChannels(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return

        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

        val priceAlertsChannel = NotificationChannel(
            CHANNEL_ID_PRICE_ALERTS,
            "Price Alerts",
            NotificationManager.IMPORTANCE_HIGH,
        ).apply {
            description = "Gold and silver rate threshold alerts"
            enableVibration(true)
        }

        nm.createNotificationChannel(priceAlertsChannel)
    }

    const val CHANNEL_ID_PRICE_ALERTS = "price_alerts"
}

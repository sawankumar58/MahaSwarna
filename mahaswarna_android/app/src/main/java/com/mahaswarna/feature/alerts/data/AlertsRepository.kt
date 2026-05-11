package com.mahaswarna.feature.alerts.data

import javax.inject.Inject
import javax.inject.Singleton

/**
 * Stub repository for FCM device-token registration.
 * Full implementation (price-alert CRUD) lives in Phase 2 (feature/alerts/).
 *
 * Only [registerDeviceToken] is needed by [MahaSwarnMessagingService] in Phase 1.
 */
@Singleton
class AlertsRepository @Inject constructor(
    // AlertsApi injected in Phase 2; placeholder uses lazy to avoid missing binding
    private val alertsApi: dagger.Lazy<AlertsApi>,
) {
    /**
     * Registers (or refreshes) the FCM registration token on the backend.
     * Called from [MahaSwarnMessagingService.onNewToken] when the user is authenticated.
     */
    suspend fun registerDeviceToken(fcmToken: String) {
        alertsApi.get().registerDeviceToken(RegisterTokenRequest(fcmToken))
    }
}

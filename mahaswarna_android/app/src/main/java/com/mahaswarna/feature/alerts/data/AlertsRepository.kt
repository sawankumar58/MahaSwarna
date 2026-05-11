package com.mahaswarna.feature.alerts.data

import com.mahaswarna.data.mapper.toDomain
import com.mahaswarna.feature.alerts.domain.Alert
import com.mahaswarna.local.dao.AlertDao
import com.mahaswarna.local.entity.AlertEntity
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

/**
 * AlertsRepository — manages price alert CRUD.
 *
 * Local-first: Room is the source of truth for list display.
 * Network calls update Room on success; errors propagate to ViewModel.
 *
 * FCM device-token registration is called from MahaSwarnMessagingService.onNewToken
 * when the user is authenticated.
 */
@Singleton
class AlertsRepository @Inject constructor(
    private val alertsApi: dagger.Lazy<AlertsApi>,
    private val alertDao: AlertDao,
) {

    /**
     * Reactive stream of all alerts, ordered by createdAt DESC.
     * Emits immediately from Room cache; refreshed after [syncAlerts].
     */
    fun observeAlerts(): Flow<List<Alert>> =
        alertDao.observeAll().map { entities -> entities.map { it.toDomain() } }

    /**
     * Fetches alerts from the backend and persists them to Room.
     * Called on screen entry and after create/delete to keep Room in sync.
     */
    suspend fun syncAlerts() {
        val response = alertsApi.get().listAlerts()
        val entities = response.alerts.map { dto ->
            AlertEntity(
                id        = dto.id,
                cityId    = dto.cityId,
                metal     = dto.metal,
                direction = dto.direction,
                threshold = dto.threshold.toFloat(),
                active    = true,
                createdAt = dto.createdAt,
            )
        }
        alertDao.upsertAll(entities)
    }

    /**
     * Creates a new price alert on the backend and persists it to Room immediately.
     * Fires analytics event `alert_created { metal, direction }` — call site is ViewModel.
     *
     * @throws retrofit2.HttpException on HTTP 4xx/5xx — ViewModel maps to UI error.
     */
    suspend fun createAlert(
        cityId: String,
        metal: String,
        threshold: Double,
        direction: String,
    ): Alert {
        val dto = alertsApi.get().createAlert(
            CreateAlertRequest(
                cityId    = cityId,
                metal     = metal,
                threshold = threshold,
                direction = direction,
            )
        )
        val entity = AlertEntity(
            id        = dto.id,
            cityId    = dto.cityId,
            metal     = dto.metal,
            direction = dto.direction,
            threshold = dto.threshold.toFloat(),
            active    = true,
            createdAt = dto.createdAt,
        )
        alertDao.upsert(entity)
        return entity.toDomain()
    }

    /**
     * Deletes an alert on the backend and removes it from Room immediately.
     *
     * @throws retrofit2.HttpException on HTTP 404 (already deleted) or 4xx/5xx.
     */
    suspend fun deleteAlert(alertId: String) {
        alertsApi.get().deleteAlert(alertId)
        alertDao.delete(alertId)
    }

    /**
     * Registers (or refreshes) the FCM registration token on the backend.
     * Called from MahaSwarnMessagingService.onNewToken when the user is authenticated.
     */
    suspend fun registerDeviceToken(fcmToken: String) {
        alertsApi.get().registerDeviceToken(RegisterTokenRequest(fcmToken))
    }
}

package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	ce "github.com/mahaswarna/contracts/events"
)

type DeliverAlertUseCase struct {
	alerts   *infrastructure.AlertsRepository
	tokens   *infrastructure.DeviceTokenRepository
	fcm      *infrastructure.PushNotificationClient
	audit    *infrastructure.AuditLogRepository
	notifier *events.Notifier
}

func NewDeliverAlertUseCase(alerts *infrastructure.AlertsRepository, tokens *infrastructure.DeviceTokenRepository,
	fcm *infrastructure.PushNotificationClient, audit *infrastructure.AuditLogRepository, notifier *events.Notifier) *DeliverAlertUseCase {
	return &DeliverAlertUseCase{alerts: alerts, tokens: tokens, fcm: fcm, audit: audit, notifier: notifier}
}

func (uc *DeliverAlertUseCase) Deliver(ctx context.Context, alert domain.Alert, rate float64) error {
	deviceTokens, _ := uc.tokens.GetTokensForUser(ctx, alert.UserID)
	if len(deviceTokens) == 0 { return fmt.Errorf("no tokens for user %s", alert.UserID) }

	// FCM data payload — ALL 6 FIELDS REQUIRED (canonical source).
	data := map[string]string{
		"type":      "price_alert",
		"metal":     alert.Metal,
		"direction": alert.Direction,
		"threshold": fmt.Sprintf("%.2f", alert.Threshold),
		"city_id":   alert.CityID,
		"screen":    "rates",
	}
	if err := uc.fcm.SendToDevices(ctx, deviceTokens, data); err != nil {
		shared.Logger.Error("FCM send failed", "alertID", alert.ID, "err", err)
	}
	uc.alerts.MarkDelivered(ctx, alert.ID)
	uc.notifier.Notify(ctx, ce.ChannelAlertDelivered, ce.AlertDeliveredPayload{
		AlertID: alert.ID.String(), UserID: alert.UserID.String(), CityID: alert.CityID, Metal: alert.Metal, Rate: rate,
	})
	uc.audit.Append(ctx, shared.AuditEntry{Actor: "system", Action: "alert_delivered", Entity: "alerts", EntityID: alert.ID.String()})
	return nil
}

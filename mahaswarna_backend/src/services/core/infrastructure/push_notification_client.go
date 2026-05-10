package infrastructure

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type PushNotificationClient struct{ fcm *messaging.Client }

func NewPushNotificationClient(ctx context.Context) (*PushNotificationClient, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))))
	if err != nil { return nil, fmt.Errorf("firebase: %w", err) }
	fcm, err := app.Messaging(ctx)
	if err != nil { return nil, fmt.Errorf("fcm: %w", err) }
	return &PushNotificationClient{fcm: fcm}, nil
}

func (c *PushNotificationClient) SendToDevices(ctx context.Context, tokens []string, data map[string]string) error {
	if len(tokens) == 0 { return nil }
	msgs := make([]*messaging.Message, len(tokens))
	for i, t := range tokens {
		msgs[i] = &messaging.Message{Token: t, Data: data, Android: &messaging.AndroidConfig{Priority: "high"}}
	}
	_, err := c.fcm.SendEach(ctx, msgs)
	return err
}

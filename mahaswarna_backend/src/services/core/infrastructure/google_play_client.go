package infrastructure

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

type GooglePlayClient struct {
	svc  *androidpublisher.Service
	pkg  string
}

func NewGooglePlayClient(ctx context.Context) (*GooglePlayClient, error) {
	svc, err := androidpublisher.NewService(ctx, option.WithCredentialsJSON([]byte(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"))))
	if err != nil { return nil, fmt.Errorf("play client: %w", err) }
	return &GooglePlayClient{svc: svc, pkg: os.Getenv("GOOGLE_PLAY_PACKAGE_NAME")}, nil
}

func (c *GooglePlayClient) VerifySubscriptionPurchase(ctx context.Context, productID, purchaseToken string) (tier string, expiresAt *time.Time, raw any, err error) {
	sub, err := c.svc.Purchases.Subscriptions.Get(c.pkg, productID, purchaseToken).Context(ctx).Do()
	if err != nil { return "", nil, nil, fmt.Errorf("play api: %w", err) }
	if sub.PaymentState == nil || *sub.PaymentState != 1 { return "", nil, sub, fmt.Errorf("payment not received") }
	t := time.UnixMilli(sub.ExpiryTimeMillis)
	return "PREMIUM", &t, sub, nil
}

package infrastructure

import (
	"context"
	"fmt"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type FirebaseOtpProvider struct{ client *auth.Client }

func NewFirebaseOtpProvider(ctx context.Context) (*FirebaseOtpProvider, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))))
	if err != nil { return nil, fmt.Errorf("firebase app: %w", err) }
	client, err := app.Auth(ctx)
	if err != nil { return nil, fmt.Errorf("firebase auth: %w", err) }
	return &FirebaseOtpProvider{client: client}, nil
}

// SendOTP is a no-op — Firebase SMS is triggered client-side.
func (p *FirebaseOtpProvider) SendOTP(_ context.Context, _ string) error { return nil }

// VerifyOTP validates the Firebase ID token and checks the phone_number claim.
// Credential errors must NOT be swallowed — they must return immediately.
func (p *FirebaseOtpProvider) VerifyOTP(ctx context.Context, phone, idToken string) (bool, error) {
	token, err := p.client.VerifyIDToken(ctx, idToken)
	if err != nil { return false, fmt.Errorf("firebase verify: %w", err) }
	claim, ok := token.Claims["phone_number"].(string)
	if !ok { return false, fmt.Errorf("phone_number claim missing") }
	if normaliseE164(claim) != normaliseE164(phone) { return false, fmt.Errorf("phone mismatch") }
	return true, nil
}

func normaliseE164(phone string) string {
	phone = strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
	if strings.HasPrefix(phone, "+91") { return phone }
	if strings.HasPrefix(phone, "91") && len(phone) == 12 { return "+" + phone }
	if strings.HasPrefix(phone, "0") && len(phone) == 11 { return "+91" + phone[1:] }
	if len(phone) == 10 { return "+91" + phone }
	return phone
}

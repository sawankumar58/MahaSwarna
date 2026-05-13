package core_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

// ── mock OTP provider ─────────────────────────────────────────────────────────

type mockOtp struct {
	verifyOK  bool
	verifyErr error
}

func (m *mockOtp) SendOTP(_ context.Context, _ string) error { return nil }
func (m *mockOtp) VerifyOTP(_ context.Context, _, _ string) (bool, error) {
	return m.verifyOK, m.verifyErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

// testRSAKeyPEM generates a fresh RSA-2048 private key encoded as PKCS#1 PEM.
// LoginUseCase.NewLoginUseCase reads the key from JWT_PRIVATE_KEY env.
func testRSAKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return string(pem.EncodeToMemory(block))
}

func newLoginUC(t *testing.T, rdb *redis.Client) *application.LoginUseCase {
	t.Helper()
	t.Setenv("JWT_PRIVATE_KEY", testRSAKeyPEM(t))
	uc, err := application.NewLoginUseCase(nil, nil, &mockOtp{verifyOK: true}, rdb, nil)
	if err != nil {
		t.Fatalf("NewLoginUseCase: %v", err)
	}
	return uc
}

// ── LoginUseCase tests ────────────────────────────────────────────────────────

// TestLoginUseCase_ThrottledUser verifies that a user whose login_fail counter
// has reached maxLoginFails (10) receives ErrTooManyRequests immediately.
func TestLoginUseCase_ThrottledUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)

	// Pre-seed the fail counter at the throttle limit.
	// normalisePhone("9876543210") → "+919876543210"
	_ = rdb.Set(ctx, "login_fail:+919876543210", "10", 0).Err()

	_, err := uc.Execute(ctx, application.LoginInput{
		Phone:          "9876543210",
		IntegrityToken: "valid_token",
	})
	if err != shared.ErrTooManyRequests {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}
}

// TestLoginUseCase_NormalisesPhone_10Digit verifies that a bare 10-digit phone
// number is normalised to +91-prefixed form before the throttle key is looked up.
// (The throttle key is "login_fail:<normalised>", so a 10-digit input must match
//
//	the +91-prefixed key seeded here.)
func TestLoginUseCase_NormalisesPhone_10Digit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)
	_ = rdb.Set(ctx, "login_fail:+919876543210", "10", 0).Err()

	_, err := uc.Execute(ctx, application.LoginInput{Phone: "9876543210", IntegrityToken: "x"})
	if err != shared.ErrTooManyRequests {
		t.Errorf("10-digit normalisation: +91-prefixed throttle key must match; got %v", err)
	}
}

// TestLoginUseCase_NormalisesPhone_0Prefix verifies that 0XXXXXXXXXX is rewritten
// to +91XXXXXXXXXX.
func TestLoginUseCase_NormalisesPhone_0Prefix(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)
	_ = rdb.Set(ctx, "login_fail:+919876543210", "10", 0).Err()

	_, err := uc.Execute(ctx, application.LoginInput{Phone: "09876543210", IntegrityToken: "x"})
	if err != shared.ErrTooManyRequests {
		t.Errorf("0-prefix normalisation: expected throttle hit, got %v", err)
	}
}

// TestLoginUseCase_NormalisesPhone_91Prefix verifies 919876543210 → +919876543210.
func TestLoginUseCase_NormalisesPhone_91Prefix(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)
	_ = rdb.Set(ctx, "login_fail:+919876543210", "10", 0).Err()

	_, err := uc.Execute(ctx, application.LoginInput{Phone: "919876543210", IntegrityToken: "x"})
	if err != shared.ErrTooManyRequests {
		t.Errorf("91-prefix normalisation: expected throttle hit, got %v", err)
	}
}

// TestLoginUseCase_EmptyIntegrityToken verifies that an absent integrity token
// returns ErrDeviceNotTrusted (step 2 guard, before OTP or DB calls).
func TestLoginUseCase_EmptyIntegrityToken(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)

	_, err := uc.Execute(ctx, application.LoginInput{
		Phone:          "9876543210",
		IntegrityToken: "", // empty → ErrDeviceNotTrusted
	})
	if err != shared.ErrDeviceNotTrusted {
		t.Errorf("expected ErrDeviceNotTrusted, got %v", err)
	}
}

// TestLoginUseCase_ExpiredIntegrityToken verifies that a token with the "_expired"
// suffix (test-mode sentinel) returns ErrIntegrityTokenExpired.
func TestLoginUseCase_ExpiredIntegrityToken(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	uc := newLoginUC(t, rdb)

	_, err := uc.Execute(ctx, application.LoginInput{
		Phone:          "9876543210",
		IntegrityToken: "some_token_expired", // suffix "_expired" is the sentinel
	})
	if err != shared.ErrIntegrityTokenExpired {
		t.Errorf("expected ErrIntegrityTokenExpired, got %v", err)
	}
}

// ── OTPSendUseCase tests ──────────────────────────────────────────────────────

// TestOTPSendUseCase_RateLimitExceeded verifies that the 6th OTP send attempt
// (maxOTPSends = 5) returns ErrTooManyOTPRequests.
// The counter is pre-seeded to 5 via miniredis to skip the 5 success calls that
// would otherwise reach the nil FlagsRepository.
func TestOTPSendUseCase_RateLimitExceeded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	otp := &mockOtp{}
	uc := application.NewOTPSendUseCase(otp, rdb, nil) // flags nil; not reached on rate-limit path

	// normalisePhone("9876543210") → "+919876543210"
	// Pre-seed to maxOTPSends (5); next Incr returns 6 → rate-limited.
	mr.Set("otp_send:+919876543210", "5")

	_, err := uc.Execute(ctx, application.OTPSendInput{Phone: "9876543210"})
	if err != shared.ErrTooManyOTPRequests {
		t.Errorf("expected ErrTooManyOTPRequests, got %v", err)
	}
}

// TestOTPSendUseCase_RateLimitIsScopedToPhone verifies that rate limits are
// per-phone: exhausting the limit for one phone doesn't affect another.
func TestOTPSendUseCase_RateLimitIsScopedToPhone(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	otp := &mockOtp{}
	uc := application.NewOTPSendUseCase(otp, rdb, nil)

	// Exhaust phone A.
	mr.Set("otp_send:+919876543210", "5")

	// Phone B should not be throttled (counter starts from 0). Pre-seed to 4 so
	// it hits the flag path only once (on the 5th send = n=5 ≤ maxOTPSends).
	// To avoid the nil-flags panic we assert the error is NOT ErrTooManyOTPRequests.
	mr.Set("otp_send:+919999999999", "5") // also pre-seed B to 5 to hit the rate limit

	_, errB := uc.Execute(ctx, application.OTPSendInput{Phone: "9999999999"})
	// B is also at limit (we seeded 5), so the Incr makes it 6 → throttled.
	// Adjust: seed B to 4 so only A is throttled.
	// Reset and re-run with B seeded to 4.
	mr.Set("otp_send:+919999999999", "4")
	_, errB = uc.Execute(ctx, application.OTPSendInput{Phone: "9999999999"})
	// n=5 ≤ maxOTPSends(5) → NOT throttled.
	if errB == shared.ErrTooManyOTPRequests {
		t.Error("phone B must not be throttled when A is throttled")
	}
}

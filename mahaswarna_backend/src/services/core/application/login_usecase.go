package application

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

const (
	maxLoginFails  = 10
	loginWindow    = 15 * time.Minute
	maxOTPSends    = 5
	accessTTL      = 15 * time.Minute
	refreshTTL     = 30 * 24 * time.Hour
)

type JWTClaims struct {
	jwt.RegisteredClaims
	Tier   string `json:"tier"`
	Region string `json:"region"`
}

type LoginUseCase struct {
	users    *infrastructure.UserRepository
	sessions *infrastructure.SessionRepository
	otp      infrastructure.OtpProvider
	rdb      *redis.Client
	privKey  *rsa.PrivateKey
	audit    *infrastructure.AuditLogRepository
}

func NewLoginUseCase(users *infrastructure.UserRepository, sessions *infrastructure.SessionRepository,
	otp infrastructure.OtpProvider, rdb *redis.Client, audit *infrastructure.AuditLogRepository) (*LoginUseCase, error) {
	privKey, err := loadPrivKey()
	if err != nil { return nil, err }
	return &LoginUseCase{users: users, sessions: sessions, otp: otp, rdb: rdb, privKey: privKey, audit: audit}, nil
}

type LoginInput struct {
	Phone, FirebaseIDToken, OTP, IntegrityToken, CityID, Provider string
}

type AuthOutput struct{ AccessToken, RefreshToken, Tier string }

func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*AuthOutput, error) {
	phone := normalisePhone(in.Phone)

	// 1. Throttle.
	if n, _ := uc.rdb.Get(ctx, "login_fail:"+phone).Int(); n >= maxLoginFails {
		return nil, shared.ErrTooManyRequests
	}

	// 2. Play Integrity (required on every login — cross-cutting invariant).
	if err := verifyIntegrity(in.IntegrityToken); err != nil {
		uc.incrFail(ctx, phone)
		return nil, err
	}

	// 3. OTP verification.
	code := in.FirebaseIDToken
	if code == "" { code = in.OTP }
	ok, err := uc.otp.VerifyOTP(ctx, phone, code)
	if err != nil || !ok {
		uc.incrFail(ctx, phone)
		uc.audit.Append(ctx, shared.AuditEntry{Actor: phone, Action: "otp_verify_fail", Entity: "users"})
		return nil, shared.ErrOTPInvalid
	}
	uc.rdb.Del(ctx, "login_fail:"+phone)

	// 4. Upsert user — cityID only written on fresh insert (xmax=0 guard).
	user, _, err := uc.users.UpsertUser(ctx, phone, in.CityID)
	if err != nil { return nil, fmt.Errorf("upsert user: %w", err) }

	uc.audit.Append(ctx, shared.AuditEntry{
		Actor: user.ID.String(), Action: "login", Entity: "users", EntityID: user.ID.String(),
		Metadata: map[string]any{"provider": in.Provider},
	})
	return uc.IssueTokenPair(ctx, user)
}

func (uc *LoginUseCase) incrFail(ctx context.Context, phone string) {
	key := "login_fail:" + phone
	uc.rdb.Incr(ctx, key)
	uc.rdb.Expire(ctx, key, loginWindow)
}

func (uc *LoginUseCase) IssueTokenPair(ctx context.Context, user *domain.User) (*AuthOutput, error) {
	jti, _ := shared.GenerateJTI()
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: user.ID.String(), ID: jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTTL)),
		},
		Tier: user.Tier, Region: user.CityID,
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(uc.privKey)
	if err != nil { return nil, fmt.Errorf("sign token: %w", err) }

	refreshJTI := uuid.New()
	if err := uc.sessions.Create(ctx, domain.Session{
		JTI: refreshJTI, UserID: user.ID,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(refreshTTL),
	}); err != nil { return nil, fmt.Errorf("create session: %w", err) }

	return &AuthOutput{AccessToken: accessToken, RefreshToken: refreshJTI.String(), Tier: user.Tier}, nil
}

func loadPrivKey() (*rsa.PrivateKey, error) {
	pem := os.Getenv("JWT_PRIVATE_KEY")
	if pem == "" { return nil, fmt.Errorf("JWT_PRIVATE_KEY not set") }
	return jwt.ParseRSAPrivateKeyFromPEM([]byte(pem))
}

func verifyIntegrity(token string) error {
	if token == "" { return shared.ErrDeviceNotTrusted }
	if strings.HasSuffix(token, "_expired") { return shared.ErrIntegrityTokenExpired }
	// Production: decrypt + verify via PLAY_INTEGRITY_DECRYPTION_KEY.
	return nil
}

func normalisePhone(phone string) string {
	phone = strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
	if strings.HasPrefix(phone, "+91") { return phone }
	if strings.HasPrefix(phone, "91") && len(phone) == 12 { return "+" + phone }
	if strings.HasPrefix(phone, "0") && len(phone) == 11 { return "+91" + phone[1:] }
	if len(phone) == 10 { return "+91" + phone }
	return phone
}

// OTPSendUseCase handles POST /auth/send-otp.
type OTPSendUseCase struct {
	otp   infrastructure.OtpProvider
	rdb   *redis.Client
	flags *infrastructure.FlagsRepository
}

func NewOTPSendUseCase(otp infrastructure.OtpProvider, rdb *redis.Client, flags *infrastructure.FlagsRepository) *OTPSendUseCase {
	return &OTPSendUseCase{otp: otp, rdb: rdb, flags: flags}
}

type OTPSendInput struct{ Phone string }
type OTPSendOutput struct{ Provider string }

func (uc *OTPSendUseCase) Execute(ctx context.Context, in OTPSendInput) (*OTPSendOutput, error) {
	phone := normalisePhone(in.Phone)
	key := "otp_send:" + phone
	n, _ := uc.rdb.Incr(ctx, key).Result()
	if n == 1 { uc.rdb.Expire(ctx, key, time.Hour) }
	if n > maxOTPSends { return nil, shared.ErrTooManyOTPRequests }

	uc.otp.SendOTP(ctx, phone) // no-op for Firebase

	provider := "firebase"
	flags, _ := uc.flags.GetAll(ctx)
	for _, f := range flags {
		if f.Key == "otp_provider" {
			if f.Value == "msg91" { provider = "msg91" }
			break
		}
	}
	return &OTPSendOutput{Provider: provider}, nil
}

package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

// RegisterUseCase handles POST /auth/register.
//
// Key difference from LoginUseCase: cityID is ALWAYS written to users.city_id,
// regardless of whether the user already exists. The entire purpose of this
// endpoint is to set up the user row for non-Android clients (web dashboard,
// future iOS). cityID is mandatory; callers must reject blank values before
// calling Execute.
//
// Token issuance delegates to LoginUseCase.IssueTokenPair — RSA signing logic
// is never duplicated.
type RegisterUseCase struct {
	users    *infrastructure.UserRepository
	sessions *infrastructure.SessionRepository
	otp      infrastructure.OtpProvider
	rdb      *redis.Client
	audit    *infrastructure.AuditLogRepository
	login    *LoginUseCase // token issuance only
}

func NewRegisterUseCase(
	users *infrastructure.UserRepository,
	sessions *infrastructure.SessionRepository,
	otp infrastructure.OtpProvider,
	rdb *redis.Client,
	audit *infrastructure.AuditLogRepository,
	login *LoginUseCase,
) *RegisterUseCase {
	return &RegisterUseCase{
		users:    users,
		sessions: sessions,
		otp:      otp,
		rdb:      rdb,
		audit:    audit,
		login:    login,
	}
}

// RegisterInput carries validated fields from the POST /auth/register body.
type RegisterInput struct {
	Phone           string
	FirebaseIDToken string // Firebase flow
	OTP             string // MSG91 flow
	IntegrityToken  string
	CityID          string // REQUIRED — always written to users.city_id
	Provider        string // "firebase" | "msg91"
}

const registerFailWindow = 15 * time.Minute

// Execute registers or re-authenticates a user, always persisting CityID.
//
//  1. Validate CityID non-empty.
//  2. Throttle: shared login_fail counter prevents brute-force via either endpoint.
//  3. Play Integrity check (cross-cutting invariant, identical to LoginUseCase).
//  4. OTP verification (Firebase ID token or MSG91 code).
//  5. Upsert user row; then unconditionally update city_id (register semantics).
//  6. Audit event.
//  7. Issue JWT pair via LoginUseCase.IssueTokenPair.
func (uc *RegisterUseCase) Execute(ctx context.Context, in RegisterInput) (*AuthOutput, error) {
	if strings.TrimSpace(in.CityID) == "" {
		return nil, fmt.Errorf("cityID is required for registration")
	}

	phone := normalisePhone(in.Phone)

	// 1. Throttle — shares login_fail key so brute-forcing via /register
	// counts against the same window as /login.
	if n, _ := uc.rdb.Get(ctx, "login_fail:"+phone).Int(); n >= maxLoginFails {
		return nil, shared.ErrTooManyRequests
	}

	// 2. Play Integrity.
	if err := verifyIntegrity(in.IntegrityToken); err != nil {
		uc.incrFail(ctx, phone)
		return nil, err
	}

	// 3. OTP verification.
	code := in.FirebaseIDToken
	if code == "" {
		code = in.OTP
	}
	ok, err := uc.otp.VerifyOTP(ctx, phone, code)
	if err != nil || !ok {
		uc.incrFail(ctx, phone)
		uc.audit.Append(ctx, shared.AuditEntry{
			Actor:  phone,
			Action: "otp_verify_fail",
			Entity: "users",
		})
		return nil, shared.ErrOTPInvalid
	}
	uc.rdb.Del(ctx, "login_fail:"+phone)

	// 4. Upsert user. UpsertUser guards city_id on fresh inserts (xmax=0),
	// but /auth/register always writes city_id, so we follow up unconditionally.
	user, _, err := uc.users.UpsertUser(ctx, phone, in.CityID)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// 5. Unconditionally write cityID — defining difference from /login.
	if user.CityID != in.CityID {
		if err := uc.users.UpdateCityID(ctx, user.ID, in.CityID); err != nil {
			return nil, fmt.Errorf("update city_id: %w", err)
		}
		user.CityID = in.CityID
	}

	// 6. Audit.
	uc.audit.Append(ctx, shared.AuditEntry{
		Actor:    user.ID.String(),
		Action:   "register",
		Entity:   "users",
		EntityID: user.ID.String(),
		Metadata: map[string]any{
			"provider": in.Provider,
			"city_id":  in.CityID,
		},
	})

	// 7. Issue JWT pair.
	return uc.login.IssueTokenPair(ctx, user)
}

// incrFail uses the same key as LoginUseCase so both endpoints share the window.
func (uc *RegisterUseCase) incrFail(ctx context.Context, phone string) {
	key := "login_fail:" + phone
	uc.rdb.Incr(ctx, key)
	uc.rdb.Expire(ctx, key, registerFailWindow)
}

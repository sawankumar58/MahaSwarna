package infrastructure

import "context"

type OtpProvider interface {
	SendOTP(ctx context.Context, phone string) error
	VerifyOTP(ctx context.Context, phone, code string) (bool, error)
}

func NewOtpProvider(flag string, firebase *FirebaseOtpProvider, msg91 *Msg91OtpProvider) OtpProvider {
	switch flag {
	case "firebase": return firebase
	case "msg91":    return msg91
	default:         return &DualOtpProvider{primary: firebase, fallback: msg91}
	}
}

// DualOtpProvider: Firebase first; falls back to MSG91 on infrastructure errors ONLY.
// Credential failures (expired/invalid token) must NOT trigger the fallback.
type DualOtpProvider struct{ primary, fallback OtpProvider }

func (d *DualOtpProvider) SendOTP(ctx context.Context, phone string) error {
	return d.primary.SendOTP(ctx, phone)
}
func (d *DualOtpProvider) VerifyOTP(ctx context.Context, phone, code string) (bool, error) {
	ok, err := d.primary.VerifyOTP(ctx, phone, code)
	if err != nil && isInfraErr(err) {
		return d.fallback.VerifyOTP(ctx, phone, code)
	}
	return ok, err
}

func isInfraErr(err error) bool {
	if err == nil { return false }
	s := err.Error()
	for _, sub := range []string{"deadline exceeded","connection refused","EOF","timeout"} {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub { return true }
			}
		}
	}
	return false
}

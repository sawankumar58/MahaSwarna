package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Msg91OtpProvider struct {
	http       *http.Client
	authKey    string
	templateID string
	expiry     string
}

func NewMsg91OtpProvider() *Msg91OtpProvider {
	return &Msg91OtpProvider{
		http:       &http.Client{Timeout: 5 * time.Second},
		authKey:    os.Getenv("MSG91_AUTH_KEY"),
		templateID: os.Getenv("MSG91_TEMPLATE_ID"),
		expiry:     envOrDefault("MSG91_OTP_EXPIRY_MINUTES", "10"),
	}
}

func (p *Msg91OtpProvider) SendOTP(ctx context.Context, phone string) error {
	url := fmt.Sprintf("https://api.msg91.com/api/v5/otp?authkey=%s&template_id=%s&mobile=91%s&otp_expiry=%s",
		p.authKey, p.templateID, last10digits(phone), p.expiry)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	resp, err := p.http.Do(req)
	if err != nil { return fmt.Errorf("msg91 send: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode >= 500 { return fmt.Errorf("msg91 server error %d", resp.StatusCode) }
	return nil
}

func (p *Msg91OtpProvider) VerifyOTP(ctx context.Context, phone, otp string) (bool, error) {
	url := fmt.Sprintf("https://api.msg91.com/api/v5/otp/verify?authkey=%s&mobile=91%s&otp=%s",
		p.authKey, last10digits(phone), otp)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.http.Do(req)
	if err != nil { return false, fmt.Errorf("msg91 verify: %w", err) }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct{ Type string `json:"type"` }
	json.Unmarshal(body, &r)
	return r.Type == "success", nil
}

func last10digits(phone string) string {
	p := strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
	p = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(p, "+91"), "91"), "0")
	if len(p) >= 10 { return p[len(p)-10:] }
	return p
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

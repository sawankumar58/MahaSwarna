package shared

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

// IssueServiceToken generates HMAC-SHA256(timestamp + INTERNAL_JWT_SECRET).
// timestamp is Unix seconds (use time.Now().Unix()).
func IssueServiceToken(timestamp int64) string {
	secret := os.Getenv("INTERNAL_JWT_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyServiceToken validates X-Service-Token against the timestamp header.
// The timestamp must be within ±30 seconds of now to prevent replay attacks.
//
// Note: hmac.Equal operates on the hex-string bytes of both sides, which is
// safe here because both token and expected are hex-encoded output of the same
// HMAC function. Constant-time comparison prevents timing side-channels.
func VerifyServiceToken(token string, timestamp int64) bool {
	if math.Abs(float64(time.Now().Unix()-timestamp)) > 30 {
		return false
	}
	expected := IssueServiceToken(timestamp)
	// hmac.Equal is constant-time; inputs are hex strings produced by this package.
	return hmac.Equal([]byte(token), []byte(expected))
}

// ServiceTokenHeader returns the two header values needed for a service call.
func ServiceTokenHeader() (token, timestamp string) {
	ts := time.Now().Unix()
	return IssueServiceToken(ts), fmt.Sprintf("%d", ts)
}

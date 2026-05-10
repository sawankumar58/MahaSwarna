package events

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
)

func serviceTokenHeader(ts int64) (token, timestamp string) {
	secret := os.Getenv("INTERNAL_JWT_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	return hex.EncodeToString(mac.Sum(nil)), fmt.Sprintf("%d", ts)
}

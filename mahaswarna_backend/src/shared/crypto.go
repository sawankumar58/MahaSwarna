package shared

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateJTI returns a random 16-byte hex string suitable for use as a JWT ID.
func GenerateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

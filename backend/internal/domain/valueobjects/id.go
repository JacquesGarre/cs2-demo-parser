package valueobjects

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() string {
	buf := make([]byte, 12)
	_, err := rand.Read(buf)
	if err != nil {
		return "fallback-id"
	}

	return hex.EncodeToString(buf)
}

package entities

import (
	"crypto/rand"
	"fmt"
	"time"
)

// generateID creates a unique identifier using timestamp and random bytes
func generateID() string {
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	return fmt.Sprintf("%x_%x", timestamp, randomBytes)
}

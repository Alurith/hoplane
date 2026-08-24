package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const connectionIDLength = 32

// NewConnectionID creates an opaque identifier independent of user-editable
// connection fields. It is used as the stable key for secrets and future
// session state.
func NewConnectionID() (string, error) {
	var raw [connectionIDLength / 2]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate connection ID: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// ValidateConnectionID validates the persisted connection identifier.
func ValidateConnectionID(value string) error {
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("connection ID cannot contain surrounding whitespace")
	}
	if len(value) != connectionIDLength {
		return fmt.Errorf("connection ID must contain %d hexadecimal characters", connectionIDLength)
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("connection ID must contain only lowercase hexadecimal characters")
		}
	}
	return nil
}

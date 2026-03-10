package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type APIKeyManager struct {
}

func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{}
}

func (m *APIKeyManager) Generate() (rawKey string, hashedKey string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	rawKey = "vl_" + hex.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey = hex.EncodeToString(hash[:])

	return rawKey, hashedKey, nil
}

func (m *APIKeyManager) Hash(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

func (m *APIKeyManager) Validate(rawKey, hashedKey string) bool {
	return m.Hash(rawKey) == hashedKey
}

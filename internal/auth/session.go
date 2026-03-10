package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type SessionManager struct {
	store    SessionStore
	duration time.Duration
}

type SessionStore interface {
	CreateSession(userID int64, tokenHash string, expiresAt time.Time) error
	ValidateSession(tokenHash string) (int64, error)
	DeleteSession(tokenHash string) error
}

func NewSessionManager(store SessionStore, duration time.Duration) *SessionManager {
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &SessionManager{
		store:    store,
		duration: duration,
	}
}

func (m *SessionManager) Generate(userID int64) (rawToken string, expiresAt time.Time, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}

	rawToken = hex.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	expiresAt = time.Now().Add(m.duration)

	if err := m.store.CreateSession(userID, tokenHash, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("store session: %w", err)
	}

	return rawToken, expiresAt, nil
}

func (m *SessionManager) Validate(rawToken string) (int64, error) {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	return m.store.ValidateSession(tokenHash)
}

func (m *SessionManager) Invalidate(rawToken string) error {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	return m.store.DeleteSession(tokenHash)
}

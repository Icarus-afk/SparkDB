package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	Memory    uint32
	Time      uint32
	Threads   uint8
	KeyLength uint32
}

var defaultParams = Argon2Params{
	Memory:    64 * 1024,
	Time:      3,
	Threads:   4,
	KeyLength: 32,
}

func generateSalt(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func HashPassword(password string) (string, error) {
	salt, err := generateSalt(16)
	if err != nil {
		return "", err
	}

	p := defaultParams
	hash := argon2.IDKey([]byte(password), []byte(salt), p.Time, p.Memory, p.Threads, p.KeyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString([]byte(salt))
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads, b64Salt, b64Hash)

	return encoded, nil
}

type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*LoginAttempt
	limit    int
	lockout  time.Duration
}

type LoginAttempt struct {
	Count     int
	FirstFail time.Time
}

func NewLoginRateLimiter(limit int, lockout time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts: make(map[string]*LoginAttempt),
		limit:    limit,
		lockout:  lockout,
	}
}

func (l *LoginRateLimiter) IsLocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[key]
	if !ok {
		return false
	}

	if time.Since(a.FirstFail) > l.lockout {
		delete(l.attempts, key)
		return false
	}

	return a.Count >= l.limit
}

func (l *LoginRateLimiter) RecordFail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[key]
	if !ok {
		l.attempts[key] = &LoginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}

	if time.Since(a.FirstFail) > l.lockout {
		l.attempts[key] = &LoginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}

	a.Count++
}

func (l *LoginRateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var version int
	var memory uint32
	var time uint32
	var threads uint8

	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(decodedHash)))

	return subtle.ConstantTimeCompare(decodedHash, computed) == 1, nil
}

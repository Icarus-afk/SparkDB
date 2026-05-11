package auth

import (
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "securePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == "" {
		t.Fatal("hash should not be empty")
	}

	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error: %v", err)
	}
	if !valid {
		t.Fatal("VerifyPassword() should return true for correct password")
	}

	valid, err = VerifyPassword("wrongPassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword(wrong) error: %v", err)
	}
	if valid {
		t.Fatal("VerifyPassword() should return false for wrong password")
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	h1, _ := HashPassword("password")
	h2, _ := HashPassword("password")
	if h1 == h2 {
		t.Fatal("hashes should differ due to random salt")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	_, err := VerifyPassword("pwd", "not-a-valid-hash")
	if err == nil {
		t.Fatal("expected error for invalid hash format")
	}

	_, err = VerifyPassword("pwd", "$argon2id$v=19$m=8,t=1,p=1$salt$hash")
	if err != nil {
		t.Fatal("valid-format hash should not error")
	}
}

func TestAPIKeyGenerateAndHash(t *testing.T) {
	mgr := NewAPIKeyManager()

	raw, hashed, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("raw key should not be empty")
	}

	if !mgr.Validate(raw, hashed) {
		t.Fatal("Validate() should return true for valid key")
	}

	if mgr.Hash(raw) != hashed {
		t.Fatal("Hash() should match generated hashed key")
	}

	if mgr.Validate("wrong-key", hashed) {
		t.Fatal("Validate() should return false for wrong key")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	limiter := NewLoginRateLimiter(3, 60*time.Second)

	if limiter.IsLocked("user1") {
		t.Fatal("should not be locked initially")
	}

	limiter.RecordFail("user1")
	limiter.RecordFail("user1")
	limiter.RecordFail("user1")

	if !limiter.IsLocked("user1") {
		t.Fatal("should be locked after 3 failures")
	}

	limiter.Reset("user1")
	if limiter.IsLocked("user1") {
		t.Fatal("should not be locked after reset")
	}
}

func TestLoginRateLimiter_DifferentKeys(t *testing.T) {
	limiter := NewLoginRateLimiter(2, 60*time.Second)

	limiter.RecordFail("alice")
	limiter.RecordFail("alice")

	if !limiter.IsLocked("alice") {
		t.Fatal("alice should be locked")
	}
	if limiter.IsLocked("bob") {
		t.Fatal("bob should not be locked")
	}
}

func TestJWTManager(t *testing.T) {
	mgr := NewJWTManager("test-secret-key-1234567890123456", 0)

	token, err := mgr.Generate(1, "testuser", "admin")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := mgr.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want %q", claims.Username, "testuser")
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, want %q", claims.Role, "admin")
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", 0)

	_, err := mgr.Validate("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}

	_, err = mgr.Validate("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestJWTManager_WrongSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret-1", 0)
	mgr2 := NewJWTManager("secret-2", 0)

	token, _ := mgr1.Generate(1, "user", "admin")
	_, err := mgr2.Validate(token)
	if err == nil {
		t.Fatal("expected error validating with different secret")
	}
}

func TestJWTManager_EmptySecret(t *testing.T) {
	mgr := NewJWTManager("", 0)
	token, err := mgr.Generate(1, "user", "admin")
	if err != nil {
		t.Fatalf("Generate() with empty secret error: %v", err)
	}

	claims, err := mgr.Validate(token)
	if err != nil {
		t.Fatalf("Validate() with empty secret error: %v", err)
	}
	if claims.Username != "user" {
		t.Errorf("Username = %q, want %q", claims.Username, "user")
	}
}

package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"sparkdb/internal/database"
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

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"Abcdef1", true},
		{"abcdefgh1", true},
		{"ABCDEFGH1", true},
		{"Abcdefgh", true},
	{"Abcdef1!", false},
	{"Abcdefgh1", false},
	{"Password123", false},
	{"SecurePass9", false},
	}
	for _, tt := range tests {
		err := ValidatePasswordStrength(tt.password)
		got := err != nil
		if got != tt.wantErr {
			t.Errorf("ValidatePasswordStrength(%q) error=%v, wantErr=%v", tt.password, err, tt.wantErr)
		}
	}
}

func newTestSystemDB(t *testing.T) *database.SystemDB {
	t.Helper()
	sys, err := database.NewSystemDB(":memory:")
	if err != nil {
		t.Fatalf("NewSystemDB(): %v", err)
	}
	return sys
}

func TestAuthenticatorCreateUser(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, err := a.CreateUser("newuser", "StrongPass1", "developer")
	if err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if user.Username != "newuser" {
		t.Errorf("Username = %q, want %q", user.Username, "newuser")
	}
	if user.Role != "developer" {
		t.Errorf("Role = %q, want %q", user.Role, "developer")
	}
	if !user.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be true for new users")
	}
}

func TestAuthenticatorCreateUser_WeakPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	_, err := a.CreateUser("weakuser", "short", "developer")
	if err == nil {
		t.Fatal("expected error for weak password")
	}
}

func TestAuthenticatorCreateUser_Duplicate(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	a.CreateUser("dupuser", "StrongPass1", "developer")
	_, err := a.CreateUser("dupuser", "OtherPass2", "developer")
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestAuthenticatorLogin(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	a.CreateUser("logintest", "StrongPass1", "admin")

	resp, err := a.Login(LoginRequest{Username: "logintest", Password: "StrongPass1"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("token should not be empty")
	}
	if resp.User.Username != "logintest" {
		t.Errorf("Username = %q, want %q", resp.User.Username, "logintest")
	}
	if !resp.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be true for new users")
	}
}

func TestAuthenticatorLogin_WrongPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	a.CreateUser("wrongpw", "StrongPass1", "developer")
	_, err := a.Login(LoginRequest{Username: "wrongpw", Password: "WrongPass1"}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAuthenticatorLogin_NonexistentUser(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	_, err := a.Login(LoginRequest{Username: "nobody", Password: "StrongPass1"}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestAuthenticatorLogin_Lockout(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{
		SystemDB:    sys,
		JWTSecret:   "test-secret",
		LoginLimit:  3,
		LockoutTime: time.Minute,
	})

	a.CreateUser("lockuser", "StrongPass1", "developer")

	for i := 0; i < 3; i++ {
		a.Login(LoginRequest{Username: "lockuser", Password: "WrongPass1"}, "127.0.0.1")
	}

	_, err := a.Login(LoginRequest{Username: "lockuser", Password: "StrongPass1"}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected lockout error after 3 failed attempts")
	}
}

func TestAuthenticatorEnsureDefaultAdmin(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	if err := a.EnsureDefaultAdmin(); err != nil {
		t.Fatalf("EnsureDefaultAdmin() error: %v", err)
	}

	user, err := sys.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("admin user not found: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("Role = %q, want %q", user.Role, "admin")
	}
	if !user.PasswordChangeRequired {
		t.Error("default admin should require password change")
	}

	if err := a.EnsureDefaultAdmin(); err != nil {
		t.Fatal("EnsureDefaultAdmin() should be idempotent")
	}
}

func TestAuthenticatorUpdateUserPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("passuser", "StrongPass1", "developer")

	if err := a.UpdateUserPassword(user.ID, "NewPass123"); err != nil {
		t.Fatalf("UpdateUserPassword() error: %v", err)
	}

	updated, _ := sys.GetUser(user.ID)
	if !updated.PasswordChangeRequired {
		t.Error("admin password reset should set PasswordChangeRequired")
	}
}

func TestAuthenticatorChangeOwnPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("selfchange", "StrongPass1", "developer")

	if err := a.ChangeOwnPassword(user.ID, "StrongPass1", "NewStrPass9"); err != nil {
		t.Fatalf("ChangeOwnPassword() error: %v", err)
	}

	updated, _ := sys.GetUser(user.ID)
	if updated.PasswordChangeRequired {
		t.Error("own password change should clear PasswordChangeRequired")
	}

	resp, err := a.Login(LoginRequest{Username: "selfchange", Password: "NewStrPass9"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("login with new password error: %v", err)
	}
	if resp.PasswordChangeRequired {
		t.Error("login after own password change should have PasswordChangeRequired=false")
	}
}

func TestAuthenticatorChangeOwnPassword_WrongOldPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("wrongold", "StrongPass1", "developer")

	err := a.ChangeOwnPassword(user.ID, "WrongPass1", "NewStrPass9")
	if err == nil {
		t.Fatal("expected error for wrong old password")
	}
}

func TestAuthenticatorChangeOwnPassword_WeakNewPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("weaknew", "StrongPass1", "developer")

	err := a.ChangeOwnPassword(user.ID, "StrongPass1", "short")
	if err == nil {
		t.Fatal("expected error for weak new password")
	}
}

func TestAuthenticatorEncryptDecryptRawKey(t *testing.T) {
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: newTestSystemDB(t), JWTSecret: "test-secret"})

	encrypted, err := a.encryptRawKey("raw-api-key-value")
	if err != nil {
		t.Fatalf("encryptRawKey() error: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted key should not be empty")
	}

	decrypted, err := a.decryptRawKey(encrypted)
	if err != nil {
		t.Fatalf("decryptRawKey() error: %v", err)
	}
	if decrypted != "raw-api-key-value" {
		t.Errorf("decrypted = %q, want %q", decrypted, "raw-api-key-value")
	}
}

func TestAuthenticatorApiKeyRoundTrip(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("apiuser", "StrongPass1", "admin")

	rawKey, err := a.GenerateAPIKey(user.ID, "test-key")
	if err != nil {
		t.Fatalf("GenerateAPIKey() error: %v", err)
	}
	if rawKey == "" {
		t.Fatal("raw key should not be empty")
	}

	revealed, err := a.RevealAPIKey(1, "StrongPass1")
	if err != nil {
		t.Fatalf("RevealAPIKey() error: %v", err)
	}
	if revealed != rawKey {
		t.Errorf("RevealAPIKey() = %q, want %q", revealed, rawKey)
	}
}

func TestAuthenticatorRevealAPIKey_WrongPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("revealuser", "StrongPass1", "admin")
	a.GenerateAPIKey(user.ID, "test-key")

	_, err := a.RevealAPIKey(1, "WrongPass1")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAuthenticatorLogin_RespectsLockedUntil(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	a.CreateUser("lockedacct", "StrongPass1", "developer")
	sys.LockUser("lockedacct", 5*time.Minute)

	_, err := a.Login(LoginRequest{Username: "lockedacct", Password: "StrongPass1"}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for locked account")
	}
}

func TestAuthenticatorUpdateUserRole(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("roleuser", "StrongPass1", "developer")

	updated, err := a.UpdateUserRole(user.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateUserRole() error: %v", err)
	}
	if updated.Role != "admin" {
		t.Errorf("Role = %q, want %q", updated.Role, "admin")
	}
}

func TestAuthenticateRequest_JWT(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})
	a.CreateUser("jwtuser", "StrongPass1", "admin")

	resp, _ := a.Login(LoginRequest{Username: "jwtuser", Password: "StrongPass1"}, "127.0.0.1")

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+resp.Token)

	user, err := a.AuthenticateRequest(r)
	if err != nil {
		t.Fatalf("AuthenticateRequest() error: %v", err)
	}
	if user.Username != "jwtuser" {
		t.Errorf("Username = %q, want %q", user.Username, "jwtuser")
	}
	if user.AuthType != AuthTypeJWT {
		t.Errorf("AuthType = %q, want %q", user.AuthType, AuthTypeJWT)
	}
}

func TestAuthenticateRequest_APIKey(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})
	user, _ := a.CreateUser("keyuser", "StrongPass1", "admin")
	rawKey, _ := a.GenerateAPIKey(user.ID, "test-key")

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", rawKey)

	au, err := a.AuthenticateRequest(r)
	if err != nil {
		t.Fatalf("AuthenticateRequest() error: %v", err)
	}
	if au.Username != "keyuser" {
		t.Errorf("Username = %q, want %q", au.Username, "keyuser")
	}
	if au.AuthType != AuthTypeAPIKey {
		t.Errorf("AuthType = %q, want %q", au.AuthType, AuthTypeAPIKey)
	}
}

func TestAuthenticateRequest_Session(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})
	user, _ := a.CreateUser("sessuser", "StrongPass1", "admin")

	rawToken, _, _ := a.sessionMgr.Generate(user.ID)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Session "+rawToken)

	au, err := a.AuthenticateRequest(r)
	if err != nil {
		t.Fatalf("AuthenticateRequest() error: %v", err)
	}
	if au.Username != "sessuser" {
		t.Errorf("Username = %q, want %q", au.Username, "sessuser")
	}
	if au.AuthType != AuthTypeSession {
		t.Errorf("AuthType = %q, want %q", au.AuthType, AuthTypeSession)
	}
}

func TestAuthenticateRequest_NoAuth(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	r := httptest.NewRequest("GET", "/", nil)
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for no auth")
	}
}

func TestAuthenticateRequest_InvalidBearer(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer invalid-token")
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthenticateRequest_InvalidAuthHeader(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "InvalidScheme token")
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for unsupported auth type")
	}
}

func TestAuthenticateRequest_InvalidAPIKey(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "invalid-key")
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
}

func TestAuthenticateRequest_APIKeyLockedUser(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})
	user, _ := a.CreateUser("lockedkey", "StrongPass1", "admin")
	rawKey, _ := a.GenerateAPIKey(user.ID, "test-key")
	sys.LockUser("lockedkey", 5*time.Minute)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", rawKey)
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for locked user")
	}
}

func TestAuthenticateRequest_ExpiredJWT(t *testing.T) {
	oldJWT := NewJWTManager("test-secret", -time.Hour)
	token, _ := oldJWT.Generate(1, "expireduser", "admin")

	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	_, err := a.AuthenticateRequest(r)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestSessionManagerGenerateValidateInvalidate(t *testing.T) {
	sys := newTestSystemDB(t)
	sm := NewSessionManager(sys, 30*time.Minute)

	rawToken, expiresAt, err := sm.Generate(42)
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if rawToken == "" {
		t.Fatal("token should not be empty")
	}
	if expiresAt.IsZero() {
		t.Fatal("expiresAt should not be zero")
	}

	uid, err := sm.Validate(rawToken)
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if uid != 42 {
		t.Errorf("uid = %d, want 42", uid)
	}

	sm.Invalidate(rawToken)
	_, err = sm.Validate(rawToken)
	if err == nil {
		t.Fatal("expected error after invalidation")
	}
}

func TestSessionManagerDefaultDuration(t *testing.T) {
	sm := NewSessionManager(nil, 0)
	if sm == nil {
		t.Fatal("NewSessionManager returned nil")
	}
	if sm.duration != 24*time.Hour {
		t.Errorf("duration = %v, want 24h", sm.duration)
	}
}

func TestJWTManagerCustomDuration(t *testing.T) {
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	token, err := mgr.Generate(1, "user", "admin")
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	claims, err := mgr.Validate(token)
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if claims.Username != "user" {
		t.Errorf("Username = %q", claims.Username)
	}
}

func TestLoginRateLimiterLockoutTimeout(t *testing.T) {
	limiter := NewLoginRateLimiter(1, 50*time.Millisecond)
	limiter.RecordFail("shortlock")
	if !limiter.IsLocked("shortlock") {
		t.Fatal("should be locked immediately after fail")
	}
	time.Sleep(60 * time.Millisecond)
	if limiter.IsLocked("shortlock") {
		t.Fatal("should not be locked after timeout expires")
	}
}

func TestLoginRateLimiterRecordAfterTimeout(t *testing.T) {
	limiter := NewLoginRateLimiter(2, 50*time.Millisecond)
	limiter.RecordFail("timeoutkey")
	time.Sleep(60 * time.Millisecond)
	limiter.RecordFail("timeoutkey")
	if limiter.IsLocked("timeoutkey") {
		t.Fatal("should not be locked (first fail expired)")
	}
}

func TestAuthenticatorDeleteUser(t *testing.T) {
	sys := newTestSystemDB(t)
	a := NewAuthenticator(AuthenticatorConfig{SystemDB: sys, JWTSecret: "test-secret"})

	user, _ := a.CreateUser("deleteuser", "StrongPass1", "developer")

	if err := a.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser() error: %v", err)
	}

	_, err := sys.GetUserByUsername("deleteuser")
	if err == nil {
		t.Fatal("expected error for deleted user")
	}
}

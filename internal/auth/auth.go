package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nanodb/internal/database"
)

type contextKey string

const (
	ContextUserKey contextKey = "user"

	AuthTypeJWT     = "jwt"
	AuthTypeAPIKey  = "apikey"
	AuthTypeSession = "session"
)

type AuthUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	AuthType string `json:"auth_type"`
}

type Authenticator struct {
	systemDB    *database.SystemDB
	jwtManager  *JWTManager
	apiKeyMgr   *APIKeyManager
	sessionMgr  *SessionManager
	loginLimiter *LoginRateLimiter
	defaultAdmin bool
}

type AuthenticatorConfig struct {
	SystemDB    *database.SystemDB
	JWTSecret   string
	SessionTTL  time.Duration
	LoginLimit  int
	LockoutTime time.Duration
}

func NewAuthenticator(cfg AuthenticatorConfig) *Authenticator {
	if cfg.LoginLimit == 0 {
		cfg.LoginLimit = 5
	}
	if cfg.LockoutTime == 0 {
		cfg.LockoutTime = 15 * time.Minute
	}

	a := &Authenticator{
		systemDB:    cfg.SystemDB,
		jwtManager:  NewJWTManager(cfg.JWTSecret, cfg.SessionTTL),
		apiKeyMgr:   NewAPIKeyManager(),
		sessionMgr:  NewSessionManager(cfg.SystemDB, cfg.SessionTTL),
		loginLimiter: NewLoginRateLimiter(cfg.LoginLimit, cfg.LockoutTime),
		defaultAdmin: true,
	}

	return a
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string   `json:"token"`
	TokenType string   `json:"token_type"`
	User      AuthUser `json:"user"`
}

func (a *Authenticator) Login(req LoginRequest, ip string) (*LoginResponse, error) {
	lockKey := "login:" + req.Username

	if a.loginLimiter.IsLocked(lockKey) {
		return nil, fmt.Errorf("account temporarily locked due to too many failed attempts")
	}

	user, err := a.systemDB.GetUserByUsername(req.Username)
	if err != nil {
		a.loginLimiter.RecordFail(lockKey)
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("account is locked")
	}

	valid, err := VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !valid {
		a.loginLimiter.RecordFail(lockKey)
		return nil, fmt.Errorf("invalid credentials")
	}

	a.loginLimiter.Reset(lockKey)

	token, err := a.jwtManager.Generate(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &LoginResponse{
		Token:     token,
		TokenType: "bearer",
		User: AuthUser{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
			AuthType: AuthTypeJWT,
		},
	}, nil
}

func (a *Authenticator) AuthenticateRequest(r *http.Request) (*AuthUser, error) {
	authHeader := r.Header.Get("Authorization")
	apiKey := r.Header.Get("X-API-Key")

	if apiKey != "" {
		return a.authenticateAPIKey(apiKey)
	}

	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid authorization header")
		}

		switch strings.ToLower(parts[0]) {
		case "bearer":
			return a.authenticateJWT(parts[1])
		case "session":
			return a.authenticateSession(parts[1])
		default:
			return nil, fmt.Errorf("unsupported authorization type")
		}
	}

	return nil, fmt.Errorf("no authentication provided")
}

func (a *Authenticator) authenticateJWT(tokenString string) (*AuthUser, error) {
	claims, err := a.jwtManager.Validate(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return &AuthUser{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
		AuthType: AuthTypeJWT,
	}, nil
}

func (a *Authenticator) authenticateAPIKey(key string) (*AuthUser, error) {
	keyHash := a.apiKeyMgr.Hash(key)
	user, err := a.systemDB.FindUserByAPIKey(keyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("user account is locked")
	}

	return &AuthUser{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
		AuthType: AuthTypeAPIKey,
	}, nil
}

func (a *Authenticator) authenticateSession(tokenString string) (*AuthUser, error) {
	userID, err := a.sessionMgr.Validate(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	user, err := a.systemDB.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("user account is locked")
	}

	return &AuthUser{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
		AuthType: AuthTypeSession,
	}, nil
}

func (a *Authenticator) CreateUser(username, password, role string) (*database.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	return a.systemDB.CreateUser(username, hash, role)
}

func (a *Authenticator) GenerateAPIKey(userID int64, name string) (string, error) {
	rawKey, hashedKey, err := a.apiKeyMgr.Generate()
	if err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}

	if err := a.systemDB.CreateAPIKey(userID, hashedKey, name, nil); err != nil {
		return "", fmt.Errorf("store API key: %w", err)
	}

	return rawKey, nil
}

func (a *Authenticator) EnsureDefaultAdmin() error {
	_, err := a.systemDB.GetUserByUsername("admin")
	if err == nil {
		return nil
	}

	hash, err := HashPassword("admin")
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}

	_, err = a.systemDB.CreateUser("admin", hash, "admin")
	return err
}

func UserFromContext(ctx context.Context) *AuthUser {
	user, ok := ctx.Value(ContextUserKey).(*AuthUser)
	if !ok {
		return nil
	}
	return user
}

func ContextWithUser(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, ContextUserKey, user)
}

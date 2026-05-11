package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SystemDB struct {
	db *sql.DB
}

type User struct {
	ID                     int64      `json:"id"`
	Username               string     `json:"username"`
	PasswordHash           string     `json:"-"`
	Role                   string     `json:"role"`
	CreatedAt              time.Time  `json:"created_at"`
	LockedUntil            *time.Time `json:"locked_until,omitempty"`
	PasswordChangeRequired bool       `json:"-"`
}

type APIKey struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Name         string     `json:"name"`
	Prefix       string     `json:"prefix"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	EncryptedKey string     `json:"-"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Username  string    `json:"username"`
	IPAddress string    `json:"ip_address"`
	Query     string    `json:"query"`
	Endpoint  string    `json:"endpoint"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type ReplicationEntry struct {
	ID           int64     `json:"id"`
	DatabaseName string    `json:"database_name"`
	Query        string    `json:"query"`
	ExecutedAt   time.Time `json:"executed_at"`
}

type ReplicationState struct {
	Role           string    `json:"role"`
	PrimaryURL     string    `json:"primary_url"`
	LastAppliedID  int64     `json:"last_applied_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func NewSystemDB(path string) (*SystemDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open system db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping system db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate system db: %w", err)
	}

	return &SystemDB{db: db}, nil
}

func (s *SystemDB) Close() error {
	return s.db.Close()
}

func (s *SystemDB) DB() *sql.DB {
	return s.db
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	type migration struct {
		version     int
		description string
		sql         string
	}

	migrations := []migration{
		{
			version: 1, description: "initial schema",
			sql: `
				CREATE TABLE IF NOT EXISTS users (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					username TEXT UNIQUE NOT NULL,
					password_hash TEXT NOT NULL,
					role TEXT NOT NULL DEFAULT 'developer',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					locked_until DATETIME
				);
				CREATE TABLE IF NOT EXISTS api_keys (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					key_hash TEXT NOT NULL,
					name TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					expires_at DATETIME,
					FOREIGN KEY (user_id) REFERENCES users(id)
				);
				CREATE TABLE IF NOT EXISTS sessions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					token TEXT UNIQUE NOT NULL,
					expires_at DATETIME NOT NULL,
					FOREIGN KEY (user_id) REFERENCES users(id)
				);
				CREATE TABLE IF NOT EXISTS audit_logs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER,
					username TEXT,
					ip_address TEXT,
					query TEXT,
					endpoint TEXT,
					status TEXT,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				CREATE TABLE IF NOT EXISTS replication_log (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					database_name TEXT NOT NULL,
					query TEXT NOT NULL,
					executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				CREATE TABLE IF NOT EXISTS replication_state (
					id INTEGER PRIMARY KEY CHECK (id = 1),
					role TEXT NOT NULL DEFAULT 'standalone',
					primary_url TEXT DEFAULT '',
					last_applied_id INTEGER DEFAULT 0,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
				CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
				CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
			`,
		},
		{
			version: 2, description: "add api_key columns (prefix, encrypted_key)",
			sql: `
				ALTER TABLE api_keys ADD COLUMN prefix TEXT DEFAULT '';
				ALTER TABLE api_keys ADD COLUMN encrypted_key TEXT DEFAULT '';
			`,
		},
		{
			version: 3, description: "add password_change_required to users",
			sql: `
				ALTER TABLE users ADD COLUMN password_change_required INTEGER NOT NULL DEFAULT 0;
			`,
		},
	}

	for _, m := range migrations {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}
		if count > 0 {
			continue
		}

		if _, err := db.Exec(m.sql); err != nil {
			if m.version == 2 && isDuplicateColumnErr(err) {
				_, _ = db.Exec("INSERT INTO schema_migrations (version, description) VALUES (?, ?)", m.version, m.description)
				continue
			}
			return fmt.Errorf("migration %d (%s): %w", m.version, m.description, err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version, description) VALUES (?, ?)", m.version, m.description); err != nil {
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
	}

	return nil
}

func isDuplicateColumnErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "duplicate column") || strings.Contains(err.Error(), "already exists"))
}

func (s *SystemDB) CreateUser(username, passwordHash, role string) (*User, error) {
	result, err := s.db.Exec(
		"INSERT INTO users (username, password_hash, role, password_change_required) VALUES (?, ?, ?, 1)",
		username, passwordHash, role,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, _ := result.LastInsertId()
	return s.GetUser(id)
}

func (s *SystemDB) GetUser(id int64) (*User, error) {
	row := s.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at, locked_until, password_change_required FROM users WHERE id = ?", id,
	)

	u := &User{}
	var lockedUntil sql.NullTime
	var pwdChange int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lockedUntil, &pwdChange); err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	u.PasswordChangeRequired = pwdChange == 1
	return u, nil
}

func (s *SystemDB) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at, locked_until, password_change_required FROM users WHERE username = ?", username,
	)

	u := &User{}
	var lockedUntil sql.NullTime
	var pwdChange int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lockedUntil, &pwdChange); err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	u.PasswordChangeRequired = pwdChange == 1
	return u, nil
}

func (s *SystemDB) ListUsers() ([]*User, error) {
	rows, err := s.db.Query("SELECT id, username, role, created_at, locked_until, password_change_required FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		var lockedUntil sql.NullTime
		var pwdChange int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &lockedUntil, &pwdChange); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if lockedUntil.Valid {
			u.LockedUntil = &lockedUntil.Time
		}
		u.PasswordChangeRequired = pwdChange == 1
		users = append(users, u)
	}
	return users, nil
}

func (s *SystemDB) LockUser(username string, duration time.Duration) error {
	until := time.Now().Add(duration)
	_, err := s.db.Exec("UPDATE users SET locked_until = ? WHERE username = ?", until, username)
	return err
}

func (s *SystemDB) UnlockUser(username string) error {
	_, err := s.db.Exec("UPDATE users SET locked_until = NULL WHERE username = ?", username)
	return err
}

func (s *SystemDB) CreateAPIKey(userID int64, keyHash, name, prefix string, expiresAt *time.Time, encryptedKey string) error {
	var err error
	if expiresAt != nil {
		_, err = s.db.Exec(
			"INSERT INTO api_keys (user_id, key_hash, name, prefix, expires_at, encrypted_key) VALUES (?, ?, ?, ?, ?, ?)",
			userID, keyHash, name, prefix, *expiresAt, encryptedKey,
		)
	} else {
		_, err = s.db.Exec(
			"INSERT INTO api_keys (user_id, key_hash, name, prefix, encrypted_key) VALUES (?, ?, ?, ?, ?)",
			userID, keyHash, name, prefix, encryptedKey,
		)
	}
	return err
}

func (s *SystemDB) GetAPIKey(id int64) (*APIKey, error) {
	row := s.db.QueryRow(
		"SELECT id, user_id, name, prefix, created_at, expires_at, encrypted_key FROM api_keys WHERE id = ?", id,
	)
	k := &APIKey{}
	var expiresAt sql.NullTime
	if err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.CreatedAt, &expiresAt, &k.EncryptedKey); err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	return k, nil
}

func (s *SystemDB) ListAPIKeys() ([]*APIKey, error) {
	rows, err := s.db.Query("SELECT id, user_id, name, prefix, created_at, expires_at, encrypted_key FROM api_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var keys []*APIKey
	for rows.Next() {
		k := &APIKey{}
		var expiresAt sql.NullTime
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.CreatedAt, &expiresAt, &k.EncryptedKey); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if expiresAt.Valid {
			k.ExpiresAt = &expiresAt.Time
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *SystemDB) UpdateUserRole(id int64, role string) error {
	_, err := s.db.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	return err
}

func (s *SystemDB) UpdateUsername(id int64, username string) error {
	_, err := s.db.Exec("UPDATE users SET username = ? WHERE id = ?", username, id)
	return err
}

func (s *SystemDB) UpdateUserPassword(id int64, passwordHash string) error {
	_, err := s.db.Exec("UPDATE users SET password_hash = ?, password_change_required = 0 WHERE id = ?", passwordHash, id)
	return err
}

func (s *SystemDB) SetPasswordChangeRequired(id int64, required bool) error {
	val := 0
	if required {
		val = 1
	}
	_, err := s.db.Exec("UPDATE users SET password_change_required = ? WHERE id = ?", val, id)
	return err
}

func (s *SystemDB) DeleteUser(id int64) error {
	_, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func (s *SystemDB) DeleteAPIKey(id int64) error {
	_, err := s.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	return err
}

func (s *SystemDB) FindUserByAPIKey(keyHash string) (*User, error) {
	row := s.db.QueryRow(`
		SELECT u.id, u.username, u.password_hash, u.role, u.created_at, u.locked_until
		FROM api_keys k JOIN users u ON k.user_id = u.id
		WHERE k.key_hash = ? AND (k.expires_at IS NULL OR k.expires_at > datetime('now'))
	`, keyHash)

	u := &User{}
	var lockedUntil sql.NullTime
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lockedUntil); err != nil {
		return nil, fmt.Errorf("find by api key: %w", err)
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	return u, nil
}

func (s *SystemDB) CreateSession(userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO sessions (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, expiresAt,
	)
	return err
}

func (s *SystemDB) ValidateSession(tokenHash string) (int64, error) {
	row := s.db.QueryRow(
		"SELECT user_id FROM sessions WHERE token = ? AND expires_at > datetime('now')",
		tokenHash,
	)
	var userID int64
	if err := row.Scan(&userID); err != nil {
		return 0, fmt.Errorf("invalid session: %w", err)
	}
	return userID, nil
}

func (s *SystemDB) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", tokenHash)
	return err
}

func (s *SystemDB) LogAudit(userID *int64, username, ip, query, endpoint, status string) error {
	_, err := s.db.Exec(
		"INSERT INTO audit_logs (user_id, username, ip_address, query, endpoint, status) VALUES (?, ?, ?, ?, ?, ?)",
		userID, username, ip, query, endpoint, status,
	)
	return err
}

func (s *SystemDB) GetAuditLogs(limit int) ([]*AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, username, ip_address, query, endpoint, status, timestamp
		FROM audit_logs ORDER BY timestamp DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.IPAddress, &l.Query, &l.Endpoint, &l.Status, &l.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *SystemDB) LogReplication(databaseName, query string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO replication_log (database_name, query) VALUES (?, ?)",
		databaseName, query,
	)
	if err != nil {
		return 0, fmt.Errorf("log replication: %w", err)
	}
	return result.LastInsertId()
}

func (s *SystemDB) GetReplicationLog(since, limit int64) ([]*ReplicationEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, database_name, query, executed_at
		FROM replication_log
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?
	`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get replication log: %w", err)
	}
	defer rows.Close()

	var entries []*ReplicationEntry
	for rows.Next() {
		e := &ReplicationEntry{}
		if err := rows.Scan(&e.ID, &e.DatabaseName, &e.Query, &e.ExecutedAt); err != nil {
			return nil, fmt.Errorf("scan replication entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *SystemDB) GetReplicationState() (*ReplicationState, error) {
	row := s.db.QueryRow(`
		SELECT role, primary_url, last_applied_id, updated_at
		FROM replication_state WHERE id = 1
	`)
	st := &ReplicationState{}
	if err := row.Scan(&st.Role, &st.PrimaryURL, &st.LastAppliedID, &st.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return &ReplicationState{Role: "standalone", LastAppliedID: 0}, nil
		}
		return nil, fmt.Errorf("get replication state: %w", err)
	}
	return st, nil
}

func (s *SystemDB) InitReplicationState(role, primaryURL string) error {
	_, err := s.db.Exec(`
		INSERT INTO replication_state (id, role, primary_url, last_applied_id, updated_at)
		VALUES (1, ?, ?, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET role = excluded.role, primary_url = excluded.primary_url, updated_at = CURRENT_TIMESTAMP
	`, role, primaryURL)
	return err
}

func (s *SystemDB) UpdateReplicationAppliedID(id int64) error {
	_, err := s.db.Exec(
		"UPDATE replication_state SET last_applied_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1",
		id,
	)
	return err
}

func (s *SystemDB) CleanReplicationLog(beforeID int64) error {
	_, err := s.db.Exec("DELETE FROM replication_log WHERE id <= ?", beforeID)
	return err
}

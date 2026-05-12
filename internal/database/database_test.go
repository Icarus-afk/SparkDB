package database

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sparkdb/internal/encryption"
	"sparkdb/internal/monitor"
)

func newTestSystemDB(t *testing.T) *SystemDB {
	t.Helper()
	sys, err := NewSystemDB(":memory:")
	if err != nil {
		t.Fatalf("NewSystemDB(): %v", err)
	}
	return sys
}

func TestNewSystemDB(t *testing.T) {
	sys := newTestSystemDB(t)
	if sys == nil {
		t.Fatal("NewSystemDB returned nil")
	}
	if err := sys.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
}

func TestSystemDB_DB(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	db := sys.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("DB().Ping(): %v", err)
	}
}

func TestSystemDB_CreateAndGetUser(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, err := sys.CreateUser("testuser", "hash123", "developer")
	if err != nil {
		t.Fatalf("CreateUser(): %v", err)
	}
	if u.Username != "testuser" {
		t.Errorf("Username = %q, want %q", u.Username, "testuser")
	}
	if u.Role != "developer" {
		t.Errorf("Role = %q, want %q", u.Role, "developer")
	}
	if !u.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be true")
	}

	got, err := sys.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser(): %v", err)
	}
	if got.Username != "testuser" {
		t.Errorf("GetUser().Username = %q", got.Username)
	}
}

func TestSystemDB_GetUserByUsername(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	sys.CreateUser("byuser", "hash", "admin")

	u, err := sys.GetUserByUsername("byuser")
	if err != nil {
		t.Fatalf("GetUserByUsername(): %v", err)
	}
	if u.Role != "admin" {
		t.Errorf("Role = %q, want admin", u.Role)
	}

	_, err = sys.GetUserByUsername("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestSystemDB_ListUsers(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	sys.CreateUser("a", "hash", "admin")
	sys.CreateUser("b", "hash", "developer")

	users, err := sys.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers(): %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}
}

func TestSystemDB_UpdateUserRole(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("rolechange", "hash", "developer")
	sys.UpdateUserRole(u.ID, "admin")

	updated, _ := sys.GetUser(u.ID)
	if updated.Role != "admin" {
		t.Errorf("Role = %q, want admin", updated.Role)
	}
}

func TestSystemDB_UpdateUserPassword(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("passchange", "oldhash", "developer")
	sys.UpdateUserPassword(u.ID, "newhash")

	updated, _ := sys.GetUser(u.ID)
	if updated.PasswordHash != "newhash" {
		t.Errorf("PasswordHash = %q, want newhash", updated.PasswordHash)
	}
	if updated.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be false after password update")
	}
}

func TestSystemDB_SetPasswordChangeRequired(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("pwdreq", "hash", "developer")
	sys.UpdateUserPassword(u.ID, "newhash")
	sys.SetPasswordChangeRequired(u.ID, true)

	updated, _ := sys.GetUser(u.ID)
	if !updated.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be true after SetPasswordChangeRequired(true)")
	}
}

func TestSystemDB_SetPasswordChangeRequired_False(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("pwdreqf", "hash", "developer")
	sys.SetPasswordChangeRequired(u.ID, false)

	updated, _ := sys.GetUser(u.ID)
	if updated.PasswordChangeRequired {
		t.Error("PasswordChangeRequired should be false")
	}
}

func TestSystemDB_DeleteUser(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("delete", "hash", "developer")
	sys.DeleteUser(u.ID)

	_, err := sys.GetUser(u.ID)
	if err == nil {
		t.Fatal("expected error for deleted user")
	}
}

func TestSystemDB_LockUnlockUser(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	sys.CreateUser("lockable", "hash", "developer")

	sys.LockUser("lockable", 5*time.Minute)
	u, _ := sys.GetUserByUsername("lockable")
	if u.LockedUntil == nil {
		t.Fatal("LockedUntil should not be nil after lock")
	}
	if time.Now().After(*u.LockedUntil) {
		t.Fatal("LockedUntil should be in the future")
	}

	sys.UnlockUser("lockable")
	u, _ = sys.GetUserByUsername("lockable")
	if u.LockedUntil != nil {
		t.Fatal("LockedUntil should be nil after unlock")
	}
}

func TestSystemDB_APIKeyCRUD(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("keyowner", "hash", "admin")

	exp := time.Now().Add(24 * time.Hour)
	err := sys.CreateAPIKey(u.ID, "keyhash", "mykey", "sk_", &exp, "encrypted")
	if err != nil {
		t.Fatalf("CreateAPIKey(): %v", err)
	}

	key, err := sys.GetAPIKey(1)
	if err != nil {
		t.Fatalf("GetAPIKey(): %v", err)
	}
	if key.Name != "mykey" {
		t.Errorf("Name = %q, want mykey", key.Name)
	}
	if key.Prefix != "sk_" {
		t.Errorf("Prefix = %q, want sk_", key.Prefix)
	}

	err = sys.CreateAPIKey(u.ID, "keyhash2", "key2", "sk_", nil, "encrypted2")
	if err != nil {
		t.Fatalf("CreateAPIKey(no expiry): %v", err)
	}

	keys, err := sys.ListAPIKeys()
	if err != nil {
		t.Fatalf("ListAPIKeys(): %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("got %d keys, want 2", len(keys))
	}

	sys.DeleteAPIKey(1)
	_, err = sys.GetAPIKey(1)
	if err == nil {
		t.Fatal("expected error for deleted key")
	}
}

func TestSystemDB_FindUserByAPIKey(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("findkey", "hash", "admin")
	sys.CreateAPIKey(u.ID, "uniquehash", "findkey", "sk_", nil, "enc")

	user, err := sys.FindUserByAPIKey("uniquehash")
	if err != nil {
		t.Fatalf("FindUserByAPIKey(): %v", err)
	}
	if user.Username != "findkey" {
		t.Errorf("Username = %q, want findkey", user.Username)
	}

	_, err = sys.FindUserByAPIKey("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key hash")
	}
}

func TestSystemDB_Sessions(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	u, _ := sys.CreateUser("sessuser", "hash", "developer")

	exp := time.Now().Add(1 * time.Hour)
	err := sys.CreateSession(u.ID, "tokenhash", exp)
	if err != nil {
		t.Fatalf("CreateSession(): %v", err)
	}

	uid, err := sys.ValidateSession("tokenhash")
	if err != nil {
		t.Fatalf("ValidateSession(): %v", err)
	}
	if uid != u.ID {
		t.Errorf("UserID = %d, want %d", uid, u.ID)
	}

	_, err = sys.ValidateSession("invalidtoken")
	if err == nil {
		t.Fatal("expected error for invalid session")
	}

	sys.DeleteSession("tokenhash")
	_, err = sys.ValidateSession("tokenhash")
	if err == nil {
		t.Fatal("expected error after deleting session")
	}
}

func TestSystemDB_AuditLogs(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	uid := int64(1)
	err := sys.LogAudit(&uid, "testuser", "127.0.0.1", "SELECT 1", "/query", "success")
	if err != nil {
		t.Fatalf("LogAudit(): %v", err)
	}

	err = sys.LogAudit(nil, "anonymous", "10.0.0.1", "SELECT 2", "/query", "success")
	if err != nil {
		t.Fatalf("LogAudit(nil user): %v", err)
	}

	sys.FlushAuditLog()

	logs, err := sys.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("GetAuditLogs(): %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("got %d logs, want 2", len(logs))
	}

	logs, err = sys.GetAuditLogs(1)
	if err != nil {
		t.Fatalf("GetAuditLogs(1): %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("got %d logs, want 1", len(logs))
	}

	logs, err = sys.GetAuditLogs(0)
	if err != nil {
		t.Fatalf("GetAuditLogs(0): %v", err)
	}
	if len(logs) == 0 {
		t.Error("GetAuditLogs(0) should default to 100 limit")
	}
}

func TestSystemDB_ReplicationLog(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	id, err := sys.LogReplication("main", "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Fatalf("LogReplication(): %v", err)
	}
	if id <= 0 {
		t.Errorf("ID = %d, want > 0", id)
	}

	sys.LogReplication("main", "INSERT INTO t VALUES (2)")

	entries, err := sys.GetReplicationLog(0, 10)
	if err != nil {
		t.Fatalf("GetReplicationLog(): %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}

	entries, err = sys.GetReplicationLog(id, 10)
	if err != nil {
		t.Fatalf("GetReplicationLog(since): %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

func TestSystemDB_ReplicationState(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	state, err := sys.GetReplicationState()
	if err != nil {
		t.Fatalf("GetReplicationState() initial: %v", err)
	}
	if state.Role != "standalone" {
		t.Errorf("initial Role = %q, want standalone", state.Role)
	}

	sys.InitReplicationState("primary", "")

	state, _ = sys.GetReplicationState()
	if state.Role != "primary" {
		t.Errorf("Role = %q, want primary", state.Role)
	}

	sys.UpdateReplicationAppliedID(42)

	state, _ = sys.GetReplicationState()
	if state.LastAppliedID != 42 {
		t.Errorf("LastAppliedID = %d, want 42", state.LastAppliedID)
	}

	sys.CleanReplicationLog(50)
}

func TestSystemDB_InitReplicationState_Upsert(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	sys.InitReplicationState("primary", "")
	sys.InitReplicationState("replica", "http://primary:9600")

	state, _ := sys.GetReplicationState()
	if state.Role != "replica" {
		t.Errorf("Role = %q, want replica (upsert)", state.Role)
	}
	if state.PrimaryURL != "http://primary:9600" {
		t.Errorf("PrimaryURL = %q", state.PrimaryURL)
	}
}

func TestSystemDB_Migration(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	var count int
	sys.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count < 3 {
		t.Errorf("expected >= 3 migrations, got %d", count)
	}
}

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerOpenClose(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	db, err := m.Open("testdb")
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	if db == nil {
		t.Fatal("Open returned nil db")
	}

	db2, err := m.Open("testdb")
	if err != nil {
		t.Fatalf("Open() second call: %v", err)
	}
	if db2 != db {
		t.Error("Open() should return cached connection")
	}

	names := m.List()
	if len(names) != 1 {
		t.Errorf("List() = %v, want [testdb]", names)
	}

	m.Close("testdb")
	names = m.List()
	if len(names) != 0 {
		t.Errorf("List() after close = %v, want empty", names)
	}
}

func TestManagerClose_NonExistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	if err := m.Close("nonexistent"); err != nil {
		t.Errorf("Close(nonexistent) should not error: %v", err)
	}
}

func TestManagerOpenMultiple(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	m.Open("db1")
	m.Open("db2")

	names := m.List()
	if len(names) != 2 {
		t.Errorf("List() = %v, want 2 databases", names)
	}
}

func TestManagerCloseNonExistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	if err := m.Close("nonexistent"); err != nil {
		t.Errorf("Close(nonexistent) should not error: %v", err)
	}
}

func TestManagerDataDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	if m.DataDir() != dir {
		t.Errorf("DataDir() = %q, want %q", m.DataDir(), dir)
	}
}

func TestManagerListAll(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	m.Open("testdb")
	names := m.ListAll()
	found := false
	for _, n := range names {
		if n == "testdb" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListAll() should include testdb, got %v", names)
	}
}

func TestListAll_SkipsSystemDB(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, true, 10)

	os.WriteFile(filepath.Join(dir, "sparkdb_system.db"), []byte("SQLite format 3\x00x"), 0644)

	names := m.ListAll()
	for _, n := range names {
		if n == "sparkdb_system.db" {
			t.Error("ListAll() should skip sparkdb_system.db")
		}
	}
}

func TestListAll_SkipsWALFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "testdb"), []byte("SQLite format 3\x00x"), 0644)
	os.WriteFile(filepath.Join(dir, "testdb-wal"), []byte("wal"), 0644)
	os.WriteFile(filepath.Join(dir, "testdb-shm"), []byte("shm"), 0644)

	m := NewManager(dir, true, 10)
	names := m.ListAll()

	found := false
	for _, n := range names {
		if n == "testdb" {
			found = true
		}
		if n == "testdb-wal" || n == "testdb-shm" {
			t.Errorf("should skip wal/shm files, got %q", n)
		}
	}
	if !found {
		t.Error("should find testdb")
	}
}

func TestListAll_SkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".hidden.db"), []byte("SQLite format 3\x00x"), 0644)

	m := NewManager(dir, true, 10)
	names := m.ListAll()
	for _, n := range names {
		if strings.HasPrefix(n, ".") {
			t.Errorf("should skip hidden files, got %q", n)
		}
	}
}

func TestListAll_SkipsKnownExts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"readme.md", "config.json", "styles.css"} {
		os.WriteFile(filepath.Join(dir, name), []byte("not sqlite"), 0644)
	}

	m := NewManager(dir, true, 10)
	names := m.ListAll()
	for _, n := range names {
		t.Errorf("should not list %q (known non-sqlite ext)", n)
	}
}

func TestListAll_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	m := NewManager(dir, true, 10)
	names := m.ListAll()
	for _, n := range names {
		if n == "subdir" {
			t.Errorf("should skip directories")
		}
	}
}

func TestListAll_WithWALFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "testdb"), []byte("SQLite format 3\x00x"), 0644)
	os.WriteFile(filepath.Join(dir, "testdb-wal"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "testdb-shm"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "testdb-journal"), []byte{}, 0644)

	m := NewManager(dir, true, 10)
	names := m.ListAll()

	found := false
	for _, n := range names {
		if n == "testdb" {
			found = true
		}
	}
	if !found {
		t.Error("ListAll() should find testdb")
	}
}

func TestExecutor_Execute(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	res, err := e.Execute("testdb", "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Execute CREATE: %v", err)
	}
	if res.Error != "" {
		t.Errorf("CREATE error: %s", res.Error)
	}

	res, err = e.Execute("testdb", "INSERT INTO t VALUES (1, 'hello')")
	if err != nil {
		t.Fatalf("Execute INSERT: %v", err)
	}

	res, err = e.Execute("testdb", "SELECT * FROM t")
	if err != nil {
		t.Fatalf("Execute SELECT: %v", err)
	}
	if len(res.Columns) != 2 {
		t.Errorf("got %d columns, want 2", len(res.Columns))
	}
	if len(res.Rows) != 1 {
		t.Errorf("got %d rows, want 1", len(res.Rows))
	}

	res, err = e.Execute("testdb", "")
	if err != nil {
		t.Fatalf("Execute empty: %v", err)
	}
	if res.Error != "empty query" {
		t.Errorf("expected 'empty query', got %q", res.Error)
	}
}

func TestExecutor_ExecuteContext(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	res, err := e.ExecuteContext(context.Background(), "ctxdb", "CREATE TABLE ctx (id INT)")
	if err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if res.Error != "" {
		t.Errorf("error: %s", res.Error)
	}
}

func TestExecutor_ExecuteError(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	e.Execute("errordb", "CREATE TABLE t (id INT)")
	res, err := e.Execute("errordb", "INVALID SQL")
	if err != nil {
		t.Fatalf("Execute invalid: %v", err)
	}
	if res.Error == "" {
		t.Fatal("expected error for invalid SQL")
	}
}

func TestExecutor_Transaction(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	e.Execute("txdb", "CREATE TABLE t (id INT)")

	tres, err := e.ExecuteTransaction("txdb", []string{
		"INSERT INTO t VALUES (1)",
		"INSERT INTO t VALUES (2)",
	})
	if err != nil {
		t.Fatalf("ExecuteTransaction: %v", err)
	}
	if tres.Error != "" {
		t.Errorf("tx error: %s", tres.Error)
	}
	if len(tres.Results) != 2 {
		t.Errorf("got %d results, want 2", len(tres.Results))
	}

	tres, err = e.ExecuteTransaction("txdb", []string{
		"INSERT INTO t VALUES (3)",
		"INVALID SQL",
	})
	if err != nil {
		t.Fatalf("ExecuteTransaction: %v", err)
	}
	if len(tres.Results) != 2 {
		t.Errorf("got %d results, want 2", len(tres.Results))
	}
}

func TestExecutor_TransactionContext(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	e.Execute("txctx", "CREATE TABLE t (id INT)")
	tres, err := e.ExecuteTransactionContext(context.Background(), "txctx", []string{"INSERT INTO t VALUES (1)"})
	if err != nil {
		t.Fatalf("ExecuteTransactionContext: %v", err)
	}
	if tres.Error != "" {
		t.Errorf("error: %s", tres.Error)
	}
}

func TestExecutor_TransactionSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	e.Execute("txempty", "CREATE TABLE t (id INT)")
	tres, err := e.ExecuteTransaction("txempty", []string{"", "  ", "INSERT INTO t VALUES (1)"})
	if err != nil {
		t.Fatalf("ExecuteTransaction: %v", err)
	}
	if tres.Error != "" {
		t.Errorf("error: %s", tres.Error)
	}
	if len(tres.Results) != 1 {
		t.Errorf("got %d results, want 1", len(tres.Results))
	}
}

func TestExecutor_ListDatabases(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	e := NewExecutor(m)

	e.Execute("db_a", "CREATE TABLE t (id INT)")
	e.Execute("db_b", "CREATE TABLE t (id INT)")

	dbs := e.ListDatabases()
	if len(dbs) < 2 {
		t.Errorf("ListDatabases() = %v, want at least 2", dbs)
	}
}

func TestExecutor_WithMonitor(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	mon := monitor.New(mockDBProvider{})
	e := NewExecutorWithMonitor(m, mon)

	e.Execute("mondb", "CREATE TABLE t (id INT)")

	stats := mon.Stats()
	if stats.TotalQueries <= 0 {
		t.Errorf("TotalQueries = %d, want > 0", stats.TotalQueries)
	}
}

type mockDBProvider struct{}

func (mockDBProvider) List() []string    { return nil }
func (mockDBProvider) ListAll() []string { return nil }
func (mockDBProvider) DataDir() string   { return "." }

func TestNewEncryptedManager(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := encryption.NewCipher(key)

	m := NewEncryptedManager(dir, true, 5, c)
	if m == nil {
		t.Fatal("NewEncryptedManager returned nil")
	}
	if m.cipher == nil {
		t.Fatal("cipher should not be nil")
	}
	if m.walMode {
		t.Error("walMode should be false for encrypted manager")
	}
	if m.maxConns != 5 {
		t.Errorf("maxConns = %d, want 5", m.maxConns)
	}
}

func TestEncryptedManagerOpenClose(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := encryption.NewCipher(key)

	m := NewEncryptedManager(dir, false, 1, c)

	db, err := m.Open("encdb")
	if err != nil {
		t.Fatalf("Encrypted Open(): %v", err)
	}
	if db == nil {
		t.Fatal("Open returned nil db")
	}

	_, err = db.Exec("CREATE TABLE t (id INT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	db.Close()

	m.Close("encdb")

	db, err = m.Open("encdb")
	if err != nil {
		t.Fatalf("re-open encrypted: %v", err)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM t").Scan(&count)
	db.Close()
}

func TestEncryptedManager_ActivePath(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := encryption.NewCipher(key)

	m := NewEncryptedManager(dir, false, 1, c)

	path := m.ActivePath("test")
	if path == "" {
		t.Error("ActivePath should not be empty")
	}
}

func TestEncryptedManager_WALDisabled(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := encryption.NewCipher(key)

	m := NewEncryptedManager(dir, true, 1, c)
	if m.walMode {
		t.Error("encrypted manager must force walMode=false")
	}
}

func TestManagerCloseAll(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)
	m.Open("db1")
	m.Open("db2")

	if err := m.CloseAll(); err != nil {
		t.Fatalf("CloseAll(): %v", err)
	}
	names := m.List()
	if len(names) != 0 {
		t.Errorf("List() after CloseAll = %v, want empty", names)
	}
}

func TestManagerCloseAll_WithEncrypted(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := encryption.NewCipher(key)

	m := NewEncryptedManager(dir, false, 1, c)
	m.Open("enc1")

	if err := m.CloseAll(); err != nil {
		t.Fatalf("CloseAll() encrypted: %v", err)
	}
}

func TestIsDuplicateColumnErr(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("duplicate column name"), true},
		{fmt.Errorf("column already exists"), true},
		{fmt.Errorf("some other error"), false},
	}
	for _, tt := range tests {
		got := isDuplicateColumnErr(tt.err)
		if got != tt.want {
			t.Errorf("isDuplicateColumnErr(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)

	path := m.resolvePath("testdb")
	if !strings.HasSuffix(path, "/testdb") {
		t.Errorf("resolvePath('testdb') = %q, should end with /testdb", path)
	}
}

func TestDataDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)

	if m.DataDir() != dir {
		t.Errorf("DataDir() = %q, want %q", m.DataDir(), dir)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false, 1)

	names := m.List()
	if len(names) != 0 {
		t.Errorf("List() = %v, want empty", names)
	}

	m.Open("db1")
	names = m.List()
	if len(names) != 1 {
		t.Errorf("List() = %v, want [db1]", names)
	}
}

func TestSQLiteMagicHeader(t *testing.T) {
	if len(sqliteMagic) != 16 {
		t.Errorf("sqliteMagic length = %d, want 16", len(sqliteMagic))
	}
}

// Test that NewSystemDB from a file path works (not just :memory:)
func TestNewSystemDB_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system.db")
	sys, err := NewSystemDB(path)
	if err != nil {
		t.Fatalf("NewSystemDB(path): %v", err)
	}
	defer sys.Close()

	if err := sys.DB().Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestSystemDB_ListUsersEmpty(t *testing.T) {
	sys := newTestSystemDB(t)
	defer sys.Close()

	users, err := sys.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers(): %v", err)
	}
	if len(users) != 0 {
		t.Errorf("got %d users, want 0", len(users))
	}
}



package query

import (
	"testing"
	"time"

	"sparkdb/internal/rbac"
)

func TestIdentifyType(t *testing.T) {
	tests := []struct {
		query string
		want  QueryType
	}{
		{"SELECT * FROM users", TypeSelect},
		{"select id, name from users", TypeSelect},
		{"SELECT", TypeSelect},
		{"INSERT INTO users VALUES (1)", TypeInsert},
		{"insert into users", TypeInsert},
		{"UPDATE users SET name = 'x'", TypeUpdate},
		{"update users", TypeUpdate},
		{"DELETE FROM users", TypeDelete},
		{"delete from users", TypeDelete},
		{"CREATE TABLE t (id int)", TypeCreate},
		{"create table t", TypeCreate},
		{"DROP TABLE t", TypeDrop},
		{"drop table t", TypeDrop},
		{"ALTER TABLE t ADD COLUMN x", TypeAlter},
		{"alter table t", TypeAlter},
		{"PRAGMA journal_mode", TypePragma},
		{"pragma journal_mode", TypePragma},
		{"EXPLAIN SELECT * FROM t", TypeExplain},
		{"explain select * from t", TypeExplain},
		{"ATTACH DATABASE 'x' AS y", TypeOther},
		{"VACUUM", TypeOther},
		{"", TypeOther},
	}

	v := NewValidator()
	for _, tt := range tests {
		got := v.IdentifyType(tt.query)
		if got != tt.want {
			t.Errorf("IdentifyType(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestRequiredPermission(t *testing.T) {
	tests := []struct {
		qt    QueryType
		perm  rbac.Permission
		valid bool
	}{
		{TypeSelect, rbac.PermQuery, true},
		{TypePragma, rbac.PermQuery, true},
		{TypeExplain, rbac.PermQuery, true},
		{TypeInsert, rbac.PermWrite, true},
		{TypeUpdate, rbac.PermWrite, true},
		{TypeCreate, rbac.PermCreate, true},
		{TypeAlter, rbac.PermAlter, true},
		{TypeDrop, rbac.PermDrop, true},
		{TypeDelete, rbac.PermDelete, true},
		{TypeOther, "", false},
	}

	v := NewValidator()
	for _, tt := range tests {
		perm, ok := v.RequiredPermission(tt.qt)
		if ok != tt.valid {
			t.Errorf("RequiredPermission(%q) ok=%v, want %v", tt.qt, ok, tt.valid)
		}
		if ok && perm != tt.perm {
			t.Errorf("RequiredPermission(%q) = %q, want %q", tt.qt, perm, tt.perm)
		}
	}
}

func TestCheckDangerous(t *testing.T) {
	v := NewValidator()

	dangerous := []string{
		"DROP DATABASE test",
		"DROP TABLE users",
		"DELETE FROM sqlite_master",
		"attach database ':memory:' as x",
		"DETACH DATABASE x",
	}

	safe := []string{
		"SELECT * FROM users",
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name = 'test'",
		"DELETE FROM users WHERE id = 1",
		"CREATE TABLE items (id int)",
		"DROP INDEX IF EXISTS idx_name",
	}

	for _, q := range dangerous {
		blocked, _ := v.CheckDangerous(q)
		if !blocked {
			t.Errorf("CheckDangerous(%q) should be blocked", q)
		}
	}

	for _, q := range safe {
		blocked, _ := v.CheckDangerous(q)
		if blocked {
			t.Errorf("CheckDangerous(%q) should not be blocked", q)
		}
	}
}

func TestAnalyze(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		query     string
		wantType  QueryType
		wantTable string
	}{
		{"SELECT * FROM users", TypeSelect, "USERS"},
		{"INSERT INTO orders VALUES (1)", TypeInsert, "ORDERS"},
		{"UPDATE products SET price = 10", TypeUpdate, "PRODUCTS"},
		{"DELETE FROM logs WHERE id = 5", TypeDelete, "LOGS"},
		{"CREATE TABLE t (id int)", TypeCreate, ""},
	}

	for _, tt := range tests {
		m, err := v.Analyze(tt.query)
		if err != nil {
			t.Errorf("Analyze(%q) error: %v", tt.query, err)
			continue
		}
		if m.QueryType != tt.wantType {
			t.Errorf("Analyze(%q) type = %q, want %q", tt.query, m.QueryType, tt.wantType)
		}
		if m.TableName != tt.wantTable {
			t.Errorf("Analyze(%q) table = %q, want %q", tt.query, m.TableName, tt.wantTable)
		}
	}

	_, err := v.Analyze("")
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.Allow("test-key") {
			t.Errorf("attempt %d should be allowed", i+1)
		}
	}

	if rl.Allow("test-key") {
		t.Error("4th attempt should be blocked")
	}

	rl.Reset("test-key")
	if !rl.Allow("test-key") {
		t.Error("after reset, should be allowed")
	}
}

func TestRateLimiter_MultipleKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	if !rl.Allow("key-a") {
		t.Error("key-a first attempt should be allowed")
	}
	if rl.Allow("key-a") {
		t.Error("key-a second attempt should be blocked")
	}
	if !rl.Allow("key-b") {
		t.Error("key-b first attempt should be allowed (different key)")
	}
}

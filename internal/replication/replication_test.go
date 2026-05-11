package replication

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sparkdb/internal/database"
)

func TestIsWriteQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"SELECT * FROM users", false},
		{"select * from users", false},
		{"SELECT 1", false},
		{"PRAGMA journal_mode", false},
		{"EXPLAIN SELECT * FROM t", false},
		{"INSERT INTO users VALUES (1)", true},
		{"UPDATE users SET name = 'x'", true},
		{"DELETE FROM users", true},
		{"CREATE TABLE t (id int)", true},
		{"DROP TABLE t", true},
		{"ALTER TABLE t ADD COLUMN x", true},
		{"", true},
		{"   SELECT * FROM t", false},
		{"   INSERT INTO t VALUES (1)", true},
	}
	for _, tt := range tests {
		got := IsWriteQuery(tt.query)
		if got != tt.want {
			t.Errorf("IsWriteQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(nil, nil, "standalone", "", "", 10)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.Role() != "standalone" {
		t.Errorf("Role() = %q, want standalone", e.Role())
	}
	if e.PrimaryURL() != "" {
		t.Errorf("PrimaryURL() = %q, want empty", e.PrimaryURL())
	}
}

func TestEnginePrimaryURLTrimmed(t *testing.T) {
	e := NewEngine(nil, nil, "replica", "http://primary:9600/", "key", 10)
	if e.PrimaryURL() != "http://primary:9600" {
		t.Errorf("PrimaryURL() = %q, want http://primary:9600", e.PrimaryURL())
	}
}

func TestEngineStartStandalone(t *testing.T) {
	e := NewEngine(nil, nil, "standalone", "", "", 5)
	e.Start()
	e.Stop()
}

func TestEngineStartPrimary(t *testing.T) {
	e := NewEngine(nil, nil, "primary", "", "", 5)
	e.Start()
	e.Stop()
}

func TestEngineStartReplica(t *testing.T) {
	sys, err := database.NewSystemDB(":memory:")
	if err != nil {
		t.Fatalf("NewSystemDB(): %v", err)
	}
	defer sys.Close()

	dir := t.TempDir()
	mgr := database.NewManager(dir, false, 1)
	exec := database.NewExecutor(mgr)

	e := NewEngine(sys, exec, "replica", "http://localhost:1", "key", 1)
	e.Start()
	e.Stop()
}

func TestEngineNilExecutor(t *testing.T) {
	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, nil, "replica", "http://localhost:1", "key", 1)
	e.Start()
	e.Stop()
}

func TestEngineRoleAccessors(t *testing.T) {
	e := NewEngine(nil, nil, "primary", "http://p:9600", "key", 5)
	if e.Role() != "primary" {
		t.Errorf("Role() = %q, want primary", e.Role())
	}
	if e.PrimaryURL() != "http://p:9600" {
		t.Errorf("PrimaryURL() = %q, want http://p:9600", e.PrimaryURL())
	}
}

func TestApplyEntry(t *testing.T) {
	dir := t.TempDir()
	mgr := database.NewManager(dir, false, 1)
	exec := database.NewExecutor(mgr)

	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, exec, "standalone", "", "", 5)

	_, err := exec.Execute("applydb", "CREATE TABLE t (id INT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	entry := &database.ReplicationEntry{
		ID:           1,
		DatabaseName: "applydb",
		Query:        "INSERT INTO t VALUES (42)",
	}

	if err := e.applyEntry(entry); err != nil {
		t.Fatalf("applyEntry() error: %v", err)
	}

	res, _ := exec.Execute("applydb", "SELECT id FROM t")
	if len(res.Rows) != 1 || res.Rows[0][0] != int64(42) {
		t.Errorf("expected row with 42, got %v", res.Rows)
	}
}

func TestPollOnce_WithPrimary(t *testing.T) {
	primarySys, _ := database.NewSystemDB(":memory:")
	defer primarySys.Close()

	primarySys.InitReplicationState("primary", "")
	primarySys.LogReplication("testdb", "INSERT INTO t VALUES (1)")

	primaryHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/replication/log" {
			entries, _ := primarySys.GetReplicationLog(0, 500)
			anyEntries := make([]interface{}, len(entries))
			for i, e := range entries {
				anyEntries[i] = e
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"entries": anyEntries,
			})
		}
	})
	primaryServer := httptest.NewServer(primaryHandler)
	defer primaryServer.Close()

	dir := t.TempDir()
	mgr := database.NewManager(dir, false, 1)
	exec := database.NewExecutor(mgr)

	_, err := exec.Execute("testdb", "CREATE TABLE t (id INT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	replicaSys, _ := database.NewSystemDB(":memory:")
	defer replicaSys.Close()

	e := NewEngine(replicaSys, exec, "replica", primaryServer.URL, "key", 1)

	lastID := int64(0)
	e.pollOnce(&lastID)

	if lastID <= 0 {
		t.Errorf("lastID = %d, want > 0", lastID)
	}

	res, _ := exec.Execute("testdb", "SELECT id FROM t")
	if len(res.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(res.Rows))
	}
}

func TestPollOnce_EmptyResponse(t *testing.T) {
	primaryHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": []interface{}{},
		})
	})
	primaryServer := httptest.NewServer(primaryHandler)
	defer primaryServer.Close()

	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, nil, "replica", primaryServer.URL, "key", 1)
	lastID := int64(0)
	e.pollOnce(&lastID)

	if lastID != 0 {
		t.Errorf("lastID = %d, want 0 (no entries processed)", lastID)
	}
}

func TestPollOnce_NonOKStatus(t *testing.T) {
	primaryHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	primaryServer := httptest.NewServer(primaryHandler)
	defer primaryServer.Close()

	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, nil, "replica", primaryServer.URL, "key", 1)
	lastID := int64(0)
	e.pollOnce(&lastID)
}

func TestPollOnce_InvalidJSON(t *testing.T) {
	primaryHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	})
	primaryServer := httptest.NewServer(primaryHandler)
	defer primaryServer.Close()

	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, nil, "replica", primaryServer.URL, "key", 1)

	lastID := int64(0)
	e.pollOnce(&lastID)
}

func TestPollOnce_ConnectError(t *testing.T) {
	sys, _ := database.NewSystemDB(":memory:")
	defer sys.Close()

	e := NewEngine(sys, nil, "replica", "http://localhost:1", "key", 1)

	lastID := int64(0)
	e.pollOnce(&lastID)
}

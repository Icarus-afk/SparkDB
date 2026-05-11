package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mockDBProvider struct {
	names   []string
	dataDir string
}

func (m *mockDBProvider) List() []string {
	return m.names
}

func (m *mockDBProvider) ListAll() []string {
	return m.names
}

func (m *mockDBProvider) DataDir() string {
	return m.dataDir
}

type multiDBProvider struct {
	names   []string
	dataDir string
}

func (m *multiDBProvider) List() []string {
	return m.names
}

func (m *multiDBProvider) ListAll() []string {
	return m.names
}

func (m *multiDBProvider) DataDir() string {
	return m.dataDir
}

func TestNewMonitor(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)
	if m == nil {
		t.Fatal("New() returned nil")
	}

	s := m.Stats()
	if s.UptimeSeconds < 0 {
		t.Error("uptime should be >= 0")
	}
}

func TestRecordQuery(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	m.RecordQuery(10 * time.Millisecond)
	m.RecordQuery(20 * time.Millisecond)
	m.RecordQuery(30 * time.Millisecond)

	s := m.Stats()
	if s.TotalQueries != 3 {
		t.Errorf("TotalQueries = %d, want 3", s.TotalQueries)
	}
	if s.AvgLatencyMs <= 0 {
		t.Error("AvgLatencyMs should be > 0")
	}
}

func TestRecordFailedLogin(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	m.RecordFailedLogin()
	m.RecordFailedLogin()

	s := m.Stats()
	if s.FailedLogins != 2 {
		t.Errorf("FailedLogins = %d, want 2", s.FailedLogins)
	}
}

func TestStatsPopulated(t *testing.T) {
	p := &mockDBProvider{names: []string{"main"}}
	m := New(p)
	m.RecordQuery(5 * time.Millisecond)

	s := m.Stats()
	if s.Goroutines <= 0 {
		t.Error("Goroutines should be > 0")
	}
	if s.AllocMB <= 0 {
		t.Error("AllocMB should be > 0")
	}
	if s.NumDatabases != 1 {
		t.Errorf("NumDatabases = %d, want 1", s.NumDatabases)
	}
}

func TestP99Latency(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	for i := 0; i < 100; i++ {
		m.RecordQuery(time.Duration(i) * time.Millisecond)
	}

	s := m.Stats()
	if s.P99LatencyMs <= 0 {
		t.Error("P99LatencyMs should be > 0")
	}
	if s.P99LatencyMs >= 200 {
		t.Errorf("P99LatencyMs = %f, expected < 200", s.P99LatencyMs)
	}
}

func TestLatencyCap(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	for i := 0; i < 15000; i++ {
		m.RecordQuery(time.Duration(i) * time.Microsecond)
	}

	s := m.Stats()
	if s.TotalQueries != 15000 {
		t.Errorf("TotalQueries = %d, want 15000", s.TotalQueries)
	}
}

func TestDatabaseStats(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "testdb")
	os.WriteFile(dbPath, []byte("some content"), 0644)
	walPath := dbPath + "-wal"
	os.WriteFile(walPath, []byte("wal content"), 0644)

	p := &mockDBProvider{names: []string{"testdb"}, dataDir: dir}
	m := New(p)

	s := m.Stats()
	if s.NumDatabases != 1 {
		t.Errorf("NumDatabases = %d, want 1", s.NumDatabases)
	}
	if len(s.Databases) != 1 {
		t.Errorf("Databases = %d, want 1", len(s.Databases))
	}
	if s.Databases[0].Name != "testdb" {
		t.Errorf("Database[0].Name = %q, want %q", s.Databases[0].Name, "testdb")
	}
	if s.Databases[0].Size <= 0 {
		t.Error("Database[0].Size should be > 0")
	}
	if s.ActiveConns != 1 {
		t.Errorf("ActiveConns = %d, want 1", s.ActiveConns)
	}
}

func TestDatabaseStats_MissingFile(t *testing.T) {
	p := &mockDBProvider{names: []string{}, dataDir: t.TempDir()}
	m := New(p)

	s := m.Stats()
	if s.NumDatabases != 0 {
		t.Errorf("NumDatabases = %d, want 0", s.NumDatabases)
	}
}

func TestEmptyLatency(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	s := m.Stats()
	if s.TotalQueries != 0 {
		t.Errorf("TotalQueries = %d, want 0", s.TotalQueries)
	}
	if s.AvgLatencyMs != 0 {
		t.Errorf("AvgLatencyMs = %f, want 0", s.AvgLatencyMs)
	}
	if s.P99LatencyMs != 0 {
		t.Errorf("P99LatencyMs = %f, want 0", s.P99LatencyMs)
	}
}

func TestSingleLatency(t *testing.T) {
	p := &mockDBProvider{}
	m := New(p)

	m.RecordQuery(10 * time.Millisecond)
	s := m.Stats()
	if s.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", s.TotalQueries)
	}
	if s.AvgLatencyMs <= 0 {
		t.Error("AvgLatencyMs should be > 0")
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() with defaults error: %v", err)
	}
	if cfg.Server.Port != 9600 {
		t.Errorf("Port = %d, want 9600", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Database.MaxConns != 100 {
		t.Errorf("MaxConns = %d, want 100", cfg.Database.MaxConns)
	}
	if cfg.Backup.KeepCount != 10 {
		t.Errorf("KeepCount = %d, want 10", cfg.Backup.KeepCount)
	}
	if cfg.Replication.Role != "standalone" {
		t.Errorf("Role = %q, want standalone", cfg.Replication.Role)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{
		"server": { "host": "127.0.0.1", "port": 7777 },
		"database": { "max_connections": 50 },
		"backup": { "keep_count": 5 }
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("Port = %d, want 7777", cfg.Server.Port)
	}
	if cfg.Database.MaxConns != 50 {
		t.Errorf("MaxConns = %d, want 50", cfg.Database.MaxConns)
	}
	if cfg.Backup.KeepCount != 5 {
		t.Errorf("KeepCount = %d, want 5", cfg.Backup.KeepCount)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)
	oldVal := os.Getenv("SPARKDB_SERVER_PORT")
	os.Setenv("SPARKDB_SERVER_PORT", "5555")
	defer os.Setenv("SPARKDB_SERVER_PORT", oldVal)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Server.Port != 5555 {
		t.Errorf("Port = %d, want 5555", cfg.Server.Port)
	}
}

func TestLoadEnvOverrideDataDir(t *testing.T) {
	oldVal := os.Getenv("SPARKDB_DATABASE_DATA_DIR")
	os.Setenv("SPARKDB_DATABASE_DATA_DIR", "/custom/data")
	defer os.Setenv("SPARKDB_DATABASE_DATA_DIR", oldVal)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Database.DataDir != "/custom/data" {
		t.Errorf("DataDir = %q, want /custom/data", cfg.Database.DataDir)
	}
}

func TestLoadPortValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"server": { "port": 0 }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestLoadPortTooHigh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"server": { "port": 99999 }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for port > 65535")
	}
}

func TestLoadMinMaxConns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"database": { "max_connections": 0 }}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Database.MaxConns != 1 {
		t.Errorf("MaxConns = %d, want 1 (minimum)", cfg.Database.MaxConns)
	}
}

func TestLoadInvalidReplicationRole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"replication": { "role": "invalid" }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid replication role")
	}
}

func TestLoadReplicaRequiresPrimaryURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"replication": { "role": "replica" }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: replica requires primary_url")
	}
}

func TestLoadReplicaRequiresAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"replication": { "role": "replica", "primary_url": "http://primary:9600" }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: replica requires api_key")
	}
}

func TestLoadTLSNoAutoCert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"tls": { "enabled": true, "auto_cert": false, "cert_file": "", "key_file": "" }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: tls without auto_cert requires cert_file and key_file")
	}
}

func TestLoadTLSNoAutoCertWithFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"tls": { "enabled": true, "auto_cert": false, "cert_file": "/certs/cert.pem", "key_file": "/certs/key.pem" }}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.TLS.Enabled {
		t.Error("TLS should be enabled")
	}
	if cfg.TLS.CertFile != "/certs/cert.pem" {
		t.Errorf("CertFile = %q, want /certs/cert.pem", cfg.TLS.CertFile)
	}
}

func TestLoadReplicationRoleEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"replication": { "role": "" }}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Replication.Role != "standalone" {
		t.Errorf("Role = %q, want standalone", cfg.Replication.Role)
	}
}

func TestLoadPollIntervalMin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"replication": { "poll_interval": 0 }}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Replication.PollInterval < 1 {
		t.Errorf("PollInterval = %d, want >= 1", cfg.Replication.PollInterval)
	}
}

func TestLoadEncryptionNoKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"encryption": { "enabled": true }}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: encryption enabled but no key")
	}
}

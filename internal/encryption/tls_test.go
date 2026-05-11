package encryption

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	if err := GenerateSelfSignedCert(certPath, keyPath); err != nil {
		t.Fatalf("GenerateSelfSignedCert() error: %v", err)
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("cert file was not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key file was not created")
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(certData) == 0 {
		t.Fatal("cert file is empty")
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(keyData) == 0 {
		t.Fatal("key file is empty")
	}
}

func TestTLSServerConfig(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	if err := GenerateSelfSignedCert(certPath, keyPath); err != nil {
		t.Fatalf("GenerateSelfSignedCert() error: %v", err)
	}

	cfg, err := TLSServerConfig(certPath, keyPath)
	if err != nil {
		t.Fatalf("TLSServerConfig() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("TLSServerConfig returned nil")
	}
	if len(cfg.Certificates) != 1 {
		t.Errorf("got %d certificates, want 1", len(cfg.Certificates))
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestTLSServerConfig_InvalidFiles(t *testing.T) {
	_, err := TLSServerConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent cert files")
	}
}

func TestEnsureCertFiles_BothMissing(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	if err := EnsureCertFiles(certPath, keyPath); err != nil {
		t.Fatalf("EnsureCertFiles() error: %v", err)
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("cert should have been created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key should have been created")
	}

	if err := EnsureCertFiles(certPath, keyPath); err != nil {
		t.Fatal("EnsureCertFiles() should be idempotent")
	}
}

func TestEnsureCertFiles_CertOnlyExists(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	os.WriteFile(certPath, []byte("not-a-real-cert"), 0644)

	if err := EnsureCertFiles(certPath, keyPath); err != nil {
		t.Fatalf("EnsureCertFiles() error: %v", err)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key should have been created")
	}
}

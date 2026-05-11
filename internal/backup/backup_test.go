package backup

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"sparkdb/internal/database"
	"sparkdb/internal/encryption"
)

func newTestBackupManager(t *testing.T) (*Manager, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	backupDir := t.TempDir()
	dbManager := database.NewManager(dataDir, false, 1)

	db, err := dbManager.Open("testdb")
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO items VALUES (1, 'test')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	mgr := NewManager(backupDir, dataDir, dbManager, nil)
	return mgr, dataDir, backupDir
}

func newTestEncryptedBackupManager(t *testing.T) (*Manager, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	backupDir := t.TempDir()

	key := make([]byte, 32)
	rand.Read(key)
	c, err := encryption.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher(): %v", err)
	}

	dbManager := database.NewEncryptedManager(dataDir, false, 1, c)

	db, err := dbManager.Open("encdb")
	if err != nil {
		t.Fatalf("create encrypted db: %v", err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO items VALUES (1, 'encrypted-data')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	mgr := NewManager(backupDir, dataDir, dbManager, c)
	return mgr, dataDir, backupDir
}

func TestNewManager(t *testing.T) {
	mgr, _, _ := newTestBackupManager(t)
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestCreateAndListBackup(t *testing.T) {
	mgr, _, backupDir := newTestBackupManager(t)

	info, err := mgr.CreateBackup("testdb")
	if err != nil {
		t.Fatalf("CreateBackup() error: %v", err)
	}
	if info.Name == "" {
		t.Fatal("backup name should not be empty")
	}
	if info.Size <= 0 {
		t.Errorf("backup size = %d, want > 0", info.Size)
	}
	if info.Database != "testdb" {
		t.Errorf("Database = %q, want %q", info.Database, "testdb")
	}

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("got %d backups, want 1", len(backups))
	}
	if backups[0].Name != info.Name {
		t.Errorf("backup name mismatch: %q vs %q", backups[0].Name, info.Name)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("backup dir has %d entries, want 1", len(entries))
	}
}

func TestDeleteBackup(t *testing.T) {
	mgr, _, backupDir := newTestBackupManager(t)

	info, _ := mgr.CreateBackup("testdb")

	if err := mgr.DeleteBackup(info.Path); err != nil {
		t.Fatalf("DeleteBackup() error: %v", err)
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 0 {
		t.Errorf("expected empty backup dir, got %d entries", len(entries))
	}
}

func TestDeleteBackup_NonExistent(t *testing.T) {
	mgr, _, _ := newTestBackupManager(t)

	err := mgr.DeleteBackup("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestListBackups_Empty(t *testing.T) {
	dataDir := t.TempDir()
	backupDir := t.TempDir()
	dbManager := database.NewManager(dataDir, false, 1)
	mgr := NewManager(backupDir, dataDir, dbManager, nil)

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("got %d backups, want 0", len(backups))
	}
}

func TestRestoreBackup(t *testing.T) {
	mgr, dataDir, _ := newTestBackupManager(t)

	info, _ := mgr.CreateBackup("testdb")
	mgr.dbManager.Close("testdb")
	os.Remove(filepath.Join(dataDir, "testdb"))

	if err := mgr.RestoreBackup(info.Path, "testdb"); err != nil {
		t.Fatalf("RestoreBackup() error: %v", err)
	}

	db, err := mgr.dbManager.Open("testdb")
	if err != nil {
		t.Fatalf("reopen after restore: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("query after restore: %v", err)
	}
	if count != 1 {
		t.Errorf("items count = %d, want 1", count)
	}
}

func TestRestoreBackup_NotFound(t *testing.T) {
	mgr, _, _ := newTestBackupManager(t)

	err := mgr.RestoreBackup("/nonexistent/path.backup", "testdb")
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestRestoreBackupWithRelativePath(t *testing.T) {
	mgr, _, backupDir := newTestBackupManager(t)

	info, _ := mgr.CreateBackup("testdb")
	mgr.dbManager.Close("testdb")
	os.RemoveAll(mgr.dataDir + "/testdb")

	relPath := filepath.Base(info.Path)
	oldWd, _ := os.Getwd()
	os.Chdir(backupDir)
	err := mgr.RestoreBackup(relPath, "testdb")
	os.Chdir(oldWd)
	if err != nil {
		t.Fatalf("RestoreBackup() with relative path error: %v", err)
	}
}

func TestRestoreBackup_ResolvesInBackupDir(t *testing.T) {
	mgr, _, _ := newTestBackupManager(t)

	err := mgr.RestoreBackup("nonexistent.backup", "testdb")
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestRestoreBackup_IsDir(t *testing.T) {
	mgr, _, backupDir := newTestBackupManager(t)

	err := mgr.RestoreBackup(backupDir, "testdb")
	if err == nil {
		t.Fatal("expected error when path is directory")
	}
}

func TestListBackupsWithNonBackupFiles(t *testing.T) {
	mgr, _, backupDir := newTestBackupManager(t)

	os.WriteFile(filepath.Join(backupDir, "readme.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(backupDir, "subdir"), 0755)

	info, _ := mgr.CreateBackup("testdb")

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("got %d backups, want 1", len(backups))
	}
	if backups[0].Name != info.Name {
		t.Errorf("Name = %q, want %q", backups[0].Name, info.Name)
	}
}

func TestBackupCreatesNewDB(t *testing.T) {
	dataDir := t.TempDir()
	backupDir := t.TempDir()
	dbManager := database.NewManager(dataDir, false, 1)
	mgr := NewManager(backupDir, dataDir, dbManager, nil)

	_, err := mgr.CreateBackup("newlycreated")
	if err != nil {
		t.Fatalf("CreateBackup() should create database on demand, got error: %v", err)
	}

	backups, _ := mgr.ListBackups()
	if len(backups) != 1 {
		t.Errorf("got %d backups, want 1", len(backups))
	}
}



func TestCreateBackupWithCipher_VerifyFile(t *testing.T) {
	mgr, _, backupDir := newTestEncryptedBackupManager(t)

	info, err := mgr.CreateBackup("encdb")
	if err != nil {
		t.Fatalf("CreateBackup() with cipher error: %v", err)
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("backup file is empty")
	}

	dirEntries, _ := os.ReadDir(backupDir)
	if len(dirEntries) != 1 {
		t.Errorf("expected 1 entry in backup dir, got %d", len(dirEntries))
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	read, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(read) != string(content) {
		t.Errorf("copied content = %q, want %q", read, content)
	}
}

func TestCopyFile_NonexistentSrc(t *testing.T) {
	err := copyFile("/nonexistent", "/tmp/dst")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

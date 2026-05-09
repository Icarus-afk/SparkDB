package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sparkdb/internal/database"
	"sparkdb/internal/encryption"
)

type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Database  string    `json:"database"`
}

type Manager struct {
	backupDir string
	dataDir   string
	dbManager *database.Manager
	cipher    *encryption.Cipher
}

func NewManager(backupDir, dataDir string, dbManager *database.Manager, cipher *encryption.Cipher) *Manager {
	return &Manager{
		backupDir: backupDir,
		dataDir:   dataDir,
		dbManager: dbManager,
		cipher:    cipher,
	}
}

func (bm *Manager) CreateBackup(dbName string) (*BackupInfo, error) {
	if err := os.MkdirAll(bm.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.db.backup", dbName, timestamp)
	backupPath := filepath.Join(bm.backupDir, filename)

	db, err := bm.dbManager.Open(dbName)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	srcPath := bm.dataDir + "/" + dbName
	if bm.cipher != nil {
		decPath := srcPath + ".dec"
		if _, err := os.Stat(decPath); err == nil {
			srcPath = decPath
		}
	}

	if err := copyFile(srcPath, backupPath); err != nil {
		return nil, fmt.Errorf("copy backup: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("stat backup: %w", err)
	}

	return &BackupInfo{
		Name:      filename,
		Path:      backupPath,
		Size:      info.Size(),
		CreatedAt: info.ModTime(),
		Database:  dbName,
	}, nil
}

func (bm *Manager) RestoreBackup(backupFile, dbName string) error {
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		resolved := filepath.Join(bm.backupDir, backupFile)
		if _, err2 := os.Stat(resolved); err2 == nil {
			backupFile = resolved
		}
	}
	info, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("backup path is a directory")
	}

	dbPath := bm.dataDir + "/" + dbName

	bm.dbManager.Close(dbName)

	if err := copyFile(backupFile, dbPath); err != nil {
		return fmt.Errorf("restore file: %w", err)
	}

	if _, err := bm.dbManager.Open(dbName); err != nil {
		return fmt.Errorf("reopen after restore: %w", err)
	}

	return nil
}

func (bm *Manager) ListBackups() ([]*BackupInfo, error) {
	if err := os.MkdirAll(bm.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("ensure backup dir: %w", err)
	}

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var backups []*BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".backup") {
			continue
		}

		fi, err := entry.Info()
		if err != nil {
			continue
		}

		dbName := "unknown"
		if parts := strings.SplitN(entry.Name(), "_", 2); len(parts) > 0 {
			dbName = parts[0]
		}

		backups = append(backups, &BackupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(bm.backupDir, entry.Name()),
			Size:      fi.Size(),
			CreatedAt: fi.ModTime(),
			Database:  dbName,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

func (bm *Manager) DeleteBackup(backupFile string) error {
	return os.Remove(backupFile)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return dstFile.Sync()
}

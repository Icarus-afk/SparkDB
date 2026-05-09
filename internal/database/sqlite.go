package database

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	_ "modernc.org/sqlite"

	"sparkdb/internal/encryption"
)

type Manager struct {
	mu       sync.RWMutex
	dbs      map[string]*sql.DB
	dataDir  string
	walMode  bool
	maxConns int
	cipher   *encryption.Cipher
}

func NewManager(dataDir string, walMode bool, maxConns int) *Manager {
	return &Manager{
		dbs:      make(map[string]*sql.DB),
		dataDir:  dataDir,
		walMode:  walMode,
		maxConns: maxConns,
	}
}

func NewEncryptedManager(dataDir string, walMode bool, maxConns int, c *encryption.Cipher) *Manager {
	return &Manager{
		dbs:      make(map[string]*sql.DB),
		dataDir:  dataDir,
		walMode:  false,
		maxConns: maxConns,
		cipher:   c,
	}
}

func (m *Manager) Open(name string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if db, ok := m.dbs[name]; ok {
		return db, nil
	}

	path := m.resolvePath(name)
	actualPath := path

	if m.cipher != nil {
		if m.isEncrypted(path) {
			decPath := path + ".dec"
			if err := m.cipher.DecryptCopy(path, decPath); err != nil {
				return nil, fmt.Errorf("decrypt database %s: %w", name, err)
			}
			actualPath = decPath
		}
	}

	db, err := sql.Open("sqlite", actualPath)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", name, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if m.walMode {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("enable WAL: %w", err)
		}
	}

	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set synchronous mode: %w", err)
	}

	m.dbs[name] = db
	return db, nil
}

func (m *Manager) Close(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db, ok := m.dbs[name]
	if !ok {
		return nil
	}

	if err := db.Close(); err != nil {
		return err
	}
	delete(m.dbs, name)

	m.reencrypt(name)
	return nil
}

func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, db := range m.dbs {
		if err := db.Close(); err != nil {
			lastErr = err
		}
		delete(m.dbs, name)
		m.reencrypt(name)
	}
	return lastErr
}

func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.dbs))
	for name := range m.dbs {
		names = append(names, name)
	}
	return names
}

func (m *Manager) ListAll() []string {
	seen := make(map[string]bool)
	for name := range m.dbs {
		seen[name] = true
	}
	entries, err := os.ReadDir(m.dataDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "sparkdb_system.db" || name == "sparkdb_system.db-wal" || name == "sparkdb_system.db-shm" {
				continue
			}
			if len(name) > 4 && name[len(name)-4:] == ".dec" {
				continue
			}
			if len(name) > 4 && (name[len(name)-4:] == "-wal" || name[len(name)-4:] == "-shm") {
				continue
			}
			if len(name) > 5 && name[len(name)-5:] == "-journal" {
				continue
			}
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return names
}

func (m *Manager) resolvePath(name string) string {
	return m.dataDir + "/" + name
}

func (m *Manager) isEncrypted(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func (m *Manager) reencrypt(name string) {
	if m.cipher == nil {
		return
	}

	origPath := m.resolvePath(name)
	decPath := origPath + ".dec"

	if _, err := os.Stat(decPath); err == nil {
		if err := m.cipher.EncryptCopy(decPath, origPath); err != nil {
			fmt.Fprintf(os.Stderr, "error encrypting database %s: %v\n", name, err)
		}
		os.Remove(decPath)
		return
	}

	if info, err := os.Stat(origPath); err == nil && info.Size() > 0 {
		if err := m.cipher.EncryptFile(origPath); err != nil {
			fmt.Fprintf(os.Stderr, "error encrypting database %s: %v\n", name, err)
		}
	}
}

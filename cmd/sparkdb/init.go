package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type initConfig struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Database struct {
		DataDir  string `json:"data_dir"`
		WALMode  bool   `json:"wal_mode"`
		MaxConns int    `json:"max_connections"`
	} `json:"database"`
	Auth struct {
		JWTSecret string `json:"jwt_secret"`
	} `json:"auth"`
	TLS struct {
		Enabled  bool   `json:"enabled"`
		AutoCert bool   `json:"auto_cert"`
		CertFile string `json:"cert_file"`
		KeyFile  string `json:"key_file"`
	} `json:"tls"`
	Encryption struct {
		Enabled bool   `json:"enabled"`
		Key     string `json:"key,omitempty"`
	} `json:"encryption"`
	Backup struct {
		Dir       string `json:"dir"`
		Schedule  string `json:"schedule"`
		KeepCount int    `json:"keep_count"`
	} `json:"backup"`
	Replication struct {
		Role         string `json:"role"`
		PrimaryURL   string `json:"primary_url,omitempty"`
		APIKey       string `json:"api_key,omitempty"`
		PollInterval int    `json:"poll_interval"`
	} `json:"replication"`
}

func init() {
	var initDir string
	var initPort int
	var initGenCert bool
	var initGenKey bool
	var initDataDir string
	var initBackupDir string

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a SparkDB project (config, secrets, directories)",
		Long: `Initialize a new SparkDB project in the specified directory.

Creates:
  - config.json     with sensible defaults and a generated JWT secret
  - data/           directory for database files
  - backups/        directory for backup files

Optionally generates TLS certificates and encryption keys.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if initDir == "" {
				initDir = "."
			}

			absDir, err := filepath.Abs(initDir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			if err := os.MkdirAll(absDir, 0755); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}

			dataDir := initDataDir
			if dataDir == "" {
				dataDir = filepath.Join(absDir, "data")
			}
			backupDir := initBackupDir
			if backupDir == "" {
				backupDir = filepath.Join(absDir, "backups")
			}

			for _, d := range []string{dataDir, backupDir} {
				if err := os.MkdirAll(d, 0755); err != nil {
					return fmt.Errorf("create directory %s: %w", d, err)
				}
			}

			jwtSecret := make([]byte, 32)
			if _, err := rand.Read(jwtSecret); err != nil {
				return fmt.Errorf("generate jwt secret: %w", err)
			}

			cfg := initConfig{}
			cfg.Server.Host = "0.0.0.0"
			cfg.Server.Port = initPort
			cfg.Database.DataDir = dataDir
			cfg.Database.WALMode = true
			cfg.Database.MaxConns = 100
			cfg.Auth.JWTSecret = hex.EncodeToString(jwtSecret)
			cfg.TLS.Enabled = initGenCert
			cfg.TLS.AutoCert = initGenCert
			cfg.TLS.CertFile = filepath.Join(absDir, "sparkdb.crt")
			cfg.TLS.KeyFile = filepath.Join(absDir, "sparkdb.key")
			cfg.Encryption.Enabled = initGenKey
			cfg.Backup.Dir = backupDir
			cfg.Backup.KeepCount = 10
			cfg.Replication.Role = "standalone"
			cfg.Replication.PollInterval = 5

			if initGenKey {
				key := make([]byte, 32)
				if _, err := rand.Read(key); err != nil {
					return fmt.Errorf("generate encryption key: %w", err)
				}
				cfg.Encryption.Key = hex.EncodeToString(key)
			}

			configPath := filepath.Join(absDir, "config.json")
			f, err := os.Create(configPath)
			if err != nil {
				return fmt.Errorf("create config.json: %w", err)
			}
			defer f.Close()

			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(cfg); err != nil {
				return fmt.Errorf("write config.json: %w", err)
			}

			fmt.Printf("SparkDB project initialized in %s\n\n", absDir)
			fmt.Println("Generated files:")
			fmt.Printf("  config.json   - server configuration\n")
			fmt.Printf("  %s/   - database storage\n", filepath.Base(dataDir))
			fmt.Printf("  %s/ - backup storage\n", filepath.Base(backupDir))
			if initGenCert {
				fmt.Println("  (TLS cert and key will be auto-generated on first start)")
			}
			fmt.Println()

			fmt.Println("Next steps:")
			fmt.Printf("  1. Review config.json and adjust settings as needed\n")
			fmt.Printf("  2. Start the server:  sparkdb start -c %s\n", configPath)
			fmt.Println("  3. Log in with the default admin/admin credentials")
			fmt.Println("  4. Change the admin password immediately")
			fmt.Println()

			if initGenCert {
				fmt.Println("TLS is enabled with auto_cert. A self-signed certificate will be")
				fmt.Println("generated on first server start. For production, replace with a")
				fmt.Println("CA-signed certificate.")
			}
			if initGenKey {
				fmt.Println("Database encryption is enabled. The encryption key is in config.json.")
				fmt.Println("For production, use SPARKDB_ENCRYPTION_KEY env var instead.")
			}
			fmt.Println()
			fmt.Println("Quick start:")
			fmt.Printf("  sparkdb start -c %s\n", configPath)
			fmt.Printf("  curl -X POST http://localhost:%d/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin\"}'\n", initPort)

			return nil
		},
	}

	initCmd.Flags().StringVar(&initDir, "dir", "", "project directory (default: current directory)")
	initCmd.Flags().IntVar(&initPort, "port", 9600, "server port")
	initCmd.Flags().BoolVar(&initGenCert, "gen-cert", false, "generate a self-signed TLS certificate")
	initCmd.Flags().BoolVar(&initGenKey, "gen-key", false, "generate an encryption key and enable encryption")
	initCmd.Flags().StringVar(&initDataDir, "data-dir", "", "database data directory (default: <dir>/data)")
	initCmd.Flags().StringVar(&initBackupDir, "backup-dir", "", "backup directory (default: <dir>/backups)")
	rootCmd.AddCommand(initCmd)
}

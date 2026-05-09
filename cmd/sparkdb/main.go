package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"nanodb/internal/auth"
	"nanodb/internal/backup"
	"nanodb/internal/config"
	"nanodb/internal/database"
	"nanodb/internal/encryption"
	"nanodb/internal/server"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "sparkdb",
	Short: "SparkDB - lightweight SQLite-powered database server",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "path to config file")
}

func main() {

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the SparkDB database server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			srv, err := server.New(cfg)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}

			return srv.Start()
		},
	}
	rootCmd.AddCommand(startCmd)

	createDBCmd := &cobra.Command{
		Use:   "create-db [name]",
		Short: "Create a new database file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var mgr *database.Manager
			if cfg.Encryption.Enabled {
				ciph, err := getCipher(cfg)
				if err != nil {
					return err
				}
				mgr = database.NewEncryptedManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns, ciph)
			} else {
				mgr = database.NewManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns)
			}

			_, err = mgr.Open(args[0])
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}
			mgr.Close(args[0])
			fmt.Printf("database '%s' created\n", args[0])
			return nil
		},
	}
	rootCmd.AddCommand(createDBCmd)

	createUserCmd := &cobra.Command{
		Use:   "create-user [username] [password] [role]",
		Short: "Create a new database user (admin, developer, readonly, auditor)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			password := args[1]
			role := args[2]

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			systemDB, err := database.NewSystemDB(cfg.Database.DataDir + "/sparkdb_system.db")
			if err != nil {
				return fmt.Errorf("open system database: %w", err)
			}
			defer systemDB.Close()

			authenticator := auth.NewAuthenticator(auth.AuthenticatorConfig{
				SystemDB: systemDB,
			})

			created, err := authenticator.CreateUser(username, password, role)
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}

			fmt.Printf("user '%s' created with role '%s' (id=%d)\n", created.Username, created.Role, created.ID)
			return nil
		},
	}
	rootCmd.AddCommand(createUserCmd)

	genKeyCmd := &cobra.Command{
		Use:   "gen-key",
		Short: "Generate a new encryption key (hex-encoded, 32 bytes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := encryption.GenerateKey()
			if err != nil {
				return fmt.Errorf("generate key: %w", err)
			}
			fmt.Printf("Encryption key: %x\n", key)
			fmt.Println("Store this key securely! Set it as SPARKDB_ENCRYPTION_KEY or in config encryption.key")
			return nil
		},
	}
	rootCmd.AddCommand(genKeyCmd)

	genCertCmd := &cobra.Command{
		Use:   "gen-cert",
		Short: "Generate a self-signed TLS certificate",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			certFile, _ := cmd.Flags().GetString("cert")
			keyFile, _ := cmd.Flags().GetString("key")
			if err := encryption.GenerateSelfSignedCert(certFile, keyFile); err != nil {
				return fmt.Errorf("generate cert: %w", err)
			}
			fmt.Printf("TLS certificate generated: %s, %s\n", certFile, keyFile)
			return nil
		},
	}
	genCertCmd.Flags().String("cert", "sparkdb.crt", "certificate file path")
	genCertCmd.Flags().String("key", "sparkdb.key", "key file path")
	rootCmd.AddCommand(genCertCmd)

	encryptCmd := &cobra.Command{
		Use:   "encrypt [file]",
		Short: "Encrypt a database file with AES-256-GCM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyHex, _ := cmd.Flags().GetString("key")
			if keyHex == "" {
				keyHex = os.Getenv("SPARKDB_ENCRYPTION_KEY")
			}
			if keyHex == "" {
				return fmt.Errorf("encryption key required (--key or SPARKDB_ENCRYPTION_KEY)")
			}

			ciph, err := encryption.NewCipherFromHex(keyHex)
			if err != nil {
				return fmt.Errorf("init cipher: %w", err)
			}

			if err := ciph.EncryptFile(args[0]); err != nil {
				return fmt.Errorf("encrypt: %w", err)
			}
			fmt.Printf("encrypted: %s\n", args[0])
			return nil
		},
	}
	encryptCmd.Flags().String("key", "", "encryption key (hex)")
	rootCmd.AddCommand(encryptCmd)

	decryptCmd := &cobra.Command{
		Use:   "decrypt [file]",
		Short: "Decrypt a database file with AES-256-GCM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyHex, _ := cmd.Flags().GetString("key")
			if keyHex == "" {
				keyHex = os.Getenv("SPARKDB_ENCRYPTION_KEY")
			}
			if keyHex == "" {
				return fmt.Errorf("encryption key required (--key or SPARKDB_ENCRYPTION_KEY)")
			}

			ciph, err := encryption.NewCipherFromHex(keyHex)
			if err != nil {
				return fmt.Errorf("init cipher: %w", err)
			}

			if err := ciph.DecryptFile(args[0]); err != nil {
				return fmt.Errorf("decrypt: %w", err)
			}
			fmt.Printf("decrypted: %s\n", args[0])
			return nil
		},
	}
	decryptCmd.Flags().String("key", "", "encryption key (hex)")
	rootCmd.AddCommand(decryptCmd)

	backupCmd := &cobra.Command{
		Use:   "backup [database]",
		Short: "Create a backup of a database",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var ciph *encryption.Cipher
			if cfg.Encryption.Enabled {
				ciph, err = getCipher(cfg)
				if err != nil {
					return err
				}
			}

			var mgr *database.Manager
			if ciph != nil {
				mgr = database.NewEncryptedManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns, ciph)
			} else {
				mgr = database.NewManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns)
			}

			bm := backup.NewManager(cfg.Backup.Dir, cfg.Database.DataDir, mgr, ciph)
			dbName := "main"
			if len(args) > 0 {
				dbName = args[0]
			}

			info, err := bm.CreateBackup(dbName)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			fmt.Printf("backup created: %s (%d bytes)\n", info.Name, info.Size)
			return nil
		},
	}
	rootCmd.AddCommand(backupCmd)

	restoreCmd := &cobra.Command{
		Use:   "restore [backup-file]",
		Short: "Restore a database from a backup file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var ciph *encryption.Cipher
			if cfg.Encryption.Enabled {
				ciph, err = getCipher(cfg)
				if err != nil {
					return err
				}
			}

			var mgr *database.Manager
			if ciph != nil {
				mgr = database.NewEncryptedManager(cfg.Database.DataDir, false, cfg.Database.MaxConns, ciph)
			} else {
				mgr = database.NewManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns)
			}

			bm := backup.NewManager(cfg.Backup.Dir, cfg.Database.DataDir, mgr, ciph)
			dbName, _ := cmd.Flags().GetString("database")
			if dbName == "" {
				dbName = "main"
			}

			if err := bm.RestoreBackup(args[0], dbName); err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			fmt.Printf("restored '%s' from %s\n", dbName, args[0])
			return nil
		},
	}
	restoreCmd.Flags().String("database", "main", "target database name")
	rootCmd.AddCommand(restoreCmd)

	listBackupsCmd := &cobra.Command{
		Use:   "list-backups",
		Short: "List available database backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			mgr := database.NewManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns)
			bm := backup.NewManager(cfg.Backup.Dir, cfg.Database.DataDir, mgr, nil)

			backups, err := bm.ListBackups()
			if err != nil {
				return fmt.Errorf("list backups: %w", err)
			}

			if len(backups) == 0 {
				fmt.Println("no backups found")
				return nil
			}

			fmt.Printf("%-40s %-15s %-12s %s\n", "NAME", "DATABASE", "SIZE", "DATE")
			for _, b := range backups {
				fmt.Printf("%-40s %-15s %-12d %s\n", b.Name, b.Database, b.Size, b.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
	rootCmd.AddCommand(listBackupsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func getCipher(cfg *config.Config) (*encryption.Cipher, error) {
	key := cfg.Encryption.Key
	if key == "" && cfg.Encryption.KeyFile != "" {
		data, err := os.ReadFile(cfg.Encryption.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		key = string(data)
	}
	if key == "" {
		key = os.Getenv("SPARKDB_ENCRYPTION_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("no encryption key found")
	}
	return encryption.NewCipherFromHex(key)
}

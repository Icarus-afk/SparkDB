package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"sparkdb/internal/backup"
	"sparkdb/internal/config"
	"sparkdb/internal/database"
	"sparkdb/internal/encryption"
)

func init() {
	backupCmd := &cobra.Command{
		Use:   "backup [database]",
		Short: "Create a backup of a database",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			var ciph *encryption.Cipher
			if cfg.Encryption.Enabled {
				ciph, err = encryption.GetCipher(cfg.Encryption.Key, cfg.Encryption.KeyFile)
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
				return err
			}

			var ciph *encryption.Cipher
			if cfg.Encryption.Enabled {
				ciph, err = encryption.GetCipher(cfg.Encryption.Key, cfg.Encryption.KeyFile)
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
				return err
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
}

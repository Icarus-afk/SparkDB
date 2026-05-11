package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"sparkdb/internal/auth"
	"sparkdb/internal/config"
	"sparkdb/internal/database"
	"sparkdb/internal/encryption"
)

func init() {
	createDBCmd := &cobra.Command{
		Use:   "create-db [name]",
		Short: "Create a new database file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			var mgr *database.Manager
			if cfg.Encryption.Enabled {
				ciph, err := encryption.GetCipher(cfg.Encryption.Key, cfg.Encryption.KeyFile)
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
				return err
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
}

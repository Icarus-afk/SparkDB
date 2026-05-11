package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sparkdb/internal/encryption"
)

func init() {
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
}

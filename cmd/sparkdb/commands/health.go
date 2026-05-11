package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	healthCmd.Flags().String("url", "http://localhost:9600/health", "health check URL")
	rootCmd.AddCommand(healthCmd)
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server health",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check failed (status %d): %s", resp.StatusCode, string(body))
		}
		fmt.Fprintln(os.Stderr, string(body))
		return nil
	},
}

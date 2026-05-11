package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "sparkdb",
	Short: "SparkDB - lightweight SQLite-powered database server",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "path to config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

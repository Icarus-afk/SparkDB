package commands

import (
	"github.com/spf13/cobra"

	"sparkdb/internal/config"
	"sparkdb/internal/server"
)

func init() {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the SparkDB database server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			srv, err := server.New(cfg)
			if err != nil {
				return err
			}

			return srv.Start()
		},
	}
	rootCmd.AddCommand(startCmd)
}

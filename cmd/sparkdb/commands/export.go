package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"sparkdb/internal/client"

	"github.com/spf13/cobra"
)

var exportHost string
var exportPort int
var exportUser string
var exportPass string
var exportAPIKey string
var exportDB string
var exportFormat string
var exportOutput string

func init() {
	exportCmd := &cobra.Command{
		Use:   "export <table>",
		Short: "Export a table to CSV or JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runExport,
	}
	exportCmd.Flags().StringVar(&exportHost, "host", "localhost", "server host")
	exportCmd.Flags().IntVar(&exportPort, "port", 9600, "server port")
	exportCmd.Flags().StringVar(&exportUser, "user", "", "login username")
	exportCmd.Flags().StringVar(&exportPass, "pass", "", "login password")
	exportCmd.Flags().StringVar(&exportAPIKey, "api-key", "", "API key (alternative to user/pass)")
	exportCmd.Flags().StringVar(&exportDB, "db", "main", "target database")
	exportCmd.Flags().StringVar(&exportFormat, "format", "", "output format (csv or json); defaults to csv")
	exportCmd.Flags().StringVar(&exportOutput, "output", "", "output file (defaults to stdout)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	table := args[0]

	fmtf := exportFormat
	if fmtf == "" {
		fmtf = "csv"
	}
	if fmtf != "csv" && fmtf != "json" {
		return fmt.Errorf("format must be csv or json, got %q", fmtf)
	}

	c := client.New(exportHost, exportPort)
	if exportAPIKey != "" {
		c.SetAPIKey(exportAPIKey)
	} else {
		if exportUser == "" || exportPass == "" {
			return fmt.Errorf("credentials required: use --user and --pass, or --api-key")
		}
		if err := c.Login(exportUser, exportPass); err != nil {
			return fmt.Errorf("login: %w", err)
		}
	}

	res, err := c.Query(exportDB, fmt.Sprintf("SELECT * FROM %s", quoteIdent(table)))
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if res.Error != "" {
		return fmt.Errorf("query: %s", res.Error)
	}

	w := os.Stdout
	if exportOutput != "" {
		f, err := os.Create(exportOutput)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch fmtf {
	case "csv":
		return exportCSV(w, res.Columns, res.Rows)
	case "json":
		return exportJSON(w, res.Columns, res.Rows)
	}
	return nil
}

func exportCSV(w *os.File, columns []string, rows [][]interface{}) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write(columns); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, row := range rows {
		record := make([]string, len(row))
		for i, val := range row {
			if val == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprint(val)
			}
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	return nil
}

func exportJSON(w *os.File, columns []string, rows [][]interface{}) error {
	var records []map[string]interface{}
	for _, row := range rows {
		rec := make(map[string]interface{})
		for i, col := range columns {
			rec[col] = row[i]
		}
		records = append(records, rec)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

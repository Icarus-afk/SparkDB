package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sparkdb/internal/client"

	"github.com/spf13/cobra"
)

var importHost string
var importPort int
var importUser string
var importPass string
var importAPIKey string
var importDB string
var importFormat string

func init() {
	importCmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import data from CSV, JSON, or SQL file",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	importCmd.Flags().StringVar(&importHost, "host", "localhost", "server host")
	importCmd.Flags().IntVar(&importPort, "port", 9600, "server port")
	importCmd.Flags().StringVar(&importUser, "user", "", "login username")
	importCmd.Flags().StringVar(&importPass, "pass", "", "login password")
	importCmd.Flags().StringVar(&importAPIKey, "api-key", "", "API key (alternative to user/pass)")
	importCmd.Flags().StringVar(&importDB, "db", "main", "target database")
	importCmd.Flags().StringVar(&importFormat, "format", "", "input format (csv, json, sql); inferred from extension if not set")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	fmtf := importFormat
	if fmtf == "" {
		ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
		switch ext {
		case "csv":
			fmtf = "csv"
		case "json", "jsonl":
			fmtf = "json"
		case "sql":
			fmtf = "sql"
		default:
			return fmt.Errorf("could not infer format from extension %q, use --format", ext)
		}
	}

	c := client.New(importHost, importPort)
	if importAPIKey != "" {
		c.SetAPIKey(importAPIKey)
	} else {
		if importUser == "" || importPass == "" {
			return fmt.Errorf("credentials required: use --user and --pass, or --api-key")
		}
		if err := c.Login(importUser, importPass); err != nil {
			return fmt.Errorf("login: %w", err)
		}
	}

	switch fmtf {
	case "csv":
		return importCSV(c, filePath)
	case "json":
		return importJSON(c, filePath)
	case "sql":
		return importSQL(c, filePath)
	default:
		return fmt.Errorf("unsupported format: %s (use csv, json, or sql)", fmtf)
	}
}

func importCSV(c *client.Client, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("read csv: %w", err)
	}
	if len(rows) < 2 {
		return fmt.Errorf("csv must have at least a header row and one data row")
	}

	headers := rows[0]
	data := rows[1:]
	tableName := strings.TrimSuffix(filepath.Base(path), ".csv")

	colTypes := inferTypes(headers, data)
	var cols []string
	for i, h := range headers {
		cols = append(cols, fmt.Sprintf("%s %s", quoteIdent(h), colTypes[i]))
	}

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdent(tableName), strings.Join(cols, ", "))
	fmt.Printf("Creating table: %s\n", createSQL)
	res, err := c.Query(importDB, createSQL)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	if res.Error != "" {
		return fmt.Errorf("create table: %s", res.Error)
	}

	batchSize := 100
	for b := 0; b < len(data); b += batchSize {
		end := b + batchSize
		if end > len(data) {
			end = len(data)
		}
		batch := data[b:end]

		var placeholders []string
		var values []string
		for _, row := range batch {
			phs := make([]string, len(headers))
			for i, val := range row {
				phs[i] = escapeValue(val, colTypes[i])
			}
			placeholders = append(placeholders, "("+strings.Join(phs, ",")+")")
			_ = values
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
			quoteIdent(tableName),
			strings.Join(quoteIdents(headers), ", "),
			strings.Join(placeholders, ", "))

		res, err := c.Query(importDB, insertSQL)
		if err != nil {
			return fmt.Errorf("insert batch %d: %w", b/batchSize, err)
		}
		if res.Error != "" {
			return fmt.Errorf("insert batch %d: %s", b/batchSize, res.Error)
		}
		fmt.Printf("  imported %d rows\n", end)
	}

	fmt.Printf("Imported %d rows into %s\n", len(data), tableName)
	return nil
}

func importJSON(c *client.Client, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("json must contain at least one object")
	}

	tableName := strings.TrimSuffix(filepath.Base(path), ".json")
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}

	var cols []string
	for _, h := range headers {
		colType := inferJSONType(rows[0][h])
		cols = append(cols, fmt.Sprintf("%s %s", quoteIdent(h), colType))
	}

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdent(tableName), strings.Join(cols, ", "))
	fmt.Printf("Creating table: %s\n", createSQL)
	res, err := c.Query(importDB, createSQL)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	if res.Error != "" {
		return fmt.Errorf("create table: %s", res.Error)
	}

	batchSize := 100
	for b := 0; b < len(rows); b += batchSize {
		end := b + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[b:end]

		var placeholders []string
		for _, row := range batch {
			var vals []string
			for _, h := range headers {
				vals = append(vals, escapeValue(fmt.Sprint(row[h]), "TEXT"))
			}
			placeholders = append(placeholders, "("+strings.Join(vals, ",")+")")
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
			quoteIdent(tableName),
			strings.Join(quoteIdents(headers), ", "),
			strings.Join(placeholders, ", "))

		res, err := c.Query(importDB, insertSQL)
		if err != nil {
			return fmt.Errorf("insert batch %d: %w", b/batchSize, err)
		}
		if res.Error != "" {
			return fmt.Errorf("insert batch %d: %s", b/batchSize, res.Error)
		}
		fmt.Printf("  imported %d rows\n", end)
	}

	fmt.Printf("Imported %d rows into %s\n", len(rows), tableName)
	return nil
}

func importSQL(c *client.Client, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	statements := splitSQL(content)
	if len(statements) == 0 {
		return fmt.Errorf("no SQL statements found")
	}

	for i, stmt := range statements {
		res, err := c.Query(importDB, stmt)
		if err != nil {
			return fmt.Errorf("statement %d: %w", i+1, err)
		}
		if res.Error != "" {
			return fmt.Errorf("statement %d: %s", i+1, res.Error)
		}
	}

	fmt.Printf("Executed %d SQL statements from %s\n", len(statements), path)
	return nil
}

func inferTypes(headers []string, data [][]string) []string {
	types := make([]string, len(headers))
	for i := range types {
		types[i] = "TEXT"
	}

	for _, row := range data {
		for i, val := range row {
			if i >= len(types) {
				continue
			}
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			if _, err := strconv.ParseInt(val, 10, 64); err == nil {
				if types[i] == "TEXT" {
					types[i] = "INTEGER"
				}
				continue
			}
			if _, err := strconv.ParseFloat(val, 64); err == nil {
				if types[i] != "INTEGER" {
					types[i] = "REAL"
				}
				continue
			}
			types[i] = "TEXT"
		}
	}
	return types
}

func inferJSONType(val interface{}) string {
	if val == nil {
		return "TEXT"
	}
	switch val.(type) {
	case float64:
		s := fmt.Sprint(val)
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return "INTEGER"
		}
		return "REAL"
	case bool:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func escapeValue(val, colType string) string {
	val = strings.TrimSpace(val)
	if val == "" || strings.ToUpper(val) == "NULL" {
		return "NULL"
	}
	if colType == "INTEGER" || colType == "REAL" {
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			return val
		}
	}
	return "'" + strings.ReplaceAll(val, "'", "''") + "'"
}

func splitSQL(content string) []string {
	var stmts []string
	var buf strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		buf.WriteString(line + "\n")
		if strings.HasSuffix(strings.TrimSpace(trimmed), ";") {
			stmts = append(stmts, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}


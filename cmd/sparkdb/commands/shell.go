package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sparkdb/internal/client"
	"sparkdb/internal/format"

	"github.com/spf13/cobra"
)

var shellHost string
var shellPort int
var shellUser string
var shellPass string
var shellAPIKey string
var shellDB string
var shellOneLine string

func init() {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Interactive SQL shell",
		SilenceUsage: true,
		Long: `Start an interactive SQL shell connected to a SparkDB server.

Meta-commands:
  \q          quit
  \dt         list tables
  \d <name>   describe table
  \use <db>   switch database
  \db         show current database
  \list       list all databases
  \?          help`,
		RunE: runShell,
	}
	shellCmd.Flags().StringVar(&shellHost, "host", "localhost", "server host")
	shellCmd.Flags().IntVar(&shellPort, "port", 9600, "server port")
	shellCmd.Flags().StringVar(&shellUser, "user", "", "login username")
	shellCmd.Flags().StringVar(&shellPass, "pass", "", "login password")
	shellCmd.Flags().StringVar(&shellAPIKey, "api-key", "", "API key (alternative to user/pass)")
	shellCmd.Flags().StringVar(&shellDB, "db", "main", "target database")
	shellCmd.Flags().StringVar(&shellOneLine, "command", "", "run a single query and exit (non-interactive)")
	rootCmd.AddCommand(shellCmd)

	queryCmd := &cobra.Command{
		Use:   "query <sql>",
		Short: "Run a single SQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shellOneLine = args[0]
			return runShell(cmd, args)
		},
		SilenceUsage: true,
	}
	queryCmd.Flags().StringVar(&shellHost, "host", "localhost", "server host")
	queryCmd.Flags().IntVar(&shellPort, "port", 9600, "server port")
	queryCmd.Flags().StringVar(&shellUser, "user", "", "login username")
	queryCmd.Flags().StringVar(&shellPass, "pass", "", "login password")
	queryCmd.Flags().StringVar(&shellAPIKey, "api-key", "", "API key (alternative to user/pass)")
	queryCmd.Flags().StringVar(&shellDB, "db", "main", "target database")
	rootCmd.AddCommand(queryCmd)
}

func runShell(cmd *cobra.Command, args []string) error {
	c := client.New(shellHost, shellPort)

	if shellAPIKey != "" {
		c.SetAPIKey(shellAPIKey)
	} else {
		if shellUser == "" || shellPass == "" {
			return fmt.Errorf("credentials required: use --user and --pass, or --api-key")
		}
		if err := c.Login(shellUser, shellPass); err != nil {
			return fmt.Errorf("login: %w", err)
		}
	}

	if shellOneLine != "" {
		executeQuery(c, shellOneLine)
		return nil
	}

	fmt.Printf("Connected to SparkDB at %s:%d (user: %s, db: %s)\n", shellHost, shellPort, shellUser, shellDB)
	fmt.Println("Type \\q to quit, \\? for help")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	var buf strings.Builder
	continuation := false

	for {
		if continuation {
			fmt.Print("  -> ")
		} else {
			fmt.Printf("sparkdb[%s]> ", shellDB)
		}

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()

		line = strings.TrimSpace(line)

		if !continuation && strings.HasPrefix(line, "\\") {
			if handleMeta(c, line) {
				continue
			}
		}

		buf.WriteString(line)
		buf.WriteString(" ")

		trimmed := strings.TrimSpace(buf.String())
		if strings.HasSuffix(trimmed, ";") || strings.HasPrefix(strings.ToUpper(trimmed), "\\") {
			continuation = false
			q := strings.TrimSuffix(strings.TrimSpace(buf.String()), ";")
			q = strings.TrimSpace(q)

			if q != "" {
				if strings.HasPrefix(q, "\\") {
					handleMeta(c, q)
				} else {
					executeQuery(c, q)
				}
			}
			buf.Reset()
		} else {
			continuation = true
		}
	}

	if s := strings.TrimSpace(buf.String()); s != "" {
		executeQuery(c, strings.TrimSuffix(s, ";"))
	}

	return nil
}

func executeQuery(c *client.Client, query string) {
	res, err := c.Query(shellDB, query)
	if err != nil {
		if res != nil && res.Error != "" {
			fmt.Printf("Error: %s\n", res.Error)
			return
		}
		fmt.Printf("Error: %s\n", err)
		return
	}

	if res.Error != "" {
		fmt.Printf("Error: %s\n", res.Error)
		return
	}

	if len(res.Columns) == 0 {
		if res.Time != "" {
			fmt.Printf("Query OK (%s)\n", res.Time)
		} else {
			fmt.Println("Query OK")
		}
		return
	}

	format.Table(os.Stdout, res.Columns, res.Rows)
	if res.Time != "" {
		fmt.Printf("Time: %s\n", res.Time)
	}
}

func handleMeta(c *client.Client, line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return true
	}

	switch parts[0] {
	case "\\q", "\\quit", "exit":
		fmt.Println("bye")
		os.Exit(0)
	case "\\?":
		fmt.Println(`Meta-commands:
  \q          quit
  \dt         list tables in current db
  \d <name>   describe table columns
  \use <db>   switch to a different database
  \db         show current database
  \list       list all databases
  \?          this help`)
	case "\\dt":
		res, err := c.Query(shellDB, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return true
		}
		fmt.Printf("Tables in '%s':\n", shellDB)
		for _, row := range res.Rows {
			if len(row) > 0 {
				fmt.Printf("  %s\n", row[0])
			}
		}
		if len(res.Rows) == 0 {
			fmt.Println("  (none)")
		}
	case "\\d":
		if len(parts) < 2 {
			fmt.Println("Usage: \\d <table_name>")
			return true
		}
		res, err := c.Query(shellDB, "PRAGMA table_info("+quoteIdent(parts[1])+")")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return true
		}
		fmt.Printf("Columns of %s:\n", parts[1])
		for _, row := range res.Rows {
			if len(row) >= 6 {
				null := "YES"
				if row[3] == "1" {
					null = "NO"
				}
				pk := ""
				if row[5] == "1" {
					pk = " PK"
				}
				def := ""
				if row[4] != nil {
					def = fmt.Sprintf(" DEFAULT %v", row[4])
				}
				fmt.Printf("  %-20s %-10s %s%s%s\n", row[1], row[2], null, def, pk)
			}
		}
	case "\\use":
		if len(parts) < 2 {
			fmt.Printf("Current database: %s\n", shellDB)
			return true
		}
		shellDB = parts[1]
		fmt.Printf("Switched to database '%s'\n", shellDB)
	case "\\db":
		fmt.Printf("Current database: %s\n", shellDB)
	case "\\list":
		dbs, err := c.ListDatabases()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return true
		}
		fmt.Println("Available databases:")
		for _, db := range dbs {
			cur := ""
			if db == shellDB {
				cur = "  <-- current"
			}
			fmt.Printf("  %s%s\n", db, cur)
		}
		if len(dbs) == 0 {
			fmt.Println("  (create one with: sparkdb create-db <name>)")
		}
	default:
		fmt.Printf("Unknown meta-command: %s (try \\?)\n", parts[0])
	}
	return true
}

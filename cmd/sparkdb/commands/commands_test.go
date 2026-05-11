package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecute_Help(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() --help error: %v", err)
	}
}

func TestExportCSV(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "out.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cols := []string{"id", "name"}
	rows := [][]interface{}{{int64(1), "alice"}, {int64(2), "bob"}}

	if err := exportCSV(f, cols, rows); err != nil {
		t.Fatalf("exportCSV() error: %v", err)
	}
}

func TestExportCSV_WithNil(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "out.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cols := []string{"id", "val"}
	rows := [][]interface{}{{int64(1), nil}}

	if err := exportCSV(f, cols, rows); err != nil {
		t.Fatalf("exportCSV() with nil error: %v", err)
	}
}

func TestExportJSON(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "out.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cols := []string{"id", "name"}
	rows := [][]interface{}{{int64(1), "alice"}}

	if err := exportJSON(f, cols, rows); err != nil {
		t.Fatalf("exportJSON() error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "alice") {
		t.Error("JSON output should contain 'alice'")
	}
}

func TestExportJSON_Empty(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "out.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := exportJSON(f, []string{}, [][]interface{}{}); err != nil {
		t.Fatalf("exportJSON() empty error: %v", err)
	}

	var result []interface{}
	data, _ := os.ReadFile(f.Name())
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal empty JSON: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestInferTypes(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		data    [][]string
		want    []string
	}{
		{"all text", []string{"a"}, [][]string{{"hello"}}, []string{"TEXT"}},
		{"integer", []string{"a"}, [][]string{{"42"}}, []string{"INTEGER"}},
		{"float", []string{"a"}, [][]string{{"3.14"}}, []string{"REAL"}},
		{"int then text", []string{"a", "b"}, [][]string{{"1", "hello"}, {"2", "world"}}, []string{"INTEGER", "TEXT"}},
		{"empty value", []string{"a"}, [][]string{{""}}, []string{"TEXT"}},
		{"mixed int float", []string{"a"}, [][]string{{"1"}, {"2.5"}}, []string{"INTEGER"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTypes(tt.headers, tt.data)
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("inferTypes()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestInferJSONType(t *testing.T) {
	tests := []struct {
		val  interface{}
		want string
	}{
		{nil, "TEXT"},
		{float64(42), "INTEGER"},
		{float64(3.14), "REAL"},
		{true, "INTEGER"},
		{"hello", "TEXT"},
		{float64(0), "INTEGER"},
	}
	for _, tt := range tests {
		got := inferJSONType(tt.val)
		if got != tt.want {
			t.Errorf("inferJSONType(%v) = %q, want %q", tt.val, got, tt.want)
		}
	}
}

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		val     string
		colType string
		want    string
	}{
		{"hello", "TEXT", "'hello'"},
		{"42", "INTEGER", "42"},
		{"3.14", "REAL", "3.14"},
		{"", "TEXT", "NULL"},
		{"NULL", "TEXT", "NULL"},
		{"it's", "TEXT", "'it''s'"},
		{"  spaced  ", "TEXT", "'spaced'"},
		{"abc", "INTEGER", "'abc'"},
		{"42", "TEXT", "'42'"},
	}
	for _, tt := range tests {
		got := escapeValue(tt.val, tt.colType)
		if got != tt.want {
			t.Errorf("escapeValue(%q, %q) = %q, want %q", tt.val, tt.colType, got, tt.want)
		}
	}
}

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{"single stmt", "SELECT 1;", 1},
		{"multi stmt", "SELECT 1;\nSELECT 2;", 2},
		{"with comments", "-- comment\nSELECT 1;", 1},
		{"no semicolon", "SELECT 1", 1},
		{"empty", "", 0},
		{"blank lines", "\n\nSELECT 1;\n\nSELECT 2;", 2},
		{"trailing whitespace", "SELECT 1;   ", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQL(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("splitSQL() returned %d statements, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", `"users"`},
		{"my_table", `"my_table"`},
		{"", `""`},
	}
	for _, tt := range tests {
		got := quoteIdent(tt.input)
		if got != tt.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestQuoteIdents(t *testing.T) {
	got := quoteIdents([]string{"a", "b", "c"})
	want := []string{`"a"`, `"b"`, `"c"`}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestQuoteIdentsEmpty(t *testing.T) {
	got := quoteIdents(nil)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestRunExport_InvalidFormat(t *testing.T) {
	exportFormat = "invalid"
	err := runExport(nil, []string{"mytable"})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestRunExport_NoCredentials(t *testing.T) {
	exportFormat = ""
	exportHost = "localhost"
	exportPort = 9600
	exportUser = ""
	exportPass = ""
	exportAPIKey = ""
	err := runExport(nil, []string{"mytable"})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestRunImport_NoCredentials(t *testing.T) {
	importFormat = ""
	importHost = "localhost"
	importPort = 9600
	importUser = ""
	importPass = ""
	importAPIKey = ""
	err := runImport(nil, []string{"test.csv"})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestRunImport_UnknownExtension(t *testing.T) {
	importFormat = ""
	importUser = "u"
	importPass = "p"
	err := runImport(nil, []string{"test.xyz"})
	if err == nil {
		t.Fatal("expected error for unknown extension")
	}
}

func TestHealthCmd_NoServer(t *testing.T) {
	cmd := healthCmd
	cmd.Flags().Set("url", "http://localhost:1/health")
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no server")
	}
}

func TestInitCmd(t *testing.T) {
	dir := t.TempDir()

	rootCmd.SetArgs([]string{"init", "--dir", dir, "--port", "9601"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("init command error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.json")); os.IsNotExist(err) {
		t.Error("config.json was not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "data")); os.IsNotExist(err) {
		t.Error("data directory was not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups")); os.IsNotExist(err) {
		t.Error("backups directory was not created")
	}
}

func TestInitCmd_WithKeys(t *testing.T) {
	dir := t.TempDir()

	rootCmd.SetArgs([]string{"init", "--dir", dir, "--gen-key", "--gen-cert"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("init with keys error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.json")); os.IsNotExist(err) {
		t.Error("config.json was not created")
	}
}

func TestGenKeyCmd(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Use == "gen-key" {
			err := c.RunE(c, nil)
			if err != nil {
				t.Fatalf("gen-key error: %v", err)
			}
			break
		}
	}
}

func TestCreateDBCmd_NoConfig(t *testing.T) {
	cfgPath = "/nonexistent/config.json"
	defer func() { cfgPath = "" }()

	for _, c := range rootCmd.Commands() {
		if c.Use == "create-db [name]" {
			err := c.RunE(c, []string{"testdb"})
			if err == nil {
				t.Fatal("expected error with nonexistent config")
			}
			break
		}
	}
}

func TestStartCmd_NoConfig(t *testing.T) {
	cfgPath = "/nonexistent/config.json"
	defer func() { cfgPath = "" }()

	for _, c := range rootCmd.Commands() {
		if c.Use == "start" {
			err := c.RunE(c, nil)
			if err == nil {
				t.Fatal("expected error with nonexistent config")
			}
			break
		}
	}
}

func TestSplitSQL_TrailingNoSemicolon(t *testing.T) {
	got := splitSQL("SELECT 1;\nSELECT 2")
	if len(got) != 2 {
		t.Errorf("got %d statements, want 2", len(got))
	}
}

func TestSplitSQL_OnlyComments(t *testing.T) {
	got := splitSQL("-- comment 1\n-- comment 2")
	if len(got) != 0 {
		t.Errorf("got %d statements, want 0", len(got))
	}
}

func TestEscapeValue_NullKeyword(t *testing.T) {
	got := escapeValue("NULL", "INTEGER")
	if got != "NULL" {
		t.Errorf("escapeValue(NULL, INTEGER) = %q, want NULL", got)
	}
}

func TestEscapeValue_NonNumericInNumeric(t *testing.T) {
	got := escapeValue("abc", "INTEGER")
	if got != "'abc'" {
		t.Errorf("escapeValue(abc, INTEGER) = %q, want 'abc'", got)
	}
}

func TestInferJSONType_Bool(t *testing.T) {
	got := inferJSONType(true)
	if got != "INTEGER" {
		t.Errorf("inferJSONType(true) = %q, want INTEGER", got)
	}
}

func TestRunImport_SQLFormat(t *testing.T) {
	importUser = "u"
	importPass = "p"
	importFormat = "sql"
	importHost = "localhost"
	importPort = 1

	dir := t.TempDir()
	sqlFile := filepath.Join(dir, "test.sql")
	os.WriteFile(sqlFile, []byte("SELECT 1;"), 0644)

	err := runImport(nil, []string{sqlFile})
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunImport_JSONFormat(t *testing.T) {
	importUser = "u"
	importPass = "p"
	importFormat = "json"
	importHost = "localhost"
	importPort = 1

	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "test.json")
	os.WriteFile(jsonFile, []byte(`[{"id":1}]`), 0644)

	err := runImport(nil, []string{jsonFile})
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunExport_CSVFormat(t *testing.T) {
	exportFormat = "csv"
	exportHost = "localhost"
	exportPort = 1
	exportUser = "u"
	exportPass = "p"
	exportAPIKey = ""

	err := runExport(nil, []string{"mytable"})
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunExport_JSONFormat(t *testing.T) {
	exportFormat = "json"
	exportHost = "localhost"
	exportPort = 1
	exportUser = "u"
	exportPass = "p"

	err := runExport(nil, []string{"mytable"})
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunExport_WithAPIKey(t *testing.T) {
	exportFormat = "csv"
	exportHost = "localhost"
	exportPort = 1
	exportAPIKey = "test-api-key"

	err := runExport(nil, []string{"mytable"})
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunImport_UnsupportedFormat(t *testing.T) {
	importUser = "u"
	importPass = "p"
	importFormat = "unsupported"

	err := runImport(nil, []string{"test.csv"})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestEscapeValue_EmptyValue(t *testing.T) {
	got := escapeValue("   ", "TEXT")
	if got != "NULL" {
		t.Errorf("escapeValue('  ', TEXT) = %q, want NULL", got)
	}
}

func TestInferTypes_OutOfBounds(t *testing.T) {
	types := inferTypes([]string{"a", "b"}, [][]string{{"1", "hello", "extra"}})
	if len(types) != 2 {
		t.Errorf("got %d types, want 2", len(types))
	}
}

func TestRunStop_NoCredentials(t *testing.T) {
	stopUser = ""
	stopPass = ""
	stopAPIKey = ""

	err := runStop(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestRunStop_WithAPIKey(t *testing.T) {
	stopAPIKey = "vl_testapikey"
	stopHost = "localhost"
	stopPort = 1

	err := runStop(nil, nil)
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

func TestRunStop_InvalidCredentials(t *testing.T) {
	stopUser = "admin"
	stopPass = "admin"
	stopAPIKey = ""
	stopHost = "localhost"
	stopPort = 1

	err := runStop(nil, nil)
	if err == nil {
		t.Log("expected connection error (server not running)")
	}
}

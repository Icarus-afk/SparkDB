package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"id", "name"}, [][]interface{}{{1, "alice"}, {2, "bob"}})

	output := buf.String()
	if !strings.Contains(output, "id") {
		t.Error("output should contain column 'id'")
	}
	if !strings.Contains(output, "name") {
		t.Error("output should contain column 'name'")
	}
	if !strings.Contains(output, "alice") {
		t.Error("output should contain 'alice'")
	}
	if !strings.Contains(output, "bob") {
		t.Error("output should contain 'bob'")
	}
	if !strings.Contains(output, "(2 rows)") {
		t.Error("output should contain '(2 rows)'")
	}
}

func TestTableEmptyColumns(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{}, [][]interface{}{})
	if buf.Len() != 0 {
		t.Error("expected empty output for no columns")
	}
}

func TestTableSingleRow(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"x"}, [][]interface{}{{42}})

	output := buf.String()
	if !strings.Contains(output, "(1 row)") {
		t.Error("output should contain '(1 row)'")
	}
}

func TestTableShortColumns(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"a", "b"}, [][]interface{}{{1, 2}})

	output := buf.String()
	if !strings.Contains(output, " a ") {
		t.Error("short columns should be padded to min width")
	}
	if !strings.Contains(output, " 1 ") {
		t.Error("short values should be padded to min width")
	}
}

func TestTableVariableWidths(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"short", "verylongcolumnname"}, [][]interface{}{{"tiny", "extremelylongvalue"}})

	output := buf.String()
	if !strings.Contains(output, "extremelylongvalue") {
		t.Error("output should contain long values")
	}
}

func TestPlural(t *testing.T) {
	if plural(0) != "s" {
		t.Error("plural(0) should return 's'")
	}
	if plural(1) != "" {
		t.Error("plural(1) should return ''")
	}
	if plural(2) != "s" {
		t.Error("plural(2) should return 's'")
	}
	if plural(100) != "s" {
		t.Error("plural(100) should return 's'")
	}
}

func TestRenderSep(t *testing.T) {
	sep := renderSep([]int{3, 5})
	if !strings.HasPrefix(sep, "+") {
		t.Error("separator should start with '+'")
	}
	if !strings.HasSuffix(sep, "+") {
		t.Error("separator should end with '+'")
	}
}

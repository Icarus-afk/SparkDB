package format

import (
	"fmt"
	"io"
	"strings"
)

func Table(w io.Writer, columns []string, rows [][]interface{}) {
	if len(columns) == 0 {
		return
	}

	colWidths := make([]int, len(columns))
	for i, c := range columns {
		colWidths[i] = len(c)
	}
	for _, row := range rows {
		for i, val := range row {
			w := len(fmt.Sprint(val))
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	min := 3
	for i := range colWidths {
		if colWidths[i] < min {
			colWidths[i] = min
		}
	}

	sep := renderSep(colWidths)
	fmt.Fprintln(w, sep)
	fmt.Fprint(w, "|")
	for i, c := range columns {
		fmt.Fprintf(w, " %-*s |", colWidths[i], c)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, sep)

	for _, row := range rows {
		fmt.Fprint(w, "|")
		for i, val := range row {
			s := fmt.Sprint(val)
			fmt.Fprintf(w, " %-*s |", colWidths[i], s)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, sep)

	fmt.Fprintf(w, "(%d row%s)\n", len(rows), plural(len(rows)))
}

func renderSep(widths []int) string {
	var b strings.Builder
	b.WriteString("+")
	for _, w := range widths {
		b.WriteString(strings.Repeat("-", w+2))
		b.WriteString("+")
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

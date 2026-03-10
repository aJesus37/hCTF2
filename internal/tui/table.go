package tui

import (
	"fmt"
	"io"
	"strings"
)

// Column defines a table column with a header and width.
type Column struct {
	Header string
	Width  int
}

// PrintTable writes a lipgloss-styled table to w.
func PrintTable(w io.Writer, cols []Column, rows [][]string) {
	// Header
	var headers []string
	for _, c := range cols {
		headers = append(headers, HeaderStyle.Width(c.Width).Render(c.Header))
	}
	fmt.Fprintln(w, strings.Join(headers, ""))

	// Separator
	var sep []string
	for _, c := range cols {
		sep = append(sep, MutedStyle.Render(strings.Repeat("─", c.Width)))
	}
	fmt.Fprintln(w, strings.Join(sep, ""))

	// Rows
	for _, row := range rows {
		var cells []string
		for i, c := range cols {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			cells = append(cells, CellStyle.Width(c.Width).Render(val))
		}
		fmt.Fprintln(w, strings.Join(cells, ""))
	}
}

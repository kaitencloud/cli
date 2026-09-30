// Package output renders command results as tables, JSON, or YAML.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Supported output formats.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// Table is a tabular result with column headers and rows, used for table-formatted output.
type Table struct {
	Columns []string
	Rows    [][]string
}

// ValidateFormat returns an error if format is not a supported output format.
func ValidateFormat(format string) error {
	switch strings.TrimSpace(format) {
	case "", FormatTable, FormatJSON, FormatYAML:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// Write renders value (or table, for table format) to w in the given format.
// Callers that have no tabular form for value supply an empty Table; those results are
// rendered as YAML rather than failing, because the work they report is already done.
func Write(w io.Writer, format string, value any, table Table) error {
	switch strings.TrimSpace(format) {
	case "", FormatTable:
		if len(table.Columns) == 0 {
			return writeYAML(w, value)
		}
		return WriteTable(w, table)
	case FormatJSON:
		return writeJSON(w, value)
	case FormatYAML:
		return writeYAML(w, value)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// WriteTable renders table as an aligned, tab-separated table to w.
func WriteTable(w io.Writer, table Table) error {
	if len(table.Columns) == 0 {
		return fmt.Errorf("table columns are required")
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(table.Columns, "\t")); err != nil {
		return err
	}
	for _, row := range table.Rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeJSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, string(data))
	return err
}

func writeYAML(w io.Writer, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, string(data))
	return err
}

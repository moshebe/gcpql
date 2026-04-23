package errorreporting

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

// FormatJSON outputs result as indented JSON.
func FormatJSON(w io.Writer, result *ListResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// FormatTable outputs result as a formatted table.
func FormatTable(w io.Writer, result *ListResult) error {
	fmt.Fprintf(w, "Project: %s\n\n", result.Project)

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle(fmt.Sprintf("TOP ERROR GROUPS (%d groups)", result.Total))
	t.AppendHeader(table.Row{"#", "Count", "Services", "First Seen", "Last Seen", "Message"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 6, WidthMax: 80},
	})

	for i, g := range result.Groups {
		services := strings.Join(g.AffectedServices, ", ")
		if services == "" {
			services = "-"
		}
		firstSeen := "-"
		if !g.FirstSeen.IsZero() {
			firstSeen = g.FirstSeen.Format("2006-01-02 15:04")
		}
		lastSeen := "-"
		if !g.LastSeen.IsZero() {
			lastSeen = g.LastSeen.Format("2006-01-02 15:04")
		}
		t.AppendRow(table.Row{i + 1, g.Count, services, firstSeen, lastSeen, g.Message})
	}
	t.Render()
	return nil
}

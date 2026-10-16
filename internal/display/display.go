package display

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"claude-usage/internal/parser"
)

var (
	labelStyle = lipgloss.NewStyle().Width(10).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func colorStyle(pct int) lipgloss.Style {
	switch {
	case pct >= 80:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // red
	case pct >= 50:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	}
}

func progressBar(pct int) string {
	filled := min(max(pct/10, 0), 10)
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func renderRow(label string, entry *parser.UsageEntry) string {
	if entry == nil {
		emptyBar := dimStyle.Render(progressBar(0))
		return fmt.Sprintf("%s %s  %-5s\n",
			labelStyle.Render(label),
			emptyBar,
			dimStyle.Render("--"),
		)
	}
	bar := colorStyle(entry.Pct).Render(progressBar(entry.Pct))
	pct := fmt.Sprintf("%d%%", entry.Pct)
	resetIn := formatDuration(entry.ResetsIn)
	resetAt := entry.ResetsAt.Format("Jan 2 3:04pm")
	return fmt.Sprintf("%s %s  %-5s  resets in %-8s  %s\n",
		labelStyle.Render(label),
		bar,
		pct,
		resetIn,
		dimStyle.Render("("+resetAt+")"),
	)
}

type jsonEntry struct {
	Pct      int    `json:"pct"`
	ResetsAt string `json:"resets_at"`
	ResetsIn string `json:"resets_in"`
}

type jsonOutput struct {
	FetchedAt string     `json:"fetched_at"`
	Session   *jsonEntry `json:"session,omitempty"`
	Weekly    *jsonEntry `json:"weekly,omitempty"`
}

func toJSONEntry(e *parser.UsageEntry) *jsonEntry {
	if e == nil {
		return nil
	}
	return &jsonEntry{
		Pct:      e.Pct,
		ResetsAt: e.ResetsAt.Format(time.RFC3339),
		ResetsIn: formatDuration(e.ResetsIn),
	}
}

// Render formats UsageData as a colored terminal string or a single-line JSON
// object. Both forms end with a newline so polling output stays line-delimited.
func Render(data parser.UsageData, asJSON bool) (string, error) {
	if asJSON {
		out := jsonOutput{
			FetchedAt: data.FetchedAt.Format(time.RFC3339),
			Session:   toJSONEntry(data.Session),
			Weekly:    toJSONEntry(data.Weekly),
		}
		b, err := json.Marshal(out)
		if err != nil {
			return "", fmt.Errorf("marshalling JSON: %w", err)
		}
		return string(b) + "\n", nil
	}

	var sb strings.Builder
	if data.Session != nil || data.WantSession {
		sb.WriteString(renderRow("Session", data.Session))
	}
	if data.Weekly != nil || data.WantWeekly {
		sb.WriteString(renderRow("Weekly", data.Weekly))
	}
	return sb.String(), nil
}

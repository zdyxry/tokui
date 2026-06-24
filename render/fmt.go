package render

import (
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// formatNumber adds thousands separators to an integer for easier reading.
func formatNumber(n int64) string {
	in := strconv.FormatInt(n, 10)
	start := 0
	if n < 0 {
		start = 1
	}
	out := make([]byte, 0, len(in)+len(in)/3)
	if n < 0 {
		out = append(out, '-')
	}
	for i := start; i < len(in); i++ {
		if i > start && (len(in)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, in[i])
	}
	return string(out)
}

// truncateVisual truncates s to fit within maxWidth visual cells.
func truncateVisual(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	return runewidth.Truncate(s, maxWidth, "")
}

// fmtName truncates the name if it exceeds maxWidth and adds ellipsis
func fmtName(name string, maxWidth int) string {
	ellipsis := "..."
	ellipsisWidth := lipgloss.Width(ellipsis)

	if maxWidth <= ellipsisWidth {
		return ellipsis
	}

	if lipgloss.Width(name) <= maxWidth {
		return name
	}

	targetTextVisualWidth := maxWidth - ellipsisWidth
	if targetTextVisualWidth < 1 {
		return ellipsis
	}

	truncatedText := lipgloss.NewStyle().MaxWidth(targetTextVisualWidth).Render(name)
	return truncatedText + ellipsis
}

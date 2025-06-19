package render

import (
	"github.com/charmbracelet/lipgloss"
)

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

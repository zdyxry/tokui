package render

import (
	"github.com/charmbracelet/lipgloss"
)

// fmtName truncates the name if it exceeds maxWidth and adds ellipsis
func fmtName(name string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}

	if lipgloss.Width(name) <= maxWidth {
		return name
	}

	return lipgloss.NewStyle().MaxWidth(maxWidth).Render(name)
}

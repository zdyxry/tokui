package filter

import (
	"strings"

	"github.com/zdyxry/tokui/structure"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// ID for the name filter
	NameFilterID ID = "NameFilter"
)

// NameFilter filters individual *structure.Entry instances by their path value.
// If an entry's path value does not contain the user input, it will not be filtered/discarded.
//
// User input is handled by a textinput.Model instance, so
// the filter must update its internal state by providing the corresponding Updater implementation.
type NameFilter struct {
	input   textinput.Model
	enabled bool
}

// NewNameFilter creates a new name filter.
func NewNameFilter(placeholder string) *NameFilter {
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ebbd34"))
	ti := textinput.New()

	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = lipgloss.Width(placeholder)
	ti.Prompt = "  " // 使用搜索图标 (Nerd Font: nf-fa-search)
	ti.PromptStyle, ti.TextStyle = textStyle, textStyle

	return &NameFilter{input: ti, enabled: false}
}

// ID returns the unique identifier of the filter.
func (nf *NameFilter) ID() ID {
	return NameFilterID
}

// Toggle switches the enabled state of the filter.
func (nf *NameFilter) Toggle() {
	nf.enabled = !nf.enabled
}

// IsEnabled returns whether the filter is enabled.
func (nf *NameFilter) IsEnabled() bool {
	return nf.enabled
}

// ClearInput clears the input content but keeps the filter enabled.
func (nf *NameFilter) ClearInput() {
	nf.input.Reset()
}

// Filter filters a *structure.Entry by checking if its path value contains the current filter input.
func (nf *NameFilter) Filter(e *structure.Entry) bool {
	// If not enabled, always pass through
	if !nf.enabled {
		return true
	}
	return strings.Contains(
		strings.ToLower(e.Name()),
		strings.ToLower(nf.input.Value()),
	)
}

func (nf *NameFilter) Update(msg tea.Msg) {
	if !nf.enabled {
		return
	}

	if resizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		nf.input.Width = resizeMsg.Width - 4
	}

	var cmd tea.Cmd
	nf.input, cmd = nf.input.Update(msg)
	_ = cmd
}

func (nf *NameFilter) Reset() {
	nf.enabled = false
	nf.input.Reset()
}

func (nf *NameFilter) View() string {
	if !nf.enabled {
		return ""
	}

	// Create a bordered style for the input box
	s := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderTop(true).
		Padding(0, 1)

	return s.Render(nf.input.View())
}

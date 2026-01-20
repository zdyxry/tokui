package render

import (
	"github.com/charmbracelet/bubbles/key"
)

type bindingKey string

func (bk bindingKey) String() string {
	return string(bk)
}

const (
	backspace        bindingKey = "backspace"
	quit             bindingKey = "q"
	cancel           bindingKey = "ctrl+c"
	escape           bindingKey = "esc"
	enter            bindingKey = "enter"
	editFile         bindingKey = "e"
	quickSearch      bindingKey = "/"
	toggleChart      bindingKey = "ctrl+w"
	toggleLangFilter bindingKey = "tab"
	toggleLangSelect bindingKey = "ctrl+l"
	toggleHelp       bindingKey = "?"
	left             bindingKey = "left"
	right            bindingKey = "right"
)

var toggleHelpBinding = key.NewBinding(
	key.WithKeys(toggleHelp.String()),
	key.WithHelp(
		bindKeyStyle.Render(toggleHelp.String()),
		helpDescStyle.Render(" - Toggle full help"),
	),
)

var navigateKeyMap = [][]key.Binding{
	{
		key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp(
				bindKeyStyle.Render("↑/k"),
				helpDescStyle.Render(" - Move up"),
			),
		),
		key.NewBinding(
			key.WithKeys(editFile.String()),
			key.WithHelp(
				bindKeyStyle.Render(editFile.String()),
				helpDescStyle.Render(" - Open file in editor"),
			),
		),
		key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp(
				bindKeyStyle.Render("↓/j"),
				helpDescStyle.Render(" - Move down"),
			),
		),
		key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp(
				bindKeyStyle.Render("g/home"),
				helpDescStyle.Render(" - Go to top"),
			),
		),
		key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp(
				bindKeyStyle.Render("G/end"),
				helpDescStyle.Render(" - Go to bottom"),
			),
		),
	},
}

var shortHelp = append(navigateKeyMap[0], toggleHelpBinding)

var dirsKeyMap = [][]key.Binding{
	{
		key.NewBinding(
			key.WithKeys(enter.String()),
			key.WithHelp(
				bindKeyStyle.Render(enter.String()),
				helpDescStyle.Render(" - Open directory / Preview file"),
			),
		),
		key.NewBinding(
			key.WithKeys(backspace.String()),
			key.WithHelp(
				bindKeyStyle.Render(backspace.String()),
				helpDescStyle.Render(" - Go back up"),
			),
		),
		key.NewBinding(
			key.WithKeys(quit.String(), cancel.String()),
			key.WithHelp(
				bindKeyStyle.Render(quit.String()+"/"+cancel.String()),
				helpDescStyle.Render(" - Quit"),
			),
		),
	},
	{
		key.NewBinding(
			key.WithKeys(toggleLangFilter.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleLangFilter.String()),
				helpDescStyle.Render(" - Toggle language filter"),
			),
		),
		key.NewBinding(
			key.WithKeys(quickSearch.String()),
			key.WithHelp(
				bindKeyStyle.Render(quickSearch.String()),
				helpDescStyle.Render(" - Quick search"),
			),
		),
		key.NewBinding(
			key.WithKeys(toggleLangSelect.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleLangSelect.String()),
				helpDescStyle.Render(" - Language select"),
			),
		),
		key.NewBinding(
			key.WithKeys(toggleChart.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleChart.String()),
				helpDescStyle.Render(" - Language proportion chart"),
			),
		),
	},
	{
		toggleHelpBinding,
	},
}

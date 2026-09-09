package render

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type bindingKey string

func (bk bindingKey) String() string {
	return string(bk)
}

// parseBindingKey converts a KeyMsg to a bindingKey. Single-character uppercase
// runes are preserved so that Shift+letter bindings (e.g. "S") work, while
// everything else is normalized to lower case.
func parseBindingKey(msg tea.KeyMsg) bindingKey {
	raw := msg.String()
	if utf8.RuneCountInString(raw) == 1 && raw != strings.ToLower(raw) {
		return bindingKey(raw)
	}
	return bindingKey(strings.ToLower(raw))
}

const (
	backspace          bindingKey = "backspace"
	quit               bindingKey = "q"
	cancel             bindingKey = "ctrl+c"
	escape             bindingKey = "esc"
	enter              bindingKey = "enter"
	editFile           bindingKey = "e"
	quickSearch        bindingKey = "/"
	globalSearch       bindingKey = "ctrl+p"
	toggleChart        bindingKey = "ctrl+w"
	toggleLangFilter   bindingKey = "tab"
	toggleLangSelect   bindingKey = "ctrl+l"
	toggleHelp         bindingKey = "?"
	toggleTree         bindingKey = "t"
	toggleTreemap      bindingKey = "m"
	toggleTreemapColor bindingKey = "c"
	cycleTreemapSize   bindingKey = "M"
	cycleSortColumn    bindingKey = "s"
	toggleSortOrder    bindingKey = "S"
	toggleChanged      bindingKey = "a"
	expandAll          bindingKey = "}"
	collapseAll        bindingKey = "{"
	expandSubtree      bindingKey = "]"
	collapseSubtree    bindingKey = "["
)

var toggleHelpBinding = key.NewBinding(
	key.WithKeys(toggleHelp.String()),
	key.WithHelp(
		bindKeyStyle.Render(toggleHelp.String()),
		helpDescStyle.Render(" - Toggle full help"),
	),
)

// Tree-mode expand/collapse bindings, named so the short help bar can reuse them.
var (
	expandAllBinding = key.NewBinding(
		key.WithKeys(expandAll.String()),
		key.WithHelp(
			bindKeyStyle.Render(expandAll.String()),
			helpDescStyle.Render(" - Expand all dirs (tree)"),
		),
	)
	collapseAllBinding = key.NewBinding(
		key.WithKeys(collapseAll.String()),
		key.WithHelp(
			bindKeyStyle.Render(collapseAll.String()),
			helpDescStyle.Render(" - Collapse all dirs (tree)"),
		),
	)
	expandSubtreeBinding = key.NewBinding(
		key.WithKeys(expandSubtree.String()),
		key.WithHelp(
			bindKeyStyle.Render(expandSubtree.String()),
			helpDescStyle.Render(" - Expand subtree (tree)"),
		),
	)
	collapseSubtreeBinding = key.NewBinding(
		key.WithKeys(collapseSubtree.String()),
		key.WithHelp(
			bindKeyStyle.Render(collapseSubtree.String()),
			helpDescStyle.Render(" - Collapse subtree (tree)"),
		),
	)
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

// shortHelpTree is the bottom-bar help shown in tree mode: adds the
// expand/collapse keys so they are discoverable without opening full help.
var shortHelpTree = append(navigateKeyMap[0],
	expandAllBinding,
	collapseAllBinding,
	expandSubtreeBinding,
	collapseSubtreeBinding,
	toggleHelpBinding,
)

var dirsKeyMap = [][]key.Binding{
	{
		key.NewBinding(
			key.WithKeys(enter.String()),
			key.WithHelp(
				bindKeyStyle.Render(enter.String()),
				helpDescStyle.Render(" - Open/Expand dir / Preview file"),
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
			key.WithKeys(toggleTree.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleTree.String()),
				helpDescStyle.Render(" - Toggle tree mode"),
			),
		),
		expandAllBinding,
		collapseAllBinding,
		expandSubtreeBinding,
		collapseSubtreeBinding,
		key.NewBinding(
			key.WithKeys(toggleTreemap.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleTreemap.String()),
				helpDescStyle.Render(" - Toggle treemap"),
			),
		),
		key.NewBinding(
			key.WithKeys(toggleTreemapColor.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleTreemapColor.String()),
				helpDescStyle.Render(" - Toggle treemap color mode"),
			),
		),
		key.NewBinding(
			key.WithKeys(cycleTreemapSize.String()),
			key.WithHelp(
				bindKeyStyle.Render(cycleTreemapSize.String()),
				helpDescStyle.Render(" - Cycle treemap size metric"),
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
				helpDescStyle.Render(" - Cycle language filter"),
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
			key.WithKeys(globalSearch.String()),
			key.WithHelp(
				bindKeyStyle.Render(globalSearch.String()),
				helpDescStyle.Render(" - Global search"),
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
		key.NewBinding(
			key.WithKeys(cycleSortColumn.String()),
			key.WithHelp(
				bindKeyStyle.Render(cycleSortColumn.String()),
				helpDescStyle.Render(" - Cycle sort column"),
			),
		),
		key.NewBinding(
			key.WithKeys(toggleSortOrder.String()),
			key.WithHelp(
				bindKeyStyle.Render(toggleSortOrder.String()),
				helpDescStyle.Render(" - Toggle sort order"),
			),
		),
	},
	{
		toggleHelpBinding,
	},
}

// diffKeyMap lists the extra bindings available in Diff and Compare modes.
var diffKeyMap = []key.Binding{
	key.NewBinding(
		key.WithKeys(toggleChanged.String()),
		key.WithHelp(
			bindKeyStyle.Render(toggleChanged.String()),
			helpDescStyle.Render(" - Toggle changed-only / all files"),
		),
	),
}

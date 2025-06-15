package render

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const Version = "v1.0.0-tokei"

type ScanFinished struct{}

type ViewModel struct {
	dirModel *DirModel
	nav      *Navigation
}

func NewViewModel(n *Navigation, dirModel *DirModel) *ViewModel {
	return &ViewModel{
		nav:      n,
		dirModel: dirModel,
	}
}

func (vm *ViewModel) Init() tea.Cmd {
	// Disable mouse events as we use keyboard-only interactions
	return tea.DisableMouse
}

func (vm *ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		bk := bindingKey(strings.ToLower(msg.String()))

		switch bk {
		case quit, cancel:
			return vm, tea.Quit
		case enter:
			// If DirModel is in input mode, special handling is needed
			if vm.dirModel.mode == INPUT {
				// Get the currently selected row information before exiting search mode
				if vm.dirModel.dirsTable.Rows() != nil && len(vm.dirModel.dirsTable.SelectedRow()) >= 3 {
					selectedRow := vm.dirModel.dirsTable.SelectedRow()
					entryName := selectedRow[2]

					// Exit search mode
					vm.dirModel.ExitSearchMode()

					// Navigate using the directory name
					vm.nav.Down(entryName, 0)
					vm.dirModel.Update(ScanFinished{})
				} else {
					// If no valid row is selected, just exit search mode
					vm.dirModel.ExitSearchMode()
				}
			} else {
				// Normal mode Enter handling
				vm.levelDown()
			}
		case backspace:
			// If DirModel is in input mode, don't handle top-level shortcuts
			if vm.dirModel.mode == INPUT {
				break
			}
			vm.levelUp()
		}
	}

	// Pass all messages to the child model (DirModel) for processing
	_, cmd = vm.dirModel.Update(msg)

	return vm, cmd
}

func (vm *ViewModel) View() string {
	return vm.dirModel.View()
}

func (vm *ViewModel) levelDown() {
	if vm.dirModel.dirsTable.Rows() == nil || len(vm.dirModel.dirsTable.SelectedRow()) < 3 {
		return
	}

	selectedRow := vm.dirModel.dirsTable.SelectedRow()
	// Column 3 (index 2) contains the entry's base name
	entryName := selectedRow[2]

	vm.nav.Down(entryName, vm.dirModel.dirsTable.Cursor())

	// Notify DirModel that data has changed and needs re-rendering
	vm.dirModel.Update(ScanFinished{})
}

func (vm *ViewModel) levelUp() {
	vm.nav.Up()
	// Notify DirModel that data has changed and needs re-rendering
	vm.dirModel.Update(ScanFinished{})
}

// NewProgressBar creates a custom styled progress bar (currently unused)
func NewProgressBar(width int, full, empty rune) progress.Model {
	maxCharLen := max(
		lipgloss.Width(string(full)),
		lipgloss.Width(string(empty)),
	)

	return progress.New(
		progress.WithColorProfile(termenv.Ascii),
		progress.WithWidth(width/maxCharLen),
		progress.WithFillCharacters(full, empty),
		progress.WithoutPercentage(),
	)
}

func buildTable() *table.Model {
	tbl := table.New(table.WithFocused(true))

	style := table.DefaultStyles()
	style.Header = TableHeaderStyle
	style.Cell = lipgloss.NewStyle()
	style.Selected = SelectedRowStyle

	tbl.SetStyles(style)
	tbl.Help = help.New()

	return &tbl
}

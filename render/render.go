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

type ScanFinished struct {
	ResetCursor bool
}

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
		if vm.dirModel.mode == SELECT_LANG {
			bk := bindingKey(strings.ToLower(msg.String()))
			if bk == quit || bk == cancel {
				return vm, tea.Quit
			}
			break
		}
		bk := bindingKey(strings.ToLower(msg.String()))

		switch bk {
		case quit, cancel:
			return vm, tea.Quit
		case enter:
			if vm.dirModel.mode == INPUT {
				selectedEntry := vm.dirModel.SelectedEntry()
				vm.dirModel.ExitSearchMode()
				if selectedEntry != nil {
					if selectedEntry.IsDir {
						if vm.dirModel.treeMode {
							selectedEntry.Expanded = !selectedEntry.Expanded
							vm.dirModel.Update(ScanFinished{})
						} else {
							vm.nav.Down(selectedEntry.Name(), 0)
							vm.dirModel.Update(ScanFinished{ResetCursor: true})
						}
					} else {
						vm.dirModel.ShowFilePreview(selectedEntry.Path)
					}
				}
			} else {
				if vm.dirModel.treeMode {
					vm.toggleExpand()
				} else {
					vm.levelDown()
				}
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
	entryName := selectedRow[2]
	filePath := vm.nav.AbsPathFromSelectedRow(selectedRow)

	entry := vm.nav.Entry().GetChild(entryName)
	if entry != nil && !entry.IsDir {
		vm.dirModel.ShowFilePreview(filePath)
		return
	}

	vm.nav.Down(entryName, vm.dirModel.dirsTable.Cursor())
	vm.dirModel.Update(ScanFinished{ResetCursor: true})
}

func (vm *ViewModel) toggleExpand() {
	entry := vm.dirModel.SelectedEntry()
	if entry == nil || !entry.IsDir {
		return
	}
	entry.Expanded = !entry.Expanded
	vm.dirModel.Update(ScanFinished{})
}

func (vm *ViewModel) levelUp() {
	if vm.nav.entryStack.len() > 0 {
		vm.nav.Up()
		vm.dirModel.Update(ScanFinished{ResetCursor: true})
	}
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

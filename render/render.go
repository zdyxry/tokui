package render

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ScanFinished struct {
	ResetCursor bool
}

type ViewModel struct {
	dirModel *DirModel
	nav      *Navigation

	// testInitDoneCh is closed by Update once a ScanFinished message has been
	// processed. It is used by integration tests to avoid sending further
	// messages before initialization has finished.
	testInitDoneCh chan struct{}
}

func NewViewModel(n *Navigation, dirModel *DirModel) *ViewModel {
	return &ViewModel{
		nav:      n,
		dirModel: dirModel,
	}
}

func (vm *ViewModel) Init() tea.Cmd {
	return nil
}

func (vm *ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if vm.dirModel.mode == SELECT_LANG {
			bk := parseBindingKey(msg)
			if bk == cancel {
				return vm, tea.Quit
			}
			break
		}
		bk := parseBindingKey(msg)

		switch bk {
		case quit:
			if vm.dirModel.mode != PREVIEW && vm.dirModel.mode != INPUT && vm.dirModel.mode != SEARCH {
				return vm, tea.Quit
			}
		case cancel:
			return vm, tea.Quit
		case enter:
			switch vm.dirModel.mode {
			case INPUT:
				selectedEntry := vm.dirModel.SelectedEntry()
				vm.dirModel.ExitSearchMode()
				if selectedEntry != nil {
					if selectedEntry.IsDir {
						if vm.dirModel.treeMode {
							selectedEntry.Expanded = !selectedEntry.Expanded
							vm.dirModel.Update(ScanFinished{})
						} else {
							vm.nav.Down(selectedEntry.Name(), 0, 1)
							vm.dirModel.Update(ScanFinished{ResetCursor: true})
						}
					} else {
						vm.dirModel.ShowFilePreview(selectedEntry.Path)
					}
				} else if vm.dirModel.IsParentSelected() {
					vm.levelUp()
				}
			case SEARCH:
				vm.dirModel.applySearchResult()
			default:
				if vm.dirModel.IsParentSelected() {
					vm.levelUp()
				} else if vm.dirModel.treeMode {
					vm.toggleExpand()
				} else if vm.dirModel.treemapMode {
					vm.treemapDrillDown()
				} else {
					vm.levelDown()
				}
			}
		case backspace:
			// If DirModel is in input mode, don't handle top-level shortcuts
			if vm.dirModel.mode == INPUT || vm.dirModel.mode == SEARCH {
				break
			}
			vm.levelUp()
		}

	case tea.MouseMsg:
		return vm.handleMouseMsg(msg)

	case ScanFinished:
		_, cmd = vm.dirModel.Update(msg)
		if vm.testInitDoneCh != nil {
			close(vm.testInitDoneCh)
			vm.testInitDoneCh = nil
		}
		return vm, cmd
	}

	// Pass all messages to the child model (DirModel) for processing
	_, cmd = vm.dirModel.Update(msg)

	return vm, cmd
}

func (vm *ViewModel) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case vm.dirModel.mode == PREVIEW:
		// Click outside the preview box closes it; wheel events scroll the viewport.
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && !vm.dirModel.isInsidePreviewBox(msg.X, msg.Y) {
			vm.dirModel.ClosePreview()
			return vm, nil
		}
		_, cmd = vm.dirModel.filePreview.Update(msg)
		return vm, cmd

	case vm.dirModel.mode == SELECT_LANG:
		// Consume all mouse events while the language overlay is open.
		cmd, _ = vm.dirModel.handleLangSelectMouse(msg)
		return vm, cmd

	case vm.dirModel.showCart:
		// Click outside the chart closes it.
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && !vm.dirModel.isInsideChartBox(msg.X, msg.Y) {
			vm.dirModel.showCart = false
			return vm, nil
		}
		return vm, nil
	}

	if vm.dirModel.treemapMode {
		// Right-click mirrors left double-click drill-down: it goes back up to
		// the parent directory. This is the only mouse route up in treemap mode,
		// where no clickable ".." element exists.
		if msg.Button == tea.MouseButtonRight && msg.Action == tea.MouseActionPress {
			vm.levelUp()
			return vm, nil
		}
		idx, clickCount, handled := vm.dirModel.handleTreemapMouse(msg)
		if handled && clickCount >= 2 && idx >= 0 {
			vm.treemapDrillDown()
		}
		return vm, nil
	}

	row, clickCount, handled := vm.dirModel.handleTableMouse(msg)
	if handled {
		if clickCount >= 2 && row >= 0 {
			if vm.dirModel.treeMode {
				vm.toggleExpand()
			} else {
				vm.levelDown()
			}
		}
		return vm, nil
	}

	return vm, nil
}

func (vm *ViewModel) View() string {
	return vm.dirModel.View()
}

func (vm *ViewModel) levelDown() {
	if vm.dirModel.dirsTable.Rows() == nil || len(vm.dirModel.dirsTable.SelectedRow()) < 3 {
		return
	}

	if vm.dirModel.IsParentSelected() {
		vm.levelUp()
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

	vm.nav.Down(entryName, vm.dirModel.dirsTable.Cursor(), 1)
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

func (vm *ViewModel) treemapDrillDown() {
	entry := vm.dirModel.SelectedEntry()
	if entry == nil {
		return
	}
	if entry.IsDir {
		// The selected block may be a nested directory. Walk from the current
		// navigation entry down through each path component.
		currentPath := vm.nav.Entry().Path
		rel, err := filepath.Rel(currentPath, entry.Path)
		if err != nil {
			return
		}
		parts := strings.Split(rel, string(filepath.Separator))
		var names []string
		for _, p := range parts {
			if p == ".." {
				return
			}
			if p != "" && p != "." {
				names = append(names, p)
			}
		}
		if len(names) == 0 {
			return
		}
		for i, name := range names {
			parentCursor := 0
			if i == 0 {
				parentCursor = vm.dirModel.treemapSelected
			}
			vm.nav.Down(name, parentCursor, 0)
		}
		vm.dirModel.treemapSelected = 0
		vm.dirModel.Update(ScanFinished{ResetCursor: true})
	} else {
		vm.dirModel.ShowFilePreview(entry.Path)
	}
}

func (vm *ViewModel) levelUp() {
	if vm.nav.entryStack.len() > 0 {
		vm.nav.Up()
		vm.dirModel.treemapSelected = 0
		vm.dirModel.Update(ScanFinished{ResetCursor: true})
	}
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

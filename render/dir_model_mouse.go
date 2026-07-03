package render

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const doubleClickThreshold = 300 * time.Millisecond

type mouseClick struct {
	time time.Time
	row  int
}

// overlayBounds tracks the screen position of the currently rendered overlay.
type overlayBounds struct {
	kind      string // "preview", "chart", "langselect" or "search"
	x, y      int    // top-left corner
	w, h      int    // width and height
	langStart int    // first visible language index (for langselect)
	langEnd   int    // last visible language index + 1 (for langselect)
}

func (dm *DirModel) isInsideOverlay(x, y int) bool {
	b := dm.overlayBounds
	return x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h
}

func (dm *DirModel) isInsidePreviewBox(x, y int) bool {
	return dm.overlayBounds.kind == "preview" && dm.isInsideOverlay(x, y)
}

func (dm *DirModel) isInsideChartBox(x, y int) bool {
	return dm.overlayBounds.kind == "chart" && dm.isInsideOverlay(x, y)
}

func (dm *DirModel) isInsideLangSelectBox(x, y int) bool {
	return dm.overlayBounds.kind == "langselect" && dm.isInsideOverlay(x, y)
}

func (dm *DirModel) langSelectIndexAtY(y int) int {
	b := dm.overlayBounds
	if b.kind != "langselect" {
		return -1
	}
	// Content starts one cell below the top border.
	contentY := y - b.y - 1
	if contentY < 2 {
		return -1 // title or description line
	}
	listY := contentY - 2
	if b.langStart > 0 {
		if listY == 0 {
			return -1 // "..." scroll indicator
		}
		listY--
	}
	idx := b.langStart + listY
	if idx < b.langStart || idx >= b.langEnd || idx >= len(dm.languages) {
		return -1
	}
	return idx
}

// tableRowAtY maps a terminal Y coordinate to a table row index.
// It returns -1 when the coordinate is not over a data row.
func (dm *DirModel) tableRowAtY(y int) int {
	// Use the view from the last render; the user clicked what they saw.
	view := dm.lastTableView
	if view == "" {
		view = dm.dirsTable.View()
	}
	lines := strings.Split(view, "\n")
	if y < tableHeaderHeight || y >= len(lines) {
		return -1
	}
	visibleIdx := y - tableHeaderHeight
	if visibleIdx < 0 || visibleIdx >= len(lines)-tableHeaderHeight {
		return -1
	}

	cursor := dm.dirsTable.Cursor()
	cursorLine := dm.findCursorLineInView(view)
	if cursorLine < 0 {
		return -1
	}

	row := cursor + (visibleIdx - cursorLine)
	if row < 0 || row >= len(dm.tableEntries) {
		return -1
	}
	return row
}

// findCursorLineInView finds the visible line index (0-based, excluding header)
// that corresponds to the currently selected row by comparing exactly rendered rows.
func (dm *DirModel) findCursorLineInView(view string) int {
	lines := strings.Split(view, "\n")
	if len(lines) <= tableHeaderHeight {
		return -1
	}
	selectedRow := dm.dirsTable.SelectedRow()
	if selectedRow == nil {
		return -1
	}
	renderedSelected := strings.TrimRight(dm.renderTableRow(selectedRow, true), " ")
	for i := tableHeaderHeight; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " ") == renderedSelected {
			return i - tableHeaderHeight
		}
	}
	return -1
}

// renderTableRow replicates the rendering logic of bubbles/table's renderRow so
// that we can match a data row to its exact rendered line.
func (dm *DirModel) renderTableRow(row table.Row, selected bool) string {
	cols := dm.dirsTable.Columns()
	var cells []string
	for i, value := range row {
		if i >= len(cols) || cols[i].Width <= 0 {
			continue
		}
		style := lipgloss.NewStyle().Width(cols[i].Width).MaxWidth(cols[i].Width).Inline(true)
		renderedCell := style.Render(runewidth.Truncate(value, cols[i].Width, "…"))
		cells = append(cells, renderedCell)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Left, cells...)
	if selected {
		line = SelectedRowStyle.Render(line)
	}
	return line
}

// handleTreemapMouse handles mouse events for the treemap view.
// It returns the selected block index, the click count, and whether the event
// was consumed.
func (dm *DirModel) handleTreemapMouse(msg tea.MouseMsg) (int, int, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if dm.treemapSelected > 0 {
			dm.treemapSelected--
		}
		return dm.treemapSelected, 0, true
	case tea.MouseButtonWheelDown:
		if dm.treemapSelected < len(dm.treemapBlocks)-1 {
			dm.treemapSelected++
		}
		return dm.treemapSelected, 0, true
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return -1, 0, false
		}
		relX := msg.X
		relY := msg.Y - dm.treemapOffsetY
		idx := treemapBlockAt(dm.treemapBlocks, relX, relY)
		if idx < 0 {
			return -1, 0, false
		}
		now := time.Now()
		clickCount := 1
		if !dm.lastClick.time.IsZero() && now.Sub(dm.lastClick.time) < doubleClickThreshold && dm.lastClick.row == idx {
			clickCount = 2
		}
		dm.lastClick = mouseClick{time: now, row: idx}
		dm.treemapSelected = idx
		return idx, clickCount, true
	}
	return -1, 0, false
}

// handleTableMouse handles mouse events for the main directory table.
// It returns the selected row, the click count (2 for a double-click) and
// whether the event was consumed.
func (dm *DirModel) handleTableMouse(msg tea.MouseMsg) (int, int, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		dm.dirsTable.MoveUp(1)
		return -1, 0, true
	case tea.MouseButtonWheelDown:
		dm.dirsTable.MoveDown(1)
		return -1, 0, true
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return -1, 0, false
		}
		// Clicking the header row sorts by the clicked column.
		if msg.Y >= 0 && msg.Y < tableHeaderHeight {
			handled := dm.handleHeaderClick(msg.X)
			// Reset the row double-click tracker so a header click does not
			// accidentally combine with a subsequent row click.
			dm.lastClick = mouseClick{}
			return -1, 0, handled
		}
		row := dm.tableRowAtY(msg.Y)
		if row < 0 {
			return -1, 0, false
		}
		now := time.Now()
		clickCount := 1
		if !dm.lastClick.time.IsZero() && now.Sub(dm.lastClick.time) < doubleClickThreshold && dm.lastClick.row == row {
			clickCount = 2
		}
		dm.lastClick = mouseClick{time: now, row: row}
		dm.dirsTable.SetCursor(row)
		return row, clickCount, true
	}
	return -1, 0, false
}

// handleHeaderClick sorts by the column under the given X coordinate.
// It returns true when the click was on a sortable column header.
func (dm *DirModel) handleHeaderClick(x int) bool {
	cols := dm.dirsTable.Columns()
	visibleCols := dm.visibleColumns()
	if len(cols) == 0 || len(visibleCols) != len(cols) {
		return false
	}

	startX := 0
	for i, c := range cols {
		endX := startX + c.Width
		if x >= startX && x < endX {
			key := visibleCols[i].SortKey
			if key == SortByNone {
				return false
			}
			if dm.sortState.Key == key {
				dm.toggleSortOrder()
			} else {
				dm.sortState = SortState{Key: key, Desc: defaultDescForSortKey(key)}
			}
			dm.updateTableData()
			return true
		}
		startX = endX
	}
	return false
}

// handleLangSelectMouse handles mouse events for the language selection overlay.
func (dm *DirModel) handleLangSelectMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if dm.selectIndex > 0 {
			dm.selectIndex--
		}
		return nil, true
	case tea.MouseButtonWheelDown:
		if dm.selectIndex < len(dm.languages)-1 {
			dm.selectIndex++
		}
		return nil, true
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil, false
		}
		if !dm.isInsideLangSelectBox(msg.X, msg.Y) {
			dm.mode = READY
			dm.selectMode = false
			dm.updateTableData()
			return nil, true
		}
		idx := dm.langSelectIndexAtY(msg.Y)
		if idx >= 0 && idx < len(dm.languages) {
			dm.selectIndex = idx
			lang := dm.languages[idx]
			dm.selectedLangs[lang] = !dm.selectedLangs[lang]
		}
		return nil, true
	}
	return nil, false
}

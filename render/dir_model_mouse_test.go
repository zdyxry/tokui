package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDirModelMouseHelpers(t *testing.T) {
	dm := newTestDirModel()

	dm.overlayBounds = overlayBounds{kind: "preview", x: 10, y: 5, w: 40, h: 20}
	cases := []struct {
		x, y int
		want bool
	}{
		{10, 5, true},
		{49, 24, true},
		{9, 5, false},
		{50, 5, false},
		{10, 4, false},
		{10, 25, false},
	}
	for _, tc := range cases {
		got := dm.isInsideOverlay(tc.x, tc.y)
		if got != tc.want {
			t.Errorf("isInsideOverlay(%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
		if !dm.isInsidePreviewBox(tc.x, tc.y) && tc.want {
			t.Errorf("isInsidePreviewBox(%d,%d) should be true", tc.x, tc.y)
		}
	}

	dm.overlayBounds.kind = "chart"
	if !dm.isInsideChartBox(15, 10) {
		t.Error("expected inside chart box")
	}
	if dm.isInsidePreviewBox(15, 10) {
		t.Error("expected not inside preview box")
	}

	dm.overlayBounds.kind = "langselect"
	if !dm.isInsideLangSelectBox(15, 10) {
		t.Error("expected inside language select box")
	}
}

func TestDirModelLangSelectIndexAtY(t *testing.T) {
	dm := newTestDirModel()
	dm.languages = []string{"A", "B", "C", "D", "E"}

	t.Run("no scroll offset", func(t *testing.T) {
		dm.overlayBounds = overlayBounds{kind: "langselect", x: 0, y: 5, w: 20, h: 10, langStart: 0, langEnd: 3}
		if got := dm.langSelectIndexAtY(5 + 1); got != -1 { // title line
			t.Errorf("title line = %d, want -1", got)
		}
		if got := dm.langSelectIndexAtY(5 + 3); got != 0 { // first list item
			t.Errorf("first item = %d, want 0", got)
		}
		if got := dm.langSelectIndexAtY(5 + 4); got != 1 {
			t.Errorf("second item = %d, want 1", got)
		}
		if got := dm.langSelectIndexAtY(5 + 6); got != -1 { // past last visible item
			t.Errorf("past end = %d, want -1", got)
		}
	})

	t.Run("with scroll offset", func(t *testing.T) {
		dm.overlayBounds = overlayBounds{kind: "langselect", x: 0, y: 5, w: 20, h: 10, langStart: 2, langEnd: 5}
		if got := dm.langSelectIndexAtY(5 + 2); got != -1 { // "..." line
			t.Errorf("ellipsis line = %d, want -1", got)
		}
		if got := dm.langSelectIndexAtY(5 + 4); got != 2 { // first visible item
			t.Errorf("first visible item = %d, want 2", got)
		}
		if got := dm.langSelectIndexAtY(5 + 6); got != 4 { // last visible item
			t.Errorf("last visible item = %d, want 4", got)
		}
	})
}

func TestDirModelTableRowAtY(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.updateSize(120, 24)
	dm.lastTableView = dm.dirsTable.View()

	cursorLine := dm.findCursorLineInView(dm.lastTableView)
	if cursorLine < 0 {
		t.Fatalf("cursor line not found in view:\n%s", dm.lastTableView)
	}

	y := tableHeaderHeight + cursorLine
	got := dm.tableRowAtY(y)
	want := dm.dirsTable.Cursor()
	if got != want {
		t.Errorf("tableRowAtY(%d) = %d, want %d", y, got, want)
	}

	if dm.tableRowAtY(0) != -1 {
		t.Error("expected header Y to return -1")
	}
	if dm.tableRowAtY(1000) != -1 {
		t.Error("expected out-of-range Y to return -1")
	}
}

func TestDirModelFindCursorLineInView(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.updateSize(120, 24)

	view := dm.dirsTable.View()
	line := dm.findCursorLineInView(view)
	if line < 0 {
		t.Fatalf("findCursorLineInView returned %d", line)
	}

	lines := strings.Split(view, "\n")
	if line >= len(lines)-tableHeaderHeight {
		t.Errorf("cursor line %d out of range for view with %d data lines", line, len(lines)-tableHeaderHeight)
	}
}

func TestDirModelHandleHeaderClick(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.updateSize(120, 24)

	cols := dm.dirsTable.Columns()
	if len(cols) < 8 {
		t.Fatalf("expected at least 8 columns, got %d", len(cols))
	}

	midX := func(idx int) int {
		startX := 0
		for i := 0; i < idx; i++ {
			startX += cols[i].Width
		}
		return startX + cols[idx].Width/2
	}

	t.Run("click sortable column changes key", func(t *testing.T) {
		dm.sortState = SortState{Key: SortByTotal, Desc: true}
		if !dm.handleHeaderClick(midX(2)) {
			t.Fatal("expected Name header click to be handled")
		}
		if dm.sortState.Key != SortByName {
			t.Errorf("expected sort key %q, got %q", SortByName, dm.sortState.Key)
		}
		if dm.sortState.Desc != false {
			t.Errorf("expected Name to default ascending, got Desc=%v", dm.sortState.Desc)
		}
	})

	t.Run("click current sort column toggles order", func(t *testing.T) {
		dm.sortState = SortState{Key: SortByName, Desc: false}
		if !dm.handleHeaderClick(midX(2)) {
			t.Fatal("expected Name header click to be handled")
		}
		if dm.sortState.Key != SortByName {
			t.Errorf("expected sort key to stay %q, got %q", SortByName, dm.sortState.Key)
		}
		if dm.sortState.Desc != true {
			t.Errorf("expected Desc to toggle to true, got %v", dm.sortState.Desc)
		}
	})

	t.Run("click non-sortable column does nothing", func(t *testing.T) {
		dm.sortState = SortState{Key: SortByTotal, Desc: true}
		if dm.handleHeaderClick(midX(0)) {
			t.Error("expected icon header click to be ignored")
		}
		if dm.sortState.Key != SortByTotal {
			t.Errorf("expected sort key to remain %q, got %q", SortByTotal, dm.sortState.Key)
		}
	})

	t.Run("click outside columns does nothing", func(t *testing.T) {
		dm.sortState = SortState{Key: SortByTotal, Desc: true}
		totalWidth := 0
		for _, c := range cols {
			totalWidth += c.Width
		}
		if dm.handleHeaderClick(totalWidth + 10) {
			t.Error("expected click outside columns to be ignored")
		}
	})
}

func TestDirModelHandleTableMouseRoutesHeaderClicks(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.updateSize(120, 24)

	cols := dm.dirsTable.Columns()
	nameX := cols[0].Width + cols[1].Width + cols[2].Width/2

	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	row, clicks, handled := dm.handleTableMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      nameX,
		Y:      0,
	})
	if !handled {
		t.Fatal("expected header click to be handled")
	}
	if row != -1 || clicks != 0 {
		t.Errorf("expected row=-1, clicks=0, got row=%d, clicks=%d", row, clicks)
	}
	if dm.sortState.Key != SortByName {
		t.Errorf("expected sort key to change to %q, got %q", SortByName, dm.sortState.Key)
	}
}

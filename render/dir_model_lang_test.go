package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zdyxry/tokui/structure"
)

func TestDirModelComparableStats(t *testing.T) {
	dm := newTestDirModel()
	root := dm.nav.Entry()

	t.Run("no filter uses total stats", func(t *testing.T) {
		got := dm.comparableStats(root.Child[0])
		want := structure.CodeStats{Code: 20, Comments: 5, Blanks: 5}
		if got != want {
			t.Errorf("a.go total stats: got %+v, want %+v", got, want)
		}
	})

	t.Run("single language filter", func(t *testing.T) {
		dm.langFilterIdx = 0                     // Go
		got := dm.comparableStats(root.Child[1]) // b.py has no Go stats
		want := structure.CodeStats{}
		if got != want {
			t.Errorf("b.py Go stats: got %+v, want %+v", got, want)
		}
	})

	t.Run("multi language filter aggregates", func(t *testing.T) {
		dm.langFilterIdx = -1
		dm.selectedLangs["Go"] = true
		dm.selectedLangs["Python"] = true
		got := dm.comparableStats(root)
		want := structure.CodeStats{Code: 60, Comments: 17, Blanks: 18}
		if got != want {
			t.Errorf("root aggregated stats: got %+v, want %+v", got, want)
		}
	})
}

func TestDirModelLanguageSelectOverlay(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	if len(dm.languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(dm.languages))
	}
	if dm.languages[0] != "Go" || dm.languages[1] != "Python" {
		t.Errorf("languages = %v, want [Go Python]", dm.languages)
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if dm.mode != SELECT_LANG {
		t.Fatalf("expected SELECT_LANG mode, got %v", dm.mode)
	}
	if !dm.selectMode {
		t.Error("expected selectMode to be true")
	}
	if dm.selectIndex != 0 {
		t.Errorf("expected selectIndex 0, got %d", dm.selectIndex)
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if dm.selectIndex != 1 {
		t.Errorf("after down: selectIndex = %d, want 1", dm.selectIndex)
	}
	dm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if dm.selectIndex != 0 {
		t.Errorf("after up: selectIndex = %d, want 0", dm.selectIndex)
	}

	// Boundary at the top.
	dm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if dm.selectIndex != 0 {
		t.Errorf("top boundary: selectIndex = %d, want 0", dm.selectIndex)
	}

	// Boundary at the bottom.
	dm.selectIndex = len(dm.languages) - 1
	dm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if dm.selectIndex != len(dm.languages)-1 {
		t.Errorf("bottom boundary: selectIndex = %d, want %d", dm.selectIndex, len(dm.languages)-1)
	}

	// Toggle selection with space.
	dm.selectIndex = 0 // Go
	dm.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !dm.selectedLangs["Go"] {
		t.Error("expected Go to be selected")
	}
	dm.Update(tea.KeyMsg{Type: tea.KeySpace})
	if dm.selectedLangs["Go"] {
		t.Error("expected Go to be deselected")
	}

	// Enter applies the filter and exits select mode.
	dm.Update(tea.KeyMsg{Type: tea.KeySpace})
	dm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if dm.mode != READY {
		t.Errorf("expected READY after enter, got %v", dm.mode)
	}
	if !dm.selectedLangs["Go"] {
		t.Error("expected Go to remain selected")
	}
	if len(dm.dirsTable.Rows()) != 2 {
		t.Errorf("expected 2 Go rows, got %d", len(dm.dirsTable.Rows()))
	}

	// Escape closes language select without applying the latest selection.
	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	dm.selectIndex = 1 // Python
	dm.Update(tea.KeyMsg{Type: tea.KeySpace})
	dm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if dm.mode != READY {
		t.Errorf("expected READY after escape, got %v", dm.mode)
	}
	if dm.selectMode {
		t.Error("expected selectMode to be false")
	}
	// The previously applied Go filter is still in effect.
	if len(dm.dirsTable.Rows()) != 2 {
		t.Errorf("expected 2 rows after escape, got %d", len(dm.dirsTable.Rows()))
	}
}

func TestDirModelLanguageSelectQClosesOverlay(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if dm.mode != SELECT_LANG {
		t.Fatalf("expected SELECT_LANG mode, got %v", dm.mode)
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if dm.mode != READY {
		t.Errorf("expected q to close language select and return to READY, got %v", dm.mode)
	}
	if dm.selectMode {
		t.Error("expected selectMode to be false after q")
	}
}

func TestDirModelLanguageSelectEscRevertsChanges(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	dm.Update(tea.KeyMsg{Type: tea.KeySpace}) // select Go
	if !dm.selectedLangs["Go"] {
		t.Fatal("expected Go to be selected before escape")
	}

	// Esc should revert to the empty selection state from before the overlay opened.
	dm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if dm.mode != READY {
		t.Errorf("expected READY after escape, got %v", dm.mode)
	}
	if dm.selectedLangs["Go"] {
		t.Error("expected Go selection to be reverted after escape")
	}
	if len(dm.dirsTable.Rows()) != 3 {
		t.Errorf("expected all 3 rows after reverting selection, got %d", len(dm.dirsTable.Rows()))
	}
}

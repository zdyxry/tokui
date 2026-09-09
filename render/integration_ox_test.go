package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewModel-level regression test: O/X expand/collapse the subtree under the
// cursor, and the cursor stays on the same entry afterwards.
func TestViewModelSubtreeExpandCollapseKeys(t *testing.T) {
	vm := newIntegrationViewModel(t, true, false)
	vm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	vm.Update(ScanFinished{})

	dm := vm.dirModel
	key := func(s string) { vm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }

	// Move cursor to the "cmd" directory (row 1 in the fixture).
	key("j")
	cmd := dm.SelectedEntry()
	if cmd == nil || !cmd.IsDir {
		t.Fatalf("expected a dir under cursor, got %+v", cmd)
	}
	rowsBefore := len(dm.tableEntries)

	// O expands the whole subtree under the cursor; sibling stays collapsed.
	key("O")
	if !cmd.Expanded {
		t.Fatal("O did not expand the directory under the cursor")
	}
	if len(dm.tableEntries) <= rowsBefore {
		t.Fatalf("expected more rows after O, got %d (before %d)", len(dm.tableEntries), rowsBefore)
	}
	for _, te := range dm.tableEntries {
		if te.entry.Path == "project/render" && te.entry.Expanded {
			t.Fatal("O must not expand sibling branches")
		}
	}
	if dm.SelectedEntry() != cmd {
		t.Fatal("cursor must stay on the same entry after O")
	}

	// X collapses it back; cursor still on the same entry.
	key("X")
	if cmd.Expanded {
		t.Fatal("X did not collapse the directory under the cursor")
	}
	if dm.SelectedEntry() != cmd {
		t.Fatal("cursor must stay on the same entry after X")
	}

	// O on a file is a no-op.
	key("j") // now on cmd/app.go (first child file)
	if sel := dm.SelectedEntry(); sel != nil && sel.IsDir {
		t.Fatalf("expected a file under cursor, got %s", sel.Path)
	}
	rowsNow := len(dm.tableEntries)
	key("O")
	key("X")
	if len(dm.tableEntries) != rowsNow {
		t.Fatal("O/X on a file must be a no-op")
	}
}

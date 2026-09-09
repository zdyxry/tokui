package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewModel-level regression test: ]/[ expand/collapse the subtree under the
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

	// ] expands the whole subtree under the cursor; sibling stays collapsed.
	key("]")
	if !cmd.Expanded {
		t.Fatal("] did not expand the directory under the cursor")
	}
	if len(dm.tableEntries) <= rowsBefore {
		t.Fatalf("expected more rows after ], got %d (before %d)", len(dm.tableEntries), rowsBefore)
	}
	for _, te := range dm.tableEntries {
		if te.entry.Path == "project/render" && te.entry.Expanded {
			t.Fatal("] must not expand sibling branches")
		}
	}
	if dm.SelectedEntry() != cmd {
		t.Fatal("cursor must stay on the same entry after ]")
	}

	// [ collapses it back; cursor still on the same entry.
	key("[")
	if cmd.Expanded {
		t.Fatal("[ did not collapse the directory under the cursor")
	}
	if dm.SelectedEntry() != cmd {
		t.Fatal("cursor must stay on the same entry after [")
	}

	// ] on a file is a no-op.
	key("j") // now on cmd/app.go (first child file)
	if sel := dm.SelectedEntry(); sel != nil && sel.IsDir {
		t.Fatalf("expected a file under cursor, got %s", sel.Path)
	}
	rowsNow := len(dm.tableEntries)
	key("]")
	key("[")
	if len(dm.tableEntries) != rowsNow {
		t.Fatal("]/[ on a file must be a no-op")
	}
}

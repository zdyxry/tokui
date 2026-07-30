package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

// newShowAllDiffViewModel builds a Diff-mode ViewModel over a real temporary
// directory with one changed file (a.go), one unchanged file (unchanged.go)
// and one unchanged directory (plain/x.go), then switches to the show-all
// view where unchanged rows are rendered faint.
func newShowAllDiffViewModel(t *testing.T) (*ViewModel, *DirModel) {
	t.Helper()

	root := t.TempDir()
	writeFile := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("a.go", "changed\n")
	writeFile("unchanged.go", "same\n")
	writeFile("plain/x.go", "nested\n")

	changes := []gitx.FileChange{
		{Path: "a.go", Added: 3, Deleted: 1, Kind: gitx.Modified},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: filepath.Join(root, "a.go"), Language: "Go", Code: 10},
		{Path: filepath.Join(root, "unchanged.go"), Language: "Go", Code: 10},
		{Path: filepath.Join(root, "plain/x.go"), Language: "Go", Code: 10},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, root); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}

	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines | provider.CapChurn}
	nav := NewCodeNavigation(tree)
	dm := NewDirModelWithMode(nav, info, ModeInfo{Kind: ModeDiff, Range: "HEAD"}, false, false)
	dm.width = 140
	dm.height = 40
	dm.Update(ScanFinished{})

	// Switch to the show-all view: unchanged rows become faint context.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if dm.changedOnly() {
		t.Fatal("setup: expected changed-only filter off")
	}

	return NewViewModel(nav, dm), dm
}

func cursorOnEntry(t *testing.T, dm *DirModel, name string) {
	t.Helper()
	for i, te := range dm.tableEntries {
		if !te.isParent && te.entry.Name() == name {
			dm.dirsTable.SetCursor(i)
			return
		}
	}
	t.Fatalf("no table entry named %q (rows: %v)", name, rowNames(dm))
}

func TestLevelDownFaintFileOpensPreview(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	vm, dm := newShowAllDiffViewModel(t)
	cursorOnEntry(t, dm, "unchanged.go")

	// The row is genuinely faint: its name cell carries the faint escape.
	if got := dm.dirsTable.SelectedRow()[2]; !strings.Contains(got, "\x1b[2m") {
		t.Fatalf("setup: expected a faint name cell, got %q", got)
	}

	vm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !dm.IsInPreviewMode() {
		t.Fatal("expected Enter on a faint unchanged file to open the preview")
	}
	if dm.filePreview.errorMsg != "" {
		t.Fatalf("expected preview content, got error %q", dm.filePreview.errorMsg)
	}
	if dm.filePreview.content != "same\n" {
		t.Errorf("expected unchanged.go contents, got %q", dm.filePreview.content)
	}
}

func TestLevelDownFaintDirNavigates(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	vm, dm := newShowAllDiffViewModel(t)
	cursorOnEntry(t, dm, "plain")

	if got := dm.dirsTable.SelectedRow()[2]; !strings.Contains(got, "\x1b[2m") {
		t.Fatalf("setup: expected a faint name cell, got %q", got)
	}

	vm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := vm.nav.Entry().Name(); got != "plain" {
		t.Fatalf("expected navigation into the faint dir, at %q", got)
	}
}

func TestLevelDownCompareDeletedFilePreviewsDiff(t *testing.T) {
	repo := initPreviewRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(repo, "old", "legacy.rb")
	if err := os.WriteFile(legacy, []byte("legacy code\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "add legacy")
	// Delete the file for the S2 snapshot.
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}

	s1 := provider.Result{Files: []provider.FileStats{
		{Path: legacy, Language: "Ruby", Code: 100},
		{Path: filepath.Join(repo, "main.go"), Language: "Go", Code: 10},
	}}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: filepath.Join(repo, "main.go"), Language: "Go", Code: 10},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromCompare(s1, s2, repo); err != nil {
		t.Fatalf("BuildFromCompare: %v", err)
	}

	info := provider.Info{Name: "scc", Version: "3.x", Capabilities: provider.CapLines | provider.CapDelta}
	mode := ModeInfo{
		Kind:     ModeCompare,
		Range:    "v1..v2",
		RepoRoot: repo,
		DiffRev:  "HEAD",
	}
	nav := NewCodeNavigation(tree)
	dm := NewDirModelWithMode(nav, info, mode, false, false)
	dm.width = 140
	dm.height = 40
	dm.Update(ScanFinished{})
	vm := NewViewModel(nav, dm)

	// Enter the "old" directory, then the deleted file with its marker.
	cursorOnEntry(t, dm, "old")
	vm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := vm.nav.Entry().Name(); got != "old" {
		t.Fatalf("setup: expected navigation into old, at %q", got)
	}
	cursorOnEntry(t, dm, "legacy.rb")
	if got := dm.dirsTable.SelectedRow()[2]; !strings.Contains(got, "(deleted)") {
		t.Fatalf("setup: expected the deleted marker in the name cell, got %q", got)
	}

	vm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !dm.IsInPreviewMode() {
		t.Fatal("expected Enter on a deleted file to open the preview")
	}
	fp := dm.filePreview
	if fp.errorMsg != "" {
		t.Fatalf("expected the deletion diff, got error %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "deleted file mode") || !strings.Contains(fp.content, "legacy code") {
		t.Errorf("expected the deletion diff of legacy.rb, got:\n%s", fp.content)
	}
}

func TestTreeModeEnterOnFileOpensDiffPreview(t *testing.T) {
	repo := initPreviewRepo(t)

	changes := []gitx.FileChange{
		{Path: "main.go", Added: 1, Deleted: 1, Kind: gitx.Modified},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: filepath.Join(repo, "main.go"), Language: "Go", Code: 1},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, repo); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}

	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines | provider.CapChurn}
	nav := NewCodeNavigation(tree)
	dm := NewDirModelWithMode(nav, info, worktreeMode(repo), true /* treeMode */, false)
	dm.width = 140
	dm.height = 40
	dm.Update(ScanFinished{})
	vm := NewViewModel(nav, dm)

	cursorOnEntry(t, dm, "main.go")
	vm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !dm.IsInPreviewMode() {
		t.Fatal("expected Enter on a file in tree mode to open the preview")
	}
	fp := dm.filePreview
	if fp.errorMsg != "" {
		t.Fatalf("expected the file diff, got error %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "old version") || !strings.Contains(fp.content, "new version") {
		t.Errorf("expected the side-by-side diff of main.go, got:\n%s", fp.content)
	}
}

package render

import (
	"bytes"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/zdyxry/tokui/structure"
)

func init() {
	// Force a deterministic, colorless profile so golden files and CI
	// produce the same output regardless of the host terminal.
	lipgloss.SetColorProfile(termenv.Ascii)
}

const (
	integrationWidth  = 120
	integrationHeight = 40
)

// integrationTree returns a small but representative project tree for TUI
// integration tests. It contains files in multiple languages and a nested
// directory so navigation, search, filters and language selection can all be
// exercised.
func integrationTree() *structure.Tree {
	root := structure.NewDirEntry("project")

	cmd := structure.NewDirEntry("project/cmd")
	root.AddChild(cmd)
	cmd.AddChild(structure.NewFileEntry("project/cmd/app.go", map[string]structure.CodeStats{
		"Go": {Code: 100, Comments: 20, Blanks: 10},
	}))
	cmd.AddChild(structure.NewFileEntry("project/cmd/error.go", map[string]structure.CodeStats{
		"Go": {Code: 50, Comments: 10, Blanks: 5},
	}))

	renderDir := structure.NewDirEntry("project/render")
	root.AddChild(renderDir)
	renderDir.AddChild(structure.NewFileEntry("project/render/dir_model.go", map[string]structure.CodeStats{
		"Go": {Code: 200, Comments: 40, Blanks: 20},
	}))

	root.AddChild(structure.NewFileEntry("project/main.go", map[string]structure.CodeStats{
		"Go": {Code: 30, Comments: 5, Blanks: 5},
	}))

	root.AddChild(structure.NewFileEntry("project/README.md", map[string]structure.CodeStats{
		"Markdown": {Code: 20, Comments: 0, Blanks: 0},
	}))

	root.AggregateStats()
	return structure.NewTree(root)
}

// newIntegrationViewModel builds a ViewModel wired to the integration tree.
// The model is left in PENDING mode so tests can observe the full lifecycle.
func newIntegrationViewModel(t *testing.T, treeMode, treemapMode bool) *ViewModel {
	t.Helper()

	nav := NewCodeNavigation(integrationTree())
	dm := NewDirModel(nav, "test-version", treeMode, treemapMode)
	vm := NewViewModel(nav, dm)
	// Synchronization channel used by startIntegrationApp to wait until the
	// initial ScanFinished message has been processed.
	vm.testInitDoneCh = make(chan struct{})
	return vm
}

// startIntegrationApp creates a TestModel for the integration tree, sends the
// initial window-size and scan-finished messages, and waits for the project
// root to be rendered.
func startIntegrationApp(t *testing.T, treeMode, treemapMode bool) *teatest.TestModel {
	t.Helper()

	vm := newIntegrationViewModel(t, treeMode, treemapMode)
	tm := teatest.NewTestModel(t, vm, teatest.WithInitialTermSize(integrationWidth, integrationHeight))

	// Finish initialization just like the real application does. Wait for the
	// ScanFinished message to be processed before returning so the caller can
	// safely assert on the rendered output without racing against startup.
	doneCh := vm.testInitDoneCh
	tm.Send(tea.WindowSizeMsg{Width: integrationWidth, Height: integrationHeight})
	tm.Send(ScanFinished{})
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ScanFinished to be processed")
	}

	return tm
}

// sendKey sends a single key press to the test model. It accepts both special
// key types (e.g. tea.KeyEnter) and runes.
func sendKey(t *testing.T, tm *teatest.TestModel, key string) {
	t.Helper()
	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	})
}

// sendSpecialKey sends a non-rune key message.
func sendSpecialKey(t *testing.T, tm *teatest.TestModel, keyType tea.KeyType) {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: keyType})
}

// waitForOutput blocks until the rendered output contains the requested
// substring. It is used instead of sleeps to keep tests deterministic.
func waitForOutput(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(want))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// waitForAllOutputs blocks until the rendered output contains all of the
// requested substrings. Checking everything in a single WaitFor avoids the
// output buffer being drained between successive checks.
func waitForAllOutputs(t *testing.T, tm *teatest.TestModel, wants ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		for _, want := range wants {
			if !bytes.Contains(bts, []byte(want)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))
}

// quitApp sends the quit key and waits for the program to finish.
func quitApp(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	sendKey(t, tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestIntegration_InitialRenderShowsProject(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Verify the initial render contains the project root, a child directory,
	// a file, and the total column header.
	waitForAllOutputs(t, tm, "project", "cmd", "main.go", "Total")

	quitApp(t, tm)
}

func TestIntegration_NavigateIntoDirectoryAndBack(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Move selection down to "cmd" and enter it.
	sendSpecialKey(t, tm, tea.KeyDown)
	waitForOutput(t, tm, "cmd")
	sendSpecialKey(t, tm, tea.KeyEnter)

	// The cmd directory contents should now be visible.
	waitForAllOutputs(t, tm, "app.go", "error.go")

	// Go back up to the project root.
	sendSpecialKey(t, tm, tea.KeyBackspace)
	waitForOutput(t, tm, "main.go")

	quitApp(t, tm)
}

func TestIntegration_ToggleTreeMode(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	sendKey(t, tm, "t")
	// Tree mode updates the status bar and renders the root children.
	waitForAllOutputs(t, tm, "Tree", "cmd")

	quitApp(t, tm)
}

func TestIntegration_ToggleTreemapMode(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	sendKey(t, tm, "m")
	// Treemap mode updates the status bar and renders top-level blocks.
	waitForAllOutputs(t, tm, "Treemap", "cmd")

	quitApp(t, tm)
}

func TestIntegration_QuickSearchFiltersRows(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Activate the name filter.
	sendKey(t, tm, "/")
	waitForOutput(t, tm, "Filter by name")

	// Type a query that only matches main.go.
	tm.Type("main")
	// main.go becomes the only matching row.
	waitForOutput(t, tm, "main.go")

	// Escape closes the filter and restores all rows.
	sendSpecialKey(t, tm, tea.KeyEsc)
	// After closing the filter, other rows are visible again.
	waitForOutput(t, tm, "cmd")

	quitApp(t, tm)
}

func TestIntegration_GlobalSearchJumpsToFile(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Open global search.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	waitForOutput(t, tm, "Global Search")

	// Search for a file deep in the tree.
	tm.Type("dir_model.go")
	waitForOutput(t, tm, "dir_model.go")

	// Apply the result.
	sendSpecialKey(t, tm, tea.KeyEnter)
	waitForOutput(t, tm, "render")

	quitApp(t, tm)
}

func TestIntegration_CycleSortColumn(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Cycle from Total -> Percent -> Name.
	sendKey(t, tm, "s")
	waitForOutput(t, tm, "% of Parent")

	sendKey(t, tm, "s")
	waitForOutput(t, tm, "Name")

	quitApp(t, tm)
}

func TestIntegration_ToggleSortOrder(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Toggle the current column's sort direction.
	sendKey(t, tm, "S")
	waitForOutput(t, tm, "Total ▲")

	quitApp(t, tm)
}

func TestIntegration_LanguageSelectFilters(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	// Open language selection overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlL})
	waitForOutput(t, tm, "Select Languages")

	// Toggle Go off and Markdown on, then confirm.
	sendSpecialKey(t, tm, tea.KeySpace)
	sendKey(t, tm, "j")
	sendSpecialKey(t, tm, tea.KeySpace)
	sendSpecialKey(t, tm, tea.KeyEnter)

	waitForOutput(t, tm, "Markdown")

	quitApp(t, tm)
}

func TestIntegration_QuitFromReadyMode(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	quitApp(t, tm)

	// After quitting we can read the final output and it should still show the
	// project was rendered.
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatalf("reading final output: %v", err)
	}
	if !bytes.Contains(out, []byte("project")) {
		t.Error("expected final output to contain the project name")
	}
}

func TestIntegration_GoldenInitialRender(t *testing.T) {
	tm := startIntegrationApp(t, false, false)

	quitApp(t, tm)

	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatalf("reading final output: %v", err)
	}
	teatest.RequireEqualOutput(t, out)
}

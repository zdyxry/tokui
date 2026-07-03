package render

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/zdyxry/tokui/search"
	"github.com/zdyxry/tokui/structure"
)

// newSearchInput creates a styled text input for the global search overlay.
func newSearchInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Search files and directories..."
	ti.Focus()
	ti.Prompt = "  "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebbd34"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebbd34"))
	return ti
}

func (dm *DirModel) openGlobalSearch() {
	if dm.mode == SEARCH {
		return
	}
	// Close other overlays/state before entering global search.
	dm.showCart = false
	if dm.mode == PREVIEW {
		dm.filePreview = nil
	}
	dm.mode = SEARCH
	dm.searchInput.Reset()
	dm.searchInput.Focus()
	dm.searchCursor = 0
	dm.searchOffset = 0
	dm.searchMatches = nil
	dm.updateSearchQuery()
}

func (dm *DirModel) closeGlobalSearch() {
	if dm.mode != SEARCH {
		return
	}
	dm.mode = READY
	dm.searchInput.Blur()
	dm.searchMatches = nil
	dm.searchCursor = 0
	dm.searchOffset = 0
	dm.pendingSearchTarget = nil
}

func (dm *DirModel) updateSearchQuery() {
	if dm.searchIndex == nil {
		dm.searchMatches = nil
		return
	}
	query := dm.searchInput.Value()
	dm.searchMatches = dm.searchIndex.Find(query)
	dm.searchCursor = 0
	dm.searchOffset = 0
}

// SelectedSearchMatch returns the currently highlighted search match, if any.
func (dm *DirModel) SelectedSearchMatch() *search.Match {
	if dm.mode != SEARCH || dm.searchCursor < 0 || dm.searchCursor >= len(dm.searchMatches) {
		return nil
	}
	return &dm.searchMatches[dm.searchCursor]
}

// applySearchResult navigates to the selected search result and positions the
// cursor accordingly. It closes the global search overlay.
func (dm *DirModel) applySearchResult() {
	match := dm.SelectedSearchMatch()
	if match == nil {
		dm.closeGlobalSearch()
		return
	}

	target := match.Item.Entry
	if target == nil {
		dm.closeGlobalSearch()
		return
	}

	wasTreeMode := dm.treeMode
	wasTreemapMode := dm.treemapMode

	if wasTreeMode {
		// Tree mode should keep the project root as the tree root. Expand the
		// path from the root to the target and select the corresponding row so
		// the full project tree stays visible.
		dm.nav.entry = dm.nav.tree.Root()
		dm.nav.entryStack = &entryStack{}
		dm.nav.cursor = 0
		dm.expandPathFromRootTo(target)

		dm.closeGlobalSearch()
		dm.updateTableData(true)

		idx := dm.findChildIndex(target)
		if idx >= 0 && idx < len(dm.tableEntries) {
			dm.nav.cursor = idx
			dm.dirsTable.SetCursor(idx)
		}
		return
	}

	// NavigateToPath returns the target entry and leaves the navigation at the parent directory.
	found := dm.nav.NavigateToPath(match.Item.Path)
	if found == nil {
		dm.closeGlobalSearch()
		return
	}

	var childToSelect *structure.Entry
	if found.IsDir {
		// Enter the directory so its children fill the canvas/table.
		dm.nav.Down(found.Name(), 0, 0)
	} else {
		// Navigation is already at the parent directory; remember the child to select.
		childToSelect = found
	}

	dm.closeGlobalSearch()
	dm.updateTableData(true)

	if wasTreemapMode {
		// Defer block selection until viewTreemap has laid out the blocks.
		dm.pendingSearchTarget = found
	} else {
		// After the table has been rebuilt, position the cursor on the target row.
		if childToSelect != nil {
			idx := dm.findChildIndex(childToSelect)
			if idx >= 0 && idx < len(dm.tableEntries) {
				dm.nav.cursor = idx
				dm.dirsTable.SetCursor(idx)
			}
		}
	}
}

// expandPathFromRootTo expands every directory on the path from the project
// root down to the given target entry so that the target row becomes visible
// in tree mode. The navigation entry is left at the project root.
func (dm *DirModel) expandPathFromRootTo(target *structure.Entry) {
	root := dm.nav.tree.Root()
	if root == nil || target == nil {
		return
	}
	root.Expanded = true
	if root == target {
		return
	}

	relPath, err := filepath.Rel(root.Path, target.Path)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "" || relPath == "." {
		return
	}

	parts := strings.Split(relPath, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			continue
		}
		child := current.GetChild(part)
		if child == nil || !child.IsDir {
			return
		}
		child.Expanded = true
		current = child
	}
	if target.IsDir {
		target.Expanded = true
	}
}

// findChildIndex returns the table index of the given entry within the current
// directory's visible children. Returns -1 if not found.
func (dm *DirModel) findChildIndex(target *structure.Entry) int {
	if target == nil {
		return -1
	}
	for i, te := range dm.tableEntries {
		if te.entry == target {
			return i
		}
	}
	return -1
}

func (dm *DirModel) viewSearchOverlay(bg string) string {
	if dm.width < 20 || dm.height < 10 {
		return bg
	}

	// Box dimensions include border and padding. Content fits inside with a
	// 2-cell horizontal and vertical margin reserved for border + padding.
	boxWidth := min(dm.width-4, 100)
	boxHeight := min(dm.height-4, 30)
	if boxWidth < 20 {
		boxWidth = min(dm.width, 20)
	}
	if boxHeight < 8 {
		boxHeight = min(dm.height, 8)
	}

	// Inner width is the visible content area inside border + padding.
	innerWidth := boxWidth - 4

	// Size the text input so prompt + value fits within the inner width.
	promptWidth := lipgloss.Width(dm.searchInput.Prompt)
	dm.searchInput.Width = max(1, innerWidth-promptWidth)
	inputView := dm.searchInput.View()

	// Content lines: title(1) + description(1) + input(1) + results + status(1).
	// Box height = content lines + padding(2) + border(2) => results = boxHeight - 8.
	resultHeight := boxHeight - 8
	if resultHeight < 1 {
		resultHeight = 1
	}

	var resultLines []string
	if len(dm.searchMatches) == 0 {
		if dm.searchInput.Value() == "" {
			resultLines = append(resultLines, lipgloss.NewStyle().Faint(true).Render("Type to search files and directories"))
		} else {
			resultLines = append(resultLines, lipgloss.NewStyle().Faint(true).Render("No matches"))
		}
	} else {
		// Keep the cursor visible.
		if dm.searchCursor < dm.searchOffset {
			dm.searchOffset = dm.searchCursor
		}
		if dm.searchCursor >= dm.searchOffset+resultHeight {
			dm.searchOffset = dm.searchCursor - resultHeight + 1
		}
		if dm.searchOffset < 0 {
			dm.searchOffset = 0
		}

		end := dm.searchOffset + resultHeight
		if end > len(dm.searchMatches) {
			end = len(dm.searchMatches)
		}

		for i := dm.searchOffset; i < end; i++ {
			match := dm.searchMatches[i]
			line := dm.renderSearchResult(match, i == dm.searchCursor, innerWidth)
			resultLines = append(resultLines, line)
		}
	}

	// Pad result lines to keep the box height stable.
	for len(resultLines) < resultHeight {
		resultLines = append(resultLines, "")
	}

	status := fmt.Sprintf("%d/%d", dm.searchCursor+1, len(dm.searchMatches))
	if len(dm.searchMatches) == 0 {
		status = "0/0"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3a86ff"))
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(boxWidth).
		Height(boxHeight)

	content := []string{
		titleStyle.Render("Global Search"),
		lipgloss.NewStyle().Faint(true).Render("Ctrl+P: open • Enter: jump • Esc: close"),
		inputView,
		lipgloss.JoinVertical(lipgloss.Top, resultLines...),
		lipgloss.NewStyle().Faint(true).Align(lipgloss.Right).Render(status),
	}

	box := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Top, content...))
	dm.overlayBounds = overlayBounds{
		kind: "search",
		x:    dm.width/2 - lipgloss.Width(box)/2,
		y:    dm.height/2 - lipgloss.Height(box)/2,
		w:    lipgloss.Width(box),
		h:    lipgloss.Height(box),
	}

	return OverlayCenter(dm.width, dm.height, bg, box)
}

// byteIndexesToRuneIndexes converts byte offsets into rune offsets for the
// given string. The fuzzy package returns byte indexes, while lipgloss.StyleRunes
// expects rune indexes.
func byteIndexesToRuneIndexes(s string, byteIdxs []int) []int {
	runeIdxs := make([]int, len(byteIdxs))
	for i, bi := range byteIdxs {
		if bi < 0 || bi > len(s) {
			continue
		}
		runeIdxs[i] = utf8.RuneCountInString(s[:bi])
	}
	return runeIdxs
}

func (dm *DirModel) renderSearchResult(match search.Match, selected bool, maxWidth int) string {
	path := match.Item.Path
	if path == "." {
		path = dm.nav.tree.Root().Path
		if path == "" || path == "." {
			path = "(root)"
		}
	}

	icon := EntryIcon(match.Item.Entry)
	prefix := icon + "  "
	prefixWidth := lipgloss.Width(prefix)
	available := maxWidth - prefixWidth
	if available < 5 {
		available = 5
	}

	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fb5607"))
	plainStyle := lipgloss.NewStyle()
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("240"))

	// The fuzzy package returns byte indexes; StyleRunes expects rune indexes.
	runeIndexes := byteIndexesToRuneIndexes(path, match.MatchedIndexes)
	renderedPath := lipgloss.StyleRunes(path, runeIndexes, highlightStyle, plainStyle)

	// Truncate the highlighted path if it exceeds the available width.
	if lipgloss.Width(renderedPath) > available {
		renderedPath = ansi.Truncate(renderedPath, available-1, "…")
	}

	line := prefix + renderedPath

	if selected {
		// Pad the line to fill the inner width so the selection background is consistent.
		pad := maxWidth - lipgloss.Width(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		line = selectedStyle.Render(line)
	}

	return ansi.Truncate(line, maxWidth, "")
}

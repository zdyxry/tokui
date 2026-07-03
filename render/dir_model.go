package render

import (
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/search"
	"github.com/zdyxry/tokui/structure"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tableEntry struct {
	entry    *structure.Entry
	depth    int
	isParent bool
}

type DirModel struct {
	columns       []Column
	dirsTable     *table.Model
	nav           *Navigation
	filters       filter.FiltersList
	mode          Mode
	height        int
	width         int
	fullHelp      bool
	showCart      bool
	languages     []string
	langFilterIdx int // -1 represents "All", 0+ represents index in languages slice
	filePreview   *FilePreview
	// Language select state
	selectMode          bool
	selectedLangs       map[string]bool
	selectLangsSnapshot map[string]bool
	selectIndex         int
	err          error
	providerInfo provider.Info
	tableEntries []*tableEntry
	treeMode     bool
	treemapMode  bool
	sortState    SortState

	// Treemap view state
	treemapBlocks      []treemapBlock
	treemapSelected    int
	treemapOffsetY     int // screen Y where the treemap canvas starts
	treemapColorByLang bool
	treemapSizeKey     SortKey

	// Global search state
	searchIndex         *search.Index
	searchInput         textinput.Model
	searchMatches       []search.Match
	searchCursor        int
	searchOffset        int
	pendingSearchTarget *structure.Entry // for treemap mode selection after render

	// Mouse support
	overlayBounds overlayBounds
	lastClick     mouseClick
	lastTableView string // cached table view from the last render
}

const tableHeaderHeight = 2 // TableHeaderStyle has BorderBottom and no padding

// NewDirModel creates and initializes a directory view model.
func NewDirModel(nav *Navigation, info provider.Info, treeMode, treemapMode bool) *DirModel {
	// Treemap and tree mode are mutually exclusive at the view level.
	if treemapMode {
		treeMode = false
	}

	// Define new column headers for the table. Optional metrics are appended
	// based on the Provider's advertised capabilities.
	columns := []Column{
		{Title: ""},                                    // Icon
		{Title: ""},                                    // Full path (hidden)
		{Title: "Name", SortKey: SortByName},           // Name
		{Title: "Languages", SortKey: SortByLanguages}, // Languages involved
		{Title: "Code", SortKey: SortByCode},           // Lines of code
		{Title: "Comments", SortKey: SortByComments},   // Comment lines
		{Title: "Blanks", SortKey: SortByBlanks},       // Blank lines
		{Title: "Total", SortKey: SortByTotal},         // Total lines
		{Title: "% of Parent", SortKey: SortByPercent}, // Percentage of parent directory
	}
	if info.Capabilities&provider.CapComplexity != 0 {
		columns = append(columns, Column{Title: "Complexity", SortKey: SortByComplexity})
	}

	// Keep only the name filter
	defaultFilters := []filter.EntryFilter{
		filter.NewNameFilter("Filter by name..."),
	}

	searchInput := newSearchInput()

	dm := &DirModel{
		columns:      columns,
		filters:      filter.NewFiltersList(defaultFilters...),
		dirsTable:    buildTable(),
		mode:         PENDING,
		nav:          nav,
		langFilterIdx: -1, // Default to show all languages
		selectMode:   false,
		selectedLangs: make(map[string]bool),
		selectIndex:  0,
		providerInfo: info,
		treeMode:     treeMode,
		treemapMode:  treemapMode,
		treemapSizeKey: SortByTotal,
		sortState:    SortState{Key: SortByTotal, Desc: true},
		searchInput:  searchInput,
	}

	return dm
}

func (dm *DirModel) ToggleTreeMode() {
	dm.treeMode = !dm.treeMode
	dm.updateTableData()
}

func (dm *DirModel) ToggleTreemapMode() {
	dm.treemapMode = !dm.treemapMode
	if dm.treemapMode {
		// Treemap and tree mode are mutually exclusive at the view level.
		dm.treeMode = false
	}
	dm.treemapSelected = 0
	dm.updateTableData()
}

func (dm *DirModel) Init() tea.Cmd {
	return nil
}

func (dm *DirModel) SelectedEntry() *structure.Entry {
	if dm.treemapMode {
		if dm.treemapSelected < 0 || dm.treemapSelected >= len(dm.treemapBlocks) {
			return nil
		}
		return dm.treemapBlocks[dm.treemapSelected].entry
	}

	cursor := dm.dirsTable.Cursor()
	if cursor < 0 || cursor >= len(dm.tableEntries) {
		return nil
	}
	if dm.tableEntries[cursor].isParent {
		return nil
	}
	return dm.tableEntries[cursor].entry
}

// IsParentSelected reports whether the synthetic ".." entry is selected.
// It always returns false in treemap mode because there is no parent row.
func (dm *DirModel) IsParentSelected() bool {
	if dm.treemapMode {
		return false
	}
	cursor := dm.dirsTable.Cursor()
	return cursor >= 0 && cursor < len(dm.tableEntries) && dm.tableEntries[cursor].isParent
}

func (dm *DirModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ScanFinished:
		dm.mode = READY
		dm.updateLanguages()
		dm.updateTableData(msg.ResetCursor)
		dm.searchIndex = search.BuildIndex(dm.nav.tree.Root())

	case CycleLangFilter:
		if len(dm.languages) > 0 {
			dm.langFilterIdx++
			// Cycle back to "All"
			if dm.langFilterIdx >= len(dm.languages) {
				dm.langFilterIdx = -1
			}
		}
		dm.updateTableData()
	case OpenFileInEditor:
		return dm, openFileWithEditor(msg.Path)
	case EditorFinished:
		if msg.Err != nil {
			return dm, func() tea.Msg {
				return ErrorMsg(msg)
			}
		}
		return dm, nil
	case ErrorMsg:
		dm.err = msg.Err
		return dm, nil

	case tea.WindowSizeMsg:
		dm.updateSize(msg.Width, msg.Height)
		dm.filters.Update(msg)
		var searchCmd tea.Cmd
		dm.searchInput, searchCmd = dm.searchInput.Update(msg)
		cmd = searchCmd

	case tea.KeyMsg:
		if dm.err != nil {
			dm.err = nil
			return dm, nil
		}
		// If in preview mode, handle preview-specific keys
		if dm.mode == PREVIEW && dm.filePreview != nil {
			key := parseBindingKey(msg).String()
			if key == "q" || key == "esc" {
				// Close file preview
				dm.mode = READY
				dm.filePreview = nil
				return dm, nil
			}
			// Pass other keys to the file preview for scrolling
			_, cmd = dm.filePreview.Update(msg)
			return dm, cmd
		}

		// Handle key bindings, potentially returning a command
		cmd, handled := dm.handleKeyBindings(msg)
		if handled {
			return dm, cmd
		}
	}

	// Pass messages to the table to handle navigation (up/down movement, etc.)
	// Only update table if not in preview mode
	if dm.mode != PREVIEW {
		t, _ := dm.dirsTable.Update(msg)
		dm.dirsTable = &t
	}

	return dm, cmd
}

func (dm *DirModel) View() string {
	h := lipgloss.Height

	// Language select overlay
	if dm.mode == SELECT_LANG {
		var lines []string
		title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3a86ff")).Render("Select Languages")
		desc := lipgloss.NewStyle().Faint(true).Render("Space: toggle, Enter: confirm, Esc: cancel")
		lines = append(lines, title)
		lines = append(lines, desc)
		// Calculate visible window height (excluding title/desc, at least 2 lines)
		maxList := dm.height - 6
		if maxList < 2 {
			maxList = 2
		}
		start := 0
		end := len(dm.languages)
		if len(dm.languages) > maxList {
			// Ensure highlighted item is visible
			if dm.selectIndex < maxList/2 {
				start = 0
			} else if dm.selectIndex > len(dm.languages)-maxList/2 {
				start = len(dm.languages) - maxList
			} else {
				start = dm.selectIndex - maxList/2
			}
			end = start + maxList
			if end > len(dm.languages) {
				end = len(dm.languages)
			}
		}
		if start > 0 {
			lines = append(lines, lipgloss.NewStyle().Faint(true).Render("..."))
		}
		for i := start; i < end; i++ {
			lang := dm.languages[i]
			cursor := "  "
			if i == dm.selectIndex {
				cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#3a86ff")).Render("→ ")
			}
			var checked string
			if dm.selectedLangs[lang] {
				checked = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb5607")).Render("[x]")
			} else {
				checked = lipgloss.NewStyle().Faint(true).Render("[ ]")
			}
			langStr := lang
			if i == dm.selectIndex {
				langStr = lipgloss.NewStyle().Bold(true).Render(lang)
			}
			lines = append(lines, cursor+checked+" "+langStr)
		}
		if end < len(dm.languages) {
			lines = append(lines, lipgloss.NewStyle().Faint(true).Render("..."))
		}
		box := chartBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Top, lines...))
		boxW := lipgloss.Width(box)
		boxH := lipgloss.Height(box)
		dm.overlayBounds = overlayBounds{
			kind:      "langselect",
			x:         dm.width/2 - boxW/2,
			y:         dm.height/2 - boxH/2,
			w:         boxW,
			h:         boxH,
			langStart: start,
			langEnd:   end,
		}
		bg := lipgloss.NewStyle().Width(dm.width).Height(dm.height).Render(" ")
		return OverlayCenter(dm.width, dm.height, bg, box)
	}

	summary := dm.dirsSummary()
	keyBindings := dm.dirsTable.Help.ShortHelpView(shortHelp)
	if dm.fullHelp {
		keyBindings = dm.dirsTable.Help.FullHelpView(
			append(navigateKeyMap, dirsKeyMap...),
		)
	}

	// Calculate the available height for the main table
	dirsTableHeight := dm.height - h(keyBindings) - h(summary)

	rows := []string{keyBindings, summary}

	// If the name filter is active, reserve space for it
	var filterView string
	if f, ok := dm.filters[filter.NameFilterID].(filter.Viewer); ok {
		filterView = f.View()
		if len(filterView) > 0 {
			dirsTableHeight -= h(filterView)
			rows = append(rows, filterView)
		}
	}

	// The main content is rendered at the top of the screen (rows are reversed
	// before joining), so the treemap canvas starts at y=0.
	dm.treemapOffsetY = 0

	var mainView string
	if dm.treemapMode {
		mainView = dm.viewTreemap(dirsTableHeight)
	} else {
		dm.dirsTable.SetHeight(dirsTableHeight)
		dm.lastTableView = dm.dirsTable.View()
		mainView = dm.lastTableView
	}
	rows = append(rows, mainView)

	slices.Reverse(rows)

	bg := lipgloss.JoinVertical(lipgloss.Top, rows...)

	// If in preview mode, overlay the file preview
	if dm.mode == PREVIEW && dm.filePreview != nil {
		preview := dm.filePreview.View()
		dm.overlayBounds = overlayBounds{
			kind: "preview",
			x:    dm.width/2 - dm.filePreview.width/2,
			y:    dm.height/2 - dm.filePreview.height/2,
			w:    dm.filePreview.width,
			h:    dm.filePreview.height,
		}
		return OverlayCenter(dm.width, dm.height, bg, preview)
	}

	// If needed, overlay the chart display
	if dm.showCart {
		chart := dm.viewChart()
		chartW := lipgloss.Width(chart)
		chartH := lipgloss.Height(chart)
		dm.overlayBounds = overlayBounds{
			kind: "chart",
			x:    dm.width/2 - chartW/2,
			y:    dm.height/2 - chartH/2,
			w:    chartW,
			h:    chartH,
		}
		return OverlayCenter(dm.width, dm.height, bg, chart)
	}

	if dm.err != nil {
		errorView := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("Error: %v", dm.err))
		return OverlayCenter(dm.width, dm.height, bg, errorView)
	}

	if dm.mode == SEARCH {
		return dm.viewSearchOverlay(bg)
	}

	return bg
}

func (dm *DirModel) handleKeyBindings(msg tea.KeyMsg) (tea.Cmd, bool) {
	if dm.mode == PENDING {
		return nil, false
	}

	bk := parseBindingKey(msg)

	// Language select mode
	if dm.mode == SELECT_LANG {
		switch bk {
		case cancel:
			return nil, false
		case escape, toggleLangSelect, quit:
			// Cancel: revert any unconfirmed language selections made this session.
			dm.selectedLangs = copyLangSelection(dm.selectLangsSnapshot)
			dm.selectLangsSnapshot = nil
			dm.mode = READY
			dm.selectMode = false
			dm.updateTableData()
			return nil, true
		case "up", "k":
			if dm.selectIndex > 0 {
				dm.selectIndex--
			}
			return nil, true
		case "down", "j":
			if dm.selectIndex < len(dm.languages)-1 {
				dm.selectIndex++
			}
			return nil, true
		case " ":
			if len(dm.languages) > 0 && dm.selectIndex < len(dm.languages) {
				lang := dm.languages[dm.selectIndex]
				dm.selectedLangs[lang] = !dm.selectedLangs[lang]
			}
			return nil, true
		case enter:
			// Confirm: keep current selections and clear the snapshot.
			dm.selectLangsSnapshot = nil
			dm.mode = READY
			dm.selectMode = false
			dm.updateTableData()
			return nil, true
		default:
			return nil, true
		}
	}

	// Quick search (/ key): activate name filter mode when not already filtering.
	// When in INPUT mode, let "/" pass through as a normal filter character.
	if bk == quickSearch && dm.mode != INPUT {
		dm.mode = INPUT
		// If the filter is not enabled, enable it
		if f, ok := dm.filters[filter.NameFilterID].(*filter.NameFilter); ok {
			if !f.IsEnabled() {
				dm.filters.ToggleFilter(filter.NameFilterID)
			}
			f.ClearInput() // Only clear input content, don't disable the filter
		}
		dm.updateTableData()
		return nil, true
	}

	// Global search (Ctrl+P): activate fuzzy project-wide search
	if bk == globalSearch {
		dm.openGlobalSearch()
		return nil, true
	}

	// If in input mode, handle special keys
	if dm.mode == INPUT {
		// Escape key exits input mode
		if bk == escape {
			dm.mode = READY
			// If the filter is enabled, disable it
			if f, ok := dm.filters[filter.NameFilterID].(*filter.NameFilter); ok && f.IsEnabled() {
				dm.filters.ToggleFilter(filter.NameFilterID) // Turn off filter
			}
			dm.updateTableData()
			return nil, true
		}
		// Enter key in filter mode is not handled, let the upper layer handle navigation
		if bk == enter {
			return nil, false // Let the upper ViewModel handle navigation
		}
		// Other keys are passed to the filter for input processing
		dm.filters.Update(msg)
		dm.updateTableData()
		return nil, true
	}

	// Global search mode.
	if dm.mode == SEARCH {
		switch bk {
		case escape:
			dm.closeGlobalSearch()
			return nil, true
		case enter:
			// Handled by ViewModel to perform navigation.
			return nil, false
		case "up", "k":
			if dm.searchCursor > 0 {
				dm.searchCursor--
			}
			return nil, true
		case "down", "j":
			if dm.searchCursor < len(dm.searchMatches)-1 {
				dm.searchCursor++
			}
			return nil, true
		case "pgup":
			dm.searchCursor -= 10
			if dm.searchCursor < 0 {
				dm.searchCursor = 0
			}
			return nil, true
		case "pgdown":
			if len(dm.searchMatches) > 0 {
				dm.searchCursor += 10
				if dm.searchCursor >= len(dm.searchMatches) {
					dm.searchCursor = len(dm.searchMatches) - 1
				}
			}
			return nil, true
		case "home", "g":
			dm.searchCursor = 0
			return nil, true
		case "end", "G":
			if len(dm.searchMatches) > 0 {
				dm.searchCursor = len(dm.searchMatches) - 1
			}
			return nil, true
		}

		// Pass other keys to the search input for typing.
		var searchCmd tea.Cmd
		dm.searchInput, searchCmd = dm.searchInput.Update(msg)
		dm.updateSearchQuery()
		return searchCmd, true
	}

	// Treemap-specific navigation and toggling.
	if dm.treemapMode {
		switch bk {
		case "up", "k":
			dm.moveTreemapSelection(-1)
			return nil, true
		case "down", "j":
			dm.moveTreemapSelection(1)
			return nil, true
		case toggleTree:
			// Switch from treemap view back to tree table view.
			dm.treemapMode = false
			dm.treeMode = true
			dm.updateTableData()
			return nil, true
		case toggleTreemap:
			dm.ToggleTreemapMode()
			return nil, true
		case toggleTreemapColor:
			dm.treemapColorByLang = !dm.treemapColorByLang
			dm.updateTableData()
			return nil, true
		case cycleTreemapSize:
			dm.cycleTreemapSize()
			dm.updateTableData()
			return nil, true
		}
	}

	// Handle other shortcuts
	switch bk {
	case editFile:
		entry := dm.SelectedEntry()
		if entry != nil && !entry.IsDir {
			cmd := func() tea.Msg {
				return OpenFileInEditor{Path: entry.Path}
			}
			return cmd, true
		}

	case toggleLangSelect:
		dm.mode = SELECT_LANG
		dm.selectMode = true
		dm.selectIndex = 0
		dm.selectLangsSnapshot = copyLangSelection(dm.selectedLangs)
		return nil, true
	case toggleLangFilter:
		// Send message to toggle language filter
		dm.Update(CycleLangFilter{})
		return nil, true
	case toggleChart:
		dm.showCart = !dm.showCart
		return nil, true
	case toggleHelp:
		dm.fullHelp = !dm.fullHelp
		return nil, true
	case toggleTree:
		dm.ToggleTreeMode()
		return nil, true
	case toggleTreemap:
		dm.ToggleTreemapMode()
		return nil, true
	case cycleSortColumn:
		dm.cycleSortColumn()
		dm.updateTableData()
		return nil, true
	case toggleSortOrder:
		dm.toggleSortOrder()
		dm.updateTableData()
		return nil, true
	}

	return nil, false
}

func (dm *DirModel) dirsSummary() string {
	if dm.nav.Entry() == nil {
		return ""
	}

	currentStats := dm.comparableStats(dm.nav.Entry())

	modeStr := "Nav"
	if dm.treeMode {
		modeStr = "Tree"
	}
	if dm.treemapMode {
		modeStr = "Treemap"
	}

	codeStr := formatNumber(currentStats.Code)
	metricName := "TOTAL"
	metricValue := currentStats.Total()
	if dm.treemapMode {
		switch dm.treemapSizeKey {
		case SortByComplexity:
			metricName = "COMPLEXITY"
			metricValue = currentStats.Complexity
		}
	}
	metricStr := formatNumber(metricValue)
	if currentStats.Total() > 0 {
		codeStr = fmt.Sprintf("%s (%d%%)", codeStr, currentStats.Code*100/currentStats.Total())
	}

	// Build the status bar from most to least important. Lower-priority items
	// are hidden on narrow terminals so the path and core metrics remain readable.
	const (
		showVersionMinWidth = 110
		showSortMinWidth    = 130
	)

	items := make([]*BarItem, 0, 12)

	if dm.width >= showVersionMinWidth {
		version := dm.providerInfo.Version
		if version == "" {
			version = "unknown"
		}
		items = append(items, NewBarItem(fmt.Sprintf("%s %s", dm.providerInfo.Name, version), "#8338ec", 0))
	}

	items = append(items,
		NewBarItem("PATH", "#FF5F87", 0),
		NewBarItem(dm.nav.Entry().Path, "", -1),
		NewBarItem("MODE", "#06b6d4", 0),
		NewBarItem(modeStr, "", 0),
		NewBarItem("LANG", "#3a86ff", 0),
		NewBarItem(dm.statusLangLabel(), "", 0),
	)

	if dm.treemapMode && dm.width >= showSortMinWidth {
		colorMode := "dir"
		if dm.treemapColorByLang {
			colorMode = "lang"
		}
		items = append(items,
			NewBarItem("COLOR", "#8338ec", 0),
			NewBarItem(colorMode, "", 0),
		)
	}

	if dm.width >= showSortMinWidth {
		items = append(items,
			NewBarItem("SORT", "#14b8a6", 0),
			NewBarItem(fmt.Sprintf("%s %s", dm.sortState.Key, dm.sortState.DirectionArrow()), "", 0),
		)
	}

	items = append(items,
		NewBarItem("CODE", "#fb5607", 0),
		DefaultBarItem(codeStr),
		NewBarItem(metricName, "#ffbe0b", 0),
		DefaultBarItem(metricStr),
	)

	return statusBarStyle.Margin(1, 0, 0, 0).Render(NewStatusBar(items, dm.width))
}

// filteredChildren returns the current directory's children after applying
// active filters and sorting. It is used by both the table view and the
// treemap view.
func (dm *DirModel) filteredChildren() []*structure.Entry {
	if dm.nav.Entry() == nil || !dm.nav.Entry().IsDir {
		return nil
	}

	children := dm.nav.Entry().Child
	result := make([]*structure.Entry, 0, len(children))
	for _, child := range children {
		if !dm.filters.Valid(child) {
			continue
		}
		if dm.useMultiLangFilter() {
			activeLangs := dm.selectedLangsList()
			has := false
			for _, lang := range activeLangs {
				if s := child.GetStats(lang); s.Total() > 0 {
					has = true
					break
				}
			}
			if !has {
				continue
			}
			stats := dm.comparableStats(child)
			if stats.Total() == 0 {
				continue
			}
		} else {
			stats := dm.comparableStats(child)
			if dm.activeLang() != "" && stats.Total() == 0 {
				continue
			}
		}
		result = append(result, child)
	}

	if len(result) > 1 {
		cmpFn := dm.buildChildComparator()
		slices.SortFunc(result, cmpFn)
	}

	return result
}

func (dm *DirModel) updateSize(width, height int) {
	dm.width, dm.height = width, height
	dm.dirsTable.SetWidth(width)
	dm.updateTableData()
}

func (dm *DirModel) ExitSearchMode() {
	if dm.mode == INPUT {
		dm.mode = READY
		if f, ok := dm.filters[filter.NameFilterID].(*filter.NameFilter); ok && f.IsEnabled() {
			dm.filters.ToggleFilter(filter.NameFilterID) // Close filter
		}
		dm.updateTableData()
	}
}

// ShowFilePreview creates and shows a file preview
func (dm *DirModel) ShowFilePreview(filePath string) {
	if dm.mode == PREVIEW {
		return // Already in preview mode
	}

	dm.filePreview = NewFilePreview(filePath, dm.width, dm.height)
	dm.mode = PREVIEW
}

// IsInPreviewMode returns true if currently showing file preview
func (dm *DirModel) IsInPreviewMode() bool {
	return dm.mode == PREVIEW
}

// ClosePreview closes the file preview and returns to the directory view.
func (dm *DirModel) ClosePreview() {
	dm.mode = READY
	dm.filePreview = nil
}

func openFileWithEditor(filePath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default to vim
	}

	cmd := exec.Command(editor, filePath)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinished{Err: err}
	})
}

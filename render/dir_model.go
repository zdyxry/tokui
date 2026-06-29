package render

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/search"
	"github.com/zdyxry/tokui/structure"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

type Mode string

const (
	PENDING Mode = "PENDING"
	READY   Mode = "READY"
	INPUT   Mode = "INPUT"
	PREVIEW Mode = "PREVIEW"
	SEARCH  Mode = "SEARCH"
)

const (
	SELECT_LANG Mode = "SELECT_LANG"
)

type CycleLangFilter struct{}

// OpenFileInEditor represents a message to open a file in the default editor.
type OpenFileInEditor struct {
	Path string
}

// EditorFinished represents a message that the editor has finished.
type EditorFinished struct {
	Err error
}

// ErrorMsg represents a message containing an error.
type ErrorMsg struct {
	Err error
}

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
	selectMode    bool
	selectedLangs map[string]bool
	selectIndex   int
	err           error
	tokeiVersion  string
	tableEntries  []*tableEntry
	treeMode      bool
	treemapMode   bool
	sortState     SortState

	// Treemap view state
	treemapBlocks   []treemapBlock
	treemapSelected int
	treemapOffsetY  int // screen Y where the treemap canvas starts

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

const tableHeaderHeight = 2 // TableHeaderStyle has BorderBottom and no padding

// NewDirModel creates and initializes a directory view model.
func NewDirModel(nav *Navigation, tokeiVersion string, treeMode, treemapMode bool) *DirModel {
	// Treemap and tree mode are mutually exclusive at the view level.
	if treemapMode {
		treeMode = false
	}

	// Define new column headers for the table
	columns := []Column{
		{Title: ""},                          // Icon
		{Title: ""},                          // Full path (hidden)
		{Title: "Name", SortKey: SortByName}, // Name
		{Title: "Languages", SortKey: SortByLanguages}, // Languages involved
		{Title: "Code", SortKey: SortByCode},           // Lines of code
		{Title: "Comments", SortKey: SortByComments},   // Comment lines
		{Title: "Blanks", SortKey: SortByBlanks},       // Blank lines
		{Title: "Total", SortKey: SortByTotal},         // Total lines
		{Title: "% of Parent", SortKey: SortByPercent}, // Percentage of parent directory
	}

	// Keep only the name filter
	defaultFilters := []filter.EntryFilter{
		filter.NewNameFilter("Filter by name..."),
	}

	searchInput := newSearchInput()

	dm := &DirModel{
		columns:       columns,
		filters:       filter.NewFiltersList(defaultFilters...),
		dirsTable:     buildTable(),
		mode:          PENDING,
		nav:           nav,
		langFilterIdx: -1, // Default to show all languages
		selectMode:    false,
		selectedLangs: make(map[string]bool),
		selectIndex:   0,
		tokeiVersion:  tokeiVersion,
		treeMode:      treeMode,
		treemapMode:   treemapMode,
		sortState:     SortState{Key: SortByTotal, Desc: true},
		searchInput:   searchInput,
	}

	return dm
}

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
		case escape, toggleLangSelect:
			dm.mode = READY
			dm.selectMode = false
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
			dm.mode = READY
			dm.selectMode = false
			dm.updateTableData()
			return nil, true
		default:
			return nil, true
		}
	}

	// Quick search (/ key): activate name filter mode
	if bk == quickSearch {
		if dm.mode != INPUT {
			dm.mode = INPUT
			// If the filter is not enabled, enable it
			if f, ok := dm.filters[filter.NameFilterID].(*filter.NameFilter); ok {
				if !f.IsEnabled() {
					dm.filters.ToggleFilter(filter.NameFilterID)
				}
				f.ClearInput() // Only clear input content, don't disable the filter
			}
			dm.updateTableData()
		}
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

// updateLanguages collects available languages from current and child entries
func (dm *DirModel) updateLanguages() {
	if dm.nav.Entry() == nil {
		return
	}
	langs := make(map[string]struct{})
	for lang := range dm.nav.Entry().StatsByLang {
		langs[lang] = struct{}{}
	}
	for _, child := range dm.nav.Entry().Child {
		for lang := range child.StatsByLang {
			langs[lang] = struct{}{}
		}
	}

	dm.languages = make([]string, 0, len(langs))
	for lang := range langs {
		dm.languages = append(dm.languages, lang)
	}
	sort.Strings(dm.languages)
}

func (dm *DirModel) formatTreeName(entry *structure.Entry, depth int) string {
	var prefix string
	if entry.IsDir {
		if entry.Expanded {
			prefix = "▾ "
		} else if entry.HasChild() {
			prefix = "▸ "
		} else {
			prefix = "  "
		}
	} else {
		prefix = "  "
	}
	indent := strings.Repeat("  ", depth)
	return indent + prefix + entry.Name()
}

// useMultiLangFilter returns true when one or more languages are selected
// via the multi-language selection overlay.
func (dm *DirModel) useMultiLangFilter() bool {
	if dm.selectedLangs == nil {
		return false
	}
	for _, lang := range dm.languages {
		if dm.selectedLangs[lang] {
			return true
		}
	}
	return false
}

// activeLang returns the currently cycled single-language filter value.
// It returns "" when "All" is selected or when multi-language filter is active.
func (dm *DirModel) activeLang() string {
	if dm.useMultiLangFilter() {
		return ""
	}
	if dm.langFilterIdx > -1 && dm.langFilterIdx < len(dm.languages) {
		return dm.languages[dm.langFilterIdx]
	}
	return ""
}

// selectedLangs returns the list of languages selected in multi-select mode.
func (dm *DirModel) selectedLangsList() []string {
	if dm.selectedLangs == nil {
		return nil
	}
	langs := make([]string, 0)
	for _, lang := range dm.languages {
		if dm.selectedLangs[lang] {
			langs = append(langs, lang)
		}
	}
	return langs
}

// statusLangLabel returns the human-readable language filter label shown in
// the status bar: "All", the single filtered language, or the comma-separated
// list of selected languages.
func (dm *DirModel) statusLangLabel() string {
	if dm.useMultiLangFilter() {
		return strings.Join(dm.selectedLangsList(), ", ")
	}
	if lang := dm.activeLang(); lang != "" {
		return lang
	}
	return "All"
}

// comparableStats returns the CodeStats that should be used for both display
// and sorting under the current language filter. For single-language filter it
// returns that language's stats; for multi-language filter it aggregates the
// selected languages; otherwise it returns the entry's total stats.
func (dm *DirModel) comparableStats(e *structure.Entry) structure.CodeStats {
	if !dm.useMultiLangFilter() {
		return e.GetStats(dm.activeLang())
	}
	var sum structure.CodeStats
	for _, lang := range dm.selectedLangsList() {
		sum.Add(e.GetStats(lang))
	}
	return sum
}

// buildChildComparator returns a comparator for sorting child entries according
// to the current SortState and language filter.
func (dm *DirModel) buildChildComparator() func(a, b *structure.Entry) int {
	key := dm.sortState.Key
	desc := dm.sortState.Desc

	// Precompute filter state once to avoid repeated allocations during sorting.
	useMulti := dm.useMultiLangFilter()
	activeLang := dm.activeLang()
	selectedLangs := dm.selectedLangsList()

	getComparableStats := func(e *structure.Entry) structure.CodeStats {
		if !useMulti {
			return e.GetStats(activeLang)
		}
		var sum structure.CodeStats
		for _, lang := range selectedLangs {
			sum.Add(e.GetStats(lang))
		}
		return sum
	}

	cmpVal := func(a, b int64) int {
		if desc {
			return cmp.Compare(b, a)
		}
		return cmp.Compare(a, b)
	}
	cmpStr := func(a, b string) int {
		r := cmp.Compare(strings.ToLower(a), strings.ToLower(b))
		if desc {
			return -r
		}
		return r
	}

	switch key {
	case SortByName:
		return func(a, b *structure.Entry) int { return cmpStr(a.Name(), b.Name()) }
	case SortByLanguages:
		return func(a, b *structure.Entry) int {
			return cmpStr(strings.Join(a.Languages(), ", "), strings.Join(b.Languages(), ", "))
		}
	case SortByCode:
		return func(a, b *structure.Entry) int { return cmpVal(getComparableStats(a).Code, getComparableStats(b).Code) }
	case SortByComments:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Comments, getComparableStats(b).Comments)
		}
	case SortByBlanks:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Blanks, getComparableStats(b).Blanks)
		}
	case SortByTotal, SortByPercent:
		// SortByPercent is mathematically equivalent to SortByTotal because the
		// parent total is constant for all siblings being compared.
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Total(), getComparableStats(b).Total())
		}
	default:
		return func(a, b *structure.Entry) int { return cmpVal(a.TotalStats.Total(), b.TotalStats.Total()) }
	}
}

// cycleSortColumn advances to the next sortable column and resets the sort
// direction to the default for that column.
func (dm *DirModel) cycleSortColumn() {
	order := []SortKey{
		SortByName,
		SortByLanguages,
		SortByCode,
		SortByComments,
		SortByBlanks,
		SortByTotal,
		SortByPercent,
	}

	idx := -1
	for i, k := range order {
		if k == dm.sortState.Key {
			idx = i
			break
		}
	}
	idx = (idx + 1) % len(order)
	next := order[idx]
	dm.sortState = SortState{Key: next, Desc: defaultDescForSortKey(next)}
}

// toggleSortOrder flips the direction of the current sort column.
func (dm *DirModel) toggleSortOrder() {
	dm.sortState.Desc = !dm.sortState.Desc
}

// defaultDescForSortKey returns the default sort direction for a column:
// ascending for text columns, descending for numeric columns.
func defaultDescForSortKey(key SortKey) bool {
	switch key {
	case SortByName, SortByLanguages:
		return false
	default:
		return true
	}
}

// updateTableData updates the table rows based on current filters and state
func (dm *DirModel) updateTableData(resetCursor ...bool) {
	if dm.nav.Entry() == nil || !dm.nav.Entry().IsDir {
		return
	}

	shouldReset := false
	if len(resetCursor) > 0 {
		shouldReset = resetCursor[0]
	}

	// Sort child entries using the current column sort state.
	dm.nav.Entry().SortChildBy(dm.buildChildComparator())
	parentTotal := dm.nav.ParentTotalLines(dm.activeLang())

	dm.tableEntries = make([]*tableEntry, 0)
	rows := make([]table.Row, 0)
	maxNameWidth := lipgloss.Width(dm.columns[2].Title)
	tempLangsWidth := 24

	if dm.treeMode {
		var addEntry func(entry *structure.Entry, depth int)
		addEntry = func(entry *structure.Entry, depth int) {
			if !dm.filters.Valid(entry) {
				return
			}
			if dm.useMultiLangFilter() {
				activeLangs := dm.selectedLangsList()
				has := false
				for _, lang := range activeLangs {
					if s := entry.GetStats(lang); s.Total() > 0 {
						has = true
						break
					}
				}
				if !has {
					return
				}
				stats := dm.comparableStats(entry)
				total := stats.Total()
				if total == 0 {
					return
				}
				name := dm.formatTreeName(entry, depth)
				if lipgloss.Width(name) > maxNameWidth {
					maxNameWidth = lipgloss.Width(name)
				}
				percent := 0.0
				if parentTotal > 0 {
					percent = (float64(total) / float64(parentTotal)) * 100
				}
				langStr := strings.Join(activeLangs, ", ")
				if lipgloss.Width(langStr) > tempLangsWidth {
					langStr = fmtName(langStr, tempLangsWidth)
					pad := tempLangsWidth - lipgloss.Width(langStr)
					if pad > 0 {
						langStr += strings.Repeat(" ", pad)
					}
				}
				rows = append(rows, table.Row{
					EntryIcon(entry),
					entry.Path,
					name,
					langStr,
					strconv.FormatInt(stats.Code, 10),
					strconv.FormatInt(stats.Comments, 10),
					strconv.FormatInt(stats.Blanks, 10),
					strconv.FormatInt(total, 10),
					fmt.Sprintf("%.2f %%", percent),
				})
				dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: entry, depth: depth})
				if entry.IsDir && entry.Expanded {
					entry.SortChildBy(dm.buildChildComparator())
					for _, child := range entry.Child {
						addEntry(child, depth+1)
					}
				}
				return
			}
			stats := dm.comparableStats(entry)
			if dm.activeLang() != "" && stats.Total() == 0 {
				return
			}

			name := dm.formatTreeName(entry, depth)
			if lipgloss.Width(name) > maxNameWidth {
				maxNameWidth = lipgloss.Width(name)
			}

			percent := 0.0
			if parentTotal > 0 {
				percent = (float64(stats.Total()) / float64(parentTotal)) * 100
			}
			langStr := strings.Join(entry.Languages(), ", ")
			if lipgloss.Width(langStr) > tempLangsWidth {
				langStr = fmtName(langStr, tempLangsWidth)
			}
			rows = append(rows, table.Row{
				EntryIcon(entry),
				entry.Path,
				name,
				langStr,
				strconv.FormatInt(stats.Code, 10),
				strconv.FormatInt(stats.Comments, 10),
				strconv.FormatInt(stats.Blanks, 10),
				strconv.FormatInt(stats.Total(), 10),
				fmt.Sprintf("%.2f %%", percent),
			})
			dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: entry, depth: depth})

			if entry.IsDir && entry.Expanded {
				entry.SortChildBy(dm.buildChildComparator())
				for _, child := range entry.Child {
					addEntry(child, depth+1)
				}
			}
		}

		for _, child := range dm.nav.Entry().Child {
			addEntry(child, 0)
		}
	} else {
		// Add a synthetic ".." entry in navigation mode when not at the root.
		if dm.nav.entryStack.len() > 0 {
			parentEntry := &structure.Entry{Path: "..", IsDir: true}
			dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: parentEntry, isParent: true})
			rows = append(rows, table.Row{
				"⬆",
				"",
				"..",
				"",
				"0", "0", "0", "0", "",
			})
		}
		for _, child := range dm.nav.Entry().Child {
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
				total := stats.Total()
				if total == 0 {
					continue
				}
				name := child.Name()
				if lipgloss.Width(name) > maxNameWidth {
					maxNameWidth = lipgloss.Width(name)
				}
				percent := 0.0
				if parentTotal > 0 {
					percent = (float64(total) / float64(parentTotal)) * 100
				}
				langStr := strings.Join(activeLangs, ", ")
				if lipgloss.Width(langStr) > tempLangsWidth {
					langStr = fmtName(langStr, tempLangsWidth)
					pad := tempLangsWidth - lipgloss.Width(langStr)
					if pad > 0 {
						langStr += strings.Repeat(" ", pad)
					}
				}
				rows = append(rows, table.Row{
					EntryIcon(child),
					child.Path,
					name,
					langStr,
					strconv.FormatInt(stats.Code, 10),
					strconv.FormatInt(stats.Comments, 10),
					strconv.FormatInt(stats.Blanks, 10),
					strconv.FormatInt(total, 10),
					fmt.Sprintf("%.2f %%", percent),
				})
				dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: child, depth: 0})
				continue
			}
			stats := dm.comparableStats(child)
			if dm.activeLang() != "" && stats.Total() == 0 {
				continue
			}

			name := child.Name()
			if lipgloss.Width(name) > maxNameWidth {
				maxNameWidth = lipgloss.Width(name)
			}

			percent := 0.0
			if parentTotal > 0 {
				percent = (float64(stats.Total()) / float64(parentTotal)) * 100
			}
			langStr := strings.Join(child.Languages(), ", ")
			if lipgloss.Width(langStr) > tempLangsWidth {
				langStr = fmtName(langStr, tempLangsWidth)
			}
			rows = append(rows, table.Row{
				EntryIcon(child),
				child.Path,
				name,
				langStr,
				strconv.FormatInt(stats.Code, 10),
				strconv.FormatInt(stats.Comments, 10),
				strconv.FormatInt(stats.Blanks, 10),
				strconv.FormatInt(stats.Total(), 10),
				fmt.Sprintf("%.2f %%", percent),
			})
			dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: child, depth: 0})
		}
	}

	// --- Step 2: Calculate and set final column widths ---
	iconWidth := 4
	percentWidth := 14
	numericWidth := 12
	languagesWidth := 24

	nameWidth := maxNameWidth + 2

	// Ensure each column is wide enough for its titled (including sort indicator).
	minWidths := []int{
		iconWidth,
		0,
		nameWidth,
		languagesWidth,
		numericWidth,
		numericWidth,
		numericWidth,
		numericWidth,
		percentWidth,
	}
	for i, c := range dm.columns {
		titleWidth := lipgloss.Width(c.FmtName(dm.sortState))
		if titleWidth > minWidths[i] {
			minWidths[i] = titleWidth
		}
	}

	fixedWidths := minWidths[0] + minWidths[1] + minWidths[3] + minWidths[4] + minWidths[5] + minWidths[6] + minWidths[7] + minWidths[8]
	totalRequiredWidth := fixedWidths + minWidths[2]

	if totalRequiredWidth > dm.width {
		minWidths[2] = dm.width - fixedWidths
		if minWidths[2] < 20 {
			minWidths[2] = 20
		}
	}

	columns := make([]table.Column, len(dm.columns))
	for i, c := range dm.columns {
		columns[i] = table.Column{Title: c.FmtName(dm.sortState), Width: minWidths[i]}
	}

	dm.dirsTable.SetColumns(columns)
	dm.dirsTable.SetRows(rows)

	if len(rows) > 0 {
		if shouldReset && dm.nav.cursor < len(rows) {
			dm.dirsTable.SetCursor(dm.nav.cursor)
		} else {
			savedCursor := dm.dirsTable.Cursor()
			if savedCursor < len(rows) {
				dm.dirsTable.SetCursor(savedCursor)
			} else if dm.nav.cursor < len(rows) {
				dm.dirsTable.SetCursor(dm.nav.cursor)
			} else {
				dm.dirsTable.SetCursor(len(rows) - 1)
			}
		}
	}
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
	totalStr := formatNumber(currentStats.Total())
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
		items = append(items, NewBarItem(fmt.Sprintf("tokei %s", dm.tokeiVersion), "#8338ec", 0))
	}

	items = append(items,
		NewBarItem("PATH", "#FF5F87", 0),
		NewBarItem(dm.nav.Entry().Path, "", -1),
		NewBarItem("MODE", "#06b6d4", 0),
		NewBarItem(modeStr, "", 0),
		NewBarItem("LANG", "#3a86ff", 0),
		NewBarItem(dm.statusLangLabel(), "", 0),
	)

	if dm.width >= showSortMinWidth {
		items = append(items,
			NewBarItem("SORT", "#14b8a6", 0),
			NewBarItem(fmt.Sprintf("%s %s", dm.sortState.Key, dm.sortState.DirectionArrow()), "", 0),
		)
	}

	items = append(items,
		NewBarItem("CODE", "#fb5607", 0),
		DefaultBarItem(codeStr),
		NewBarItem("TOTAL", "#ffbe0b", 0),
		DefaultBarItem(totalStr),
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

func (dm *DirModel) viewTreemap(availableHeight int) string {
	children := dm.filteredChildren()
	if len(children) == 0 {
		dm.treemapBlocks = nil
		dm.treemapSelected = 0
		return treemapEmptyStyle.Render(" (no items to display)")
	}

	w := dm.width
	h := availableHeight
	if h < 3 {
		h = 3
	}

	getSize := func(e *structure.Entry) int64 {
		return dm.comparableStats(e).Total()
	}

	view, blocks := Treemap(w, h, children, getSize, dm.treemapSelected)
	dm.treemapBlocks = blocks

	// If a global search result was just applied in treemap mode, select the
	// corresponding block now that the blocks have been laid out.
	if dm.pendingSearchTarget != nil {
		idx := dm.findTreemapBlockIndex(dm.pendingSearchTarget)
		if idx >= 0 {
			dm.treemapSelected = idx
			view, blocks = Treemap(w, h, children, getSize, dm.treemapSelected)
			dm.treemapBlocks = blocks
		}
		dm.pendingSearchTarget = nil
	}

	if len(blocks) > 0 && dm.treemapSelected >= len(blocks) {
		dm.treemapSelected = len(blocks) - 1
		view, _ = Treemap(w, h, children, getSize, dm.treemapSelected)
	}
	return view
}

// moveTreemapSelection moves the keyboard selection among top-level (level 0)
// treemap blocks. Nested blocks can still be selected with the mouse, but j/k
// always stay at the current directory's immediate children.
func (dm *DirModel) moveTreemapSelection(delta int) {
	if len(dm.treemapBlocks) == 0 {
		return
	}

	// Collect indices of all top-level blocks in display order.
	topIdxs := make([]int, 0)
	for i, b := range dm.treemapBlocks {
		if b.level == 0 {
			topIdxs = append(topIdxs, i)
		}
	}
	if len(topIdxs) == 0 {
		return
	}

	// Find the current position among top-level blocks.
	currentTop := dm.treemapSelected
	if currentTop < 0 || currentTop >= len(dm.treemapBlocks) {
		currentTop = dm.treemapBlocks[topIdxs[0]].topIdx
	} else {
		currentTop = dm.treemapBlocks[currentTop].topIdx
	}

	pos := currentTop

	pos += delta
	if pos < 0 {
		pos = 0
	}
	if pos >= len(topIdxs) {
		pos = len(topIdxs) - 1
	}

	dm.treemapSelected = topIdxs[pos]
}

func (dm *DirModel) viewChart() string {
	chartSectors := make([]RawChartSector, 0, len(dm.nav.entry.StatsByLang))
	var totalCode float64
	for lang, stats := range dm.nav.entry.StatsByLang {
		if stats.Total() > 0 {
			chartSectors = append(chartSectors, RawChartSector{
				Label: lang,
				Value: float64(stats.Total()),
			})
			totalCode += float64(stats.Total())
		}
	}

	// Ensure the chart has a reasonable radius
	radius := min(dm.width/4, dm.height/4) - 2

	return chartBoxStyle.Render(
		Chart(
			dm.width/2,  // Chart area width
			dm.height/2, // Chart area height
			radius,
			totalCode,
			chartSectors,
		),
	)
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

// Global search helpers -----------------------------------------------------

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
		if wasTreemapMode {
			// For treemap, enter the directory so its children fill the canvas.
			dm.nav.Down(found.Name(), 0, 0)
		} else {
			// For navigation mode, enter the directory.
			dm.nav.Down(found.Name(), 0, 0)
		}
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

// findTreemapBlockIndex returns the index of the given entry in the current
// treemap block list. It returns -1 if not found.
func (dm *DirModel) findTreemapBlockIndex(target *structure.Entry) int {
	if target == nil {
		return -1
	}
	for i, b := range dm.treemapBlocks {
		if b.entry == target {
			return i
		}
	}
	return -1
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

// --- Mouse support ---------------------------------------------------------

const doubleClickThreshold = 300 * time.Millisecond

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

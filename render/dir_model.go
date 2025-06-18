package render

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/zdyxry/tokui/filter"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode string

const (
	PENDING Mode = "PENDING"
	READY   Mode = "READY"
	INPUT   Mode = "INPUT"
	PREVIEW Mode = "PREVIEW"
)

type CycleLangFilter struct{}

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
}

// NewDirModel creates and initializes a directory view model.
func NewDirModel(nav *Navigation) *DirModel {
	// Define new column headers for the table
	columns := []Column{
		{Title: ""},            // Icon
		{Title: ""},            // Full path (hidden)
		{Title: "Name"},        // Name
		{Title: "Languages"},   // Languages involved
		{Title: "Code"},        // Lines of code
		{Title: "Comments"},    // Comment lines
		{Title: "Blanks"},      // Blank lines
		{Title: "Total"},       // Total lines
		{Title: "% of Parent"}, // Percentage of parent directory
	}

	// Keep only the name filter
	defaultFilters := []filter.EntryFilter{
		filter.NewNameFilter("Filter by name..."),
	}

	dm := &DirModel{
		columns:       columns,
		filters:       filter.NewFiltersList(defaultFilters...),
		dirsTable:     buildTable(),
		mode:          PENDING,
		nav:           nav,
		langFilterIdx: -1, // Default to show all languages
	}

	return dm
}

func (dm *DirModel) Init() tea.Cmd {
	return nil
}

func (dm *DirModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ScanFinished:
		dm.mode = READY
		dm.updateLanguages()
		dm.updateTableData()

	case CycleLangFilter:
		if len(dm.languages) > 0 {
			dm.langFilterIdx++
			// Cycle back to "All"
			if dm.langFilterIdx >= len(dm.languages) {
				dm.langFilterIdx = -1
			}
		}
		dm.updateTableData()

	case tea.WindowSizeMsg:
		dm.updateSize(msg.Width, msg.Height)
		dm.filters.Update(msg)

	case tea.KeyMsg:
		// If in preview mode, handle preview-specific keys
		if dm.mode == PREVIEW && dm.filePreview != nil {
			key := strings.ToLower(msg.String())
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

		if dm.handleKeyBindings(msg) {
			return dm, nil
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

	dm.dirsTable.SetHeight(dirsTableHeight)
	rows = append(rows, dm.dirsTable.View())

	slices.Reverse(rows)

	bg := lipgloss.JoinVertical(lipgloss.Top, rows...)

	// If in preview mode, overlay the file preview
	if dm.mode == PREVIEW && dm.filePreview != nil {
		preview := dm.filePreview.View()
		return OverlayCenter(dm.width, dm.height, bg, preview)
	}

	// If needed, overlay the chart display
	if dm.showCart {
		chart := dm.viewChart()
		return OverlayCenter(dm.width, dm.height, bg, chart)
	}

	return bg
}

func (dm *DirModel) handleKeyBindings(msg tea.KeyMsg) bool {
	if dm.mode == PENDING {
		return false
	}

	bk := bindingKey(strings.ToLower(msg.String()))

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
		return true
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
			return true
		}
		// Enter key in filter mode is not handled, let the upper layer handle navigation
		if bk == enter {
			return false // Let the upper ViewModel handle navigation
		}
		// Other keys are passed to the filter for input processing
		dm.filters.Update(msg)
		dm.updateTableData()
		return true
	}

	// Handle other shortcuts
	switch bk {
	case toggleLangFilter:
		// Send message to toggle language filter
		dm.Update(CycleLangFilter{})
		return true
	case toggleChart:
		dm.showCart = !dm.showCart
		return true
	case toggleHelp:
		dm.fullHelp = !dm.fullHelp
		return true
	}

	return false
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

// updateTableData updates the table rows based on current filters and state
func (dm *DirModel) updateTableData() {
	if dm.nav.Entry() == nil || !dm.nav.Entry().IsDir {
		return
	}

	// --- [Core Modification] ---
	// Two-step update method:
	// 1. Preprocess data and calculate the maximum width needed for the Name column.
	// 2. Set all columns based on the calculated width, then populate data.

	// Get the currently active language filter
	var activeLang string
	if dm.langFilterIdx > -1 && dm.langFilterIdx < len(dm.languages) {
		activeLang = dm.languages[dm.langFilterIdx]
	}

	// Sort child entries by total lines
	dm.nav.Entry().SortChild()
	parentTotal := dm.nav.ParentTotalLines(activeLang)

	// --- Step 1: Preprocessing and calculating maximum width ---
	rows := make([]table.Row, 0, len(dm.nav.Entry().Child))
	// Initial maximum width is the width of the column header "Name"
	maxNameWidth := lipgloss.Width(dm.columns[2].Title)
	// Temporarily set a language column width for truncation, will use final value later
	tempLangsWidth := 24

	for _, child := range dm.nav.Entry().Child {
		if !dm.filters.Valid(child) {
			continue
		}
		stats := child.GetStats(activeLang)
		if activeLang != "" && stats.Total() == 0 {
			continue
		}

		// Update the maximum width for the Name column
		name := child.Name()
		if lipgloss.Width(name) > maxNameWidth {
			maxNameWidth = lipgloss.Width(name)
		}

		// Prepare row data (to be populated into the table later)
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
			child.Path, // Hidden
			name,
			langStr,
			strconv.FormatInt(stats.Code, 10),
			strconv.FormatInt(stats.Comments, 10),
			strconv.FormatInt(stats.Blanks, 10),
			strconv.FormatInt(stats.Total(), 10),
			fmt.Sprintf("%.2f %%", percent),
		})
	}

	// --- Step 2: Calculate and set final column widths ---
	iconWidth := 4
	percentWidth := 14
	numericWidth := 12
	languagesWidth := 24

	// The ideal width for the Name column is the maximum content width + 2
	nameWidth := maxNameWidth + 2

	// **Screen overflow handling**: Check if the ideal widths of all columns exceed the total screen width
	fixedWidths := iconWidth + languagesWidth + (4 * numericWidth) + percentWidth
	totalRequiredWidth := fixedWidths + nameWidth

	if totalRequiredWidth > dm.width {
		// If it exceeds, shrink the Name column to fit the screen
		nameWidth = dm.width - fixedWidths
		// Ensure the Name column has at least 20 characters width to prevent it from disappearing completely
		if nameWidth < 20 {
			nameWidth = 20
		}
	}

	// Set the final column configuration
	widths := []int{
		iconWidth, 0, nameWidth, languagesWidth, numericWidth,
		numericWidth, numericWidth, numericWidth, percentWidth,
	}
	columns := make([]table.Column, len(dm.columns))
	for i, c := range dm.columns {
		columns[i] = table.Column{Title: c.Title, Width: widths[i]}
	}

	dm.dirsTable.SetColumns(columns)
	dm.dirsTable.SetRows(rows) // Use preprocessed row data
	dm.dirsTable.SetCursor(dm.nav.cursor)
}

func (dm *DirModel) dirsSummary() string {
	if dm.nav.Entry() == nil {
		return ""
	}

	activeLang := "All"
	if dm.langFilterIdx > -1 && dm.langFilterIdx < len(dm.languages) {
		activeLang = dm.languages[dm.langFilterIdx]
	}
	currentStats := dm.nav.Entry().GetStats(activeLang)

	items := []*BarItem{
		NewBarItem(Version, "#8338ec", 0),
		NewBarItem("PATH", "#FF5F87", 0),
		NewBarItem(dm.nav.Entry().Path, "", -1),
		NewBarItem("LANG FILTER", "#3a86ff", 0),
		NewBarItem(activeLang, "", 0),
		NewBarItem("CODE", "#fb5607", 0),
		DefaultBarItem(strconv.FormatInt(currentStats.Code, 10)),
		NewBarItem("TOTAL", "#ffbe0b", 0),
		DefaultBarItem(strconv.FormatInt(currentStats.Total(), 10)),
	}
	return statusBarStyle.Margin(1, 0, 0, 0).Render(NewStatusBar(items, dm.width))
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

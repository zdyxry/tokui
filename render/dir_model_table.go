package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/zdyxry/tokui/structure"
)

// visibleColumns returns the columns that should be rendered given the current
// terminal width and sort key. Optional columns are hidden on narrow screens
// unless they are the active sort column.
func (dm *DirModel) visibleColumns() []Column {
	cols := make([]Column, 0, len(dm.columns))
	for _, c := range dm.columns {
		switch c.SortKey {
		case SortByComplexity:
			if dm.width < 80 && dm.sortState.Key != SortByComplexity {
				continue
			}
		case SortByLanguages, SortByComments, SortByBlanks:
			if dm.width < 60 && dm.sortState.Key != c.SortKey {
				continue
			}
		}
		cols = append(cols, c)
	}
	return cols
}

// columnMinWidth returns the minimum width for a visible column.
func columnMinWidth(c Column) int {
	switch c.SortKey {
	case SortByName:
		return 0 // computed from content
	case SortByLanguages:
		return 24
	case SortByPercent:
		return 14
	case SortByNone:
		return 0 // icon (handled explicitly) and hidden path column
	default:
		return 12 // numeric columns
	}
}

// buildRow creates a table row for the given entry using the supplied visible
// columns, display name, language string, stats and percentage.
// buildParentRow creates the synthetic ".." row shown in navigation mode.
func (dm *DirModel) buildParentRow(cols []Column) table.Row {
	row := make(table.Row, len(cols))
	for i, c := range cols {
		switch i {
		case 0:
			row[i] = "⬆"
		case 1:
			row[i] = ""
		default:
			switch c.SortKey {
			case SortByName:
				row[i] = ".."
			case SortByLanguages, SortByPercent:
				row[i] = ""
			default:
				row[i] = "0"
			}
		}
	}
	return row
}

func (dm *DirModel) buildRow(cols []Column, entry *structure.Entry, name, langStr string, stats structure.CodeStats, percent float64) table.Row {
	row := make(table.Row, len(cols))
	for i, c := range cols {
		switch i {
		case 0:
			row[i] = EntryIcon(entry)
		case 1:
			row[i] = entry.Path
		default:
			switch c.SortKey {
			case SortByName:
				row[i] = name
			case SortByLanguages:
				row[i] = langStr
			case SortByCode:
				row[i] = strconv.FormatInt(stats.Code, 10)
			case SortByComments:
				row[i] = strconv.FormatInt(stats.Comments, 10)
			case SortByBlanks:
				row[i] = strconv.FormatInt(stats.Blanks, 10)
			case SortByTotal:
				row[i] = strconv.FormatInt(stats.Total(), 10)
			case SortByPercent:
				row[i] = fmt.Sprintf("%.2f %%", percent)
			case SortByComplexity:
				row[i] = strconv.FormatInt(stats.Complexity, 10)
			default:
				row[i] = ""
			}
		}
	}
	return row
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
	parentTotal := dm.parentTotalForKey(dm.sortState.Key)

	cols := dm.visibleColumns()

	dm.tableEntries = make([]*tableEntry, 0)
	rows := make([]table.Row, 0)
	maxNameWidth := lipgloss.Width(cols[2].Title)
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
					percent = (float64(metricValue(stats, dm.sortState.Key)) / float64(parentTotal)) * 100
				}
				langStr := strings.Join(activeLangs, ", ")
				if lipgloss.Width(langStr) > tempLangsWidth {
					langStr = fmtName(langStr, tempLangsWidth)
					pad := tempLangsWidth - lipgloss.Width(langStr)
					if pad > 0 {
						langStr += strings.Repeat(" ", pad)
					}
				}
				rows = append(rows, dm.buildRow(cols, entry, name, langStr, stats, percent))
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
				percent = (float64(metricValue(stats, dm.sortState.Key)) / float64(parentTotal)) * 100
			}
			langStr := strings.Join(entry.Languages(), ", ")
			if lipgloss.Width(langStr) > tempLangsWidth {
				langStr = fmtName(langStr, tempLangsWidth)
			}
			rows = append(rows, dm.buildRow(cols, entry, name, langStr, stats, percent))
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
			rows = append(rows, dm.buildParentRow(cols))
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
					percent = (float64(metricValue(stats, dm.sortState.Key)) / float64(parentTotal)) * 100
				}
				langStr := strings.Join(activeLangs, ", ")
				if lipgloss.Width(langStr) > tempLangsWidth {
					langStr = fmtName(langStr, tempLangsWidth)
					pad := tempLangsWidth - lipgloss.Width(langStr)
					if pad > 0 {
						langStr += strings.Repeat(" ", pad)
					}
				}
				rows = append(rows, dm.buildRow(cols, child, name, langStr, stats, percent))
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
				percent = (float64(metricValue(stats, dm.sortState.Key)) / float64(parentTotal)) * 100
			}
			langStr := strings.Join(child.Languages(), ", ")
			if lipgloss.Width(langStr) > tempLangsWidth {
				langStr = fmtName(langStr, tempLangsWidth)
			}
			rows = append(rows, dm.buildRow(cols, child, name, langStr, stats, percent))
			dm.tableEntries = append(dm.tableEntries, &tableEntry{entry: child, depth: 0})
		}
	}

	// --- Step 2: Calculate and set final column widths ---
	nameWidth := maxNameWidth + 2

	// Compute minimum widths for each column based on its type.
	minWidths := make([]int, len(cols))
	for i, c := range cols {
		minWidths[i] = columnMinWidth(c)
		if i == 0 {
			minWidths[i] = max(minWidths[i], 4) // icon column
		}
		if i == 2 {
			minWidths[i] = nameWidth
		}
		titleWidth := lipgloss.Width(c.FmtName(dm.sortState))
		if titleWidth > minWidths[i] {
			minWidths[i] = titleWidth
		}
	}

	// Sum all fixed-width columns (everything except Name).
	fixedWidths := 0
	for i := range cols {
		if i != 2 {
			fixedWidths += minWidths[i]
		}
	}
	totalRequiredWidth := fixedWidths + minWidths[2]

	if totalRequiredWidth > dm.width {
		minWidths[2] = dm.width - fixedWidths
		if minWidths[2] < 20 {
			minWidths[2] = 20
		}
	}

	columns := make([]table.Column, len(cols))
	for i, c := range cols {
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

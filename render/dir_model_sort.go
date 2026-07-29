package render

import (
	"cmp"
	"strings"

	"github.com/zdyxry/tokui/structure"
)

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

	getComparableChange := func(e *structure.Entry) structure.Change {
		if !useMulti {
			return e.GetChange(activeLang)
		}
		var sum structure.Change
		for _, lang := range selectedLangs {
			sum.Add(e.GetChange(lang))
		}
		return sum
	}

	// Compare-mode deltas: S2 value minus the S1 (Prev) value.
	deltaCode := func(e *structure.Entry) int64 {
		return getComparableStats(e).Code - getComparableChange(e).PrevCode
	}
	deltaCmplx := func(e *structure.Entry) int64 {
		return getComparableStats(e).Complexity - getComparableChange(e).PrevComplexity
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
	case SortByTotal:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Total(), getComparableStats(b).Total())
		}
	case SortByPercent:
		if dm.modeInfo.Diff() {
			// Diff mode: percent is the share of the parent's churn, so
			// sorting by it is equivalent to sorting by churn volume.
			return func(a, b *structure.Entry) int {
				return cmpVal(churnVolume(getComparableChange(a)), churnVolume(getComparableChange(b)))
			}
		}
		// SortByPercent is mathematically equivalent to SortByTotal because the
		// parent total is constant for all siblings being compared.
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Total(), getComparableStats(b).Total())
		}
	case SortByComplexity:
		if dm.modeInfo.Compare() {
			// Compare mode sorts by the magnitude of the complexity delta.
			return func(a, b *structure.Entry) int {
				return cmpVal(abs64(deltaCmplx(a)), abs64(deltaCmplx(b)))
			}
		}
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Complexity, getComparableStats(b).Complexity)
		}
	case SortByAdded:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableChange(a).Added, getComparableChange(b).Added)
		}
	case SortByDeleted:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableChange(a).Deleted, getComparableChange(b).Deleted)
		}
	case SortByDelta:
		if dm.modeInfo.Compare() {
			// Compare mode sorts by the magnitude of the code delta.
			return func(a, b *structure.Entry) int {
				return cmpVal(abs64(deltaCode(a)), abs64(deltaCode(b)))
			}
		}
		// Diff mode sorts by the magnitude of the net line change so large
		// rewrites and large deletions both surface at the top.
		return func(a, b *structure.Entry) int {
			return cmpVal(absDelta(getComparableChange(a)), absDelta(getComparableChange(b)))
		}
	default:
		return func(a, b *structure.Entry) int { return cmpVal(a.TotalStats.Total(), b.TotalStats.Total()) }
	}
}

// absDelta returns the absolute value of the change's net line delta.
func absDelta(c structure.Change) int64 {
	return abs64(c.Delta())
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// cycleSortColumn advances to the next sortable column and resets the sort
// direction to the default for that column. The cycle follows the column
// layout of the active mode (see NewDirModelWithMode).
func (dm *DirModel) cycleSortColumn() {
	order := dm.sortCycle
	if len(order) == 0 {
		return
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

// metricValue returns the numeric value of the given CodeStats for the
// specified sort key. It is used both for table display and for percentages.
func metricValue(stats structure.CodeStats, key SortKey) int64 {
	switch key {
	case SortByComplexity:
		return stats.Complexity
	default:
		return stats.Total()
	}
}

// parentTotalForKey returns the total value of the current directory for the
// metric associated with the given sort key. It is used as the denominator for
// the "% of Parent" column.
func (dm *DirModel) parentTotalForKey(key SortKey) int64 {
	minSize := int64(1)
	if dm.nav.Entry() == nil {
		return minSize
	}
	stats := dm.comparableStats(dm.nav.Entry())
	return max(minSize, metricValue(stats, key))
}

// churnVolume returns the total lines touched by a change: added plus
// deleted. Unlike the net delta it never cancels out, so it aggregates
// cleanly up directories and works as a percentage denominator.
func churnVolume(c structure.Change) int64 {
	return c.Added + c.Deleted
}

// parentChurnVolume returns the churn volume of the current directory,
// used as the denominator for the Diff-mode "%" column.
func (dm *DirModel) parentChurnVolume() int64 {
	if dm.nav.Entry() == nil {
		return 1
	}
	return max(int64(1), churnVolume(dm.comparableChange(dm.nav.Entry())))
}

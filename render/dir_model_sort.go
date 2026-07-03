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
	case SortByComplexity:
		return func(a, b *structure.Entry) int {
			return cmpVal(getComparableStats(a).Complexity, getComparableStats(b).Complexity)
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
		SortByComplexity,
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

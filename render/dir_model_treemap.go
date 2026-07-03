package render

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

// treemapSizeFunc returns a sizing function for the treemap based on the
// current treemapSizeKey. The function respects the active language filter.
func (dm *DirModel) treemapSizeFunc() func(*structure.Entry) int64 {
	return func(e *structure.Entry) int64 {
		stats := dm.comparableStats(e)
		switch dm.treemapSizeKey {
		case SortByComplexity:
			return stats.Complexity
		default:
			return stats.Total()
		}
	}
}

// cycleTreemapSize advances the treemap sizing metric through the metrics
// supported by the current provider.
func (dm *DirModel) cycleTreemapSize() {
	order := []SortKey{SortByTotal}
	if dm.providerInfo.Capabilities&provider.CapComplexity != 0 {
		order = append(order, SortByComplexity)
	}

	idx := 0
	for i, k := range order {
		if k == dm.treemapSizeKey {
			idx = i
			break
		}
	}
	dm.treemapSizeKey = order[(idx+1)%len(order)]
}

func (dm *DirModel) viewTreemap(availableHeight int) string {
	children := dm.filteredChildren()
	if len(children) == 0 {
		dm.treemapBlocks = nil
		dm.treemapSelected = 0
		return treemapEmptyStyle.Render(" (no items to display)")
	}

	h := availableHeight
	if h < 3 {
		h = 3
	}

	getSize := dm.treemapSizeFunc()

	showLegend := dm.treemapColorByLang &&
		dm.width > treemapLegendTotalWidth+minTreemapWidthWithoutLegend
	canvasW := dm.width
	if showLegend {
		canvasW -= treemapLegendTotalWidth
	}

	view, blocks := Treemap(canvasW, h, children, getSize, dm.treemapSelected, dm.treemapColorByLang)
	dm.treemapBlocks = blocks

	// If a global search result was just applied in treemap mode, select the
	// corresponding block now that the blocks have been laid out.
	if dm.pendingSearchTarget != nil {
		idx := dm.findTreemapBlockIndex(dm.pendingSearchTarget)
		if idx >= 0 {
			dm.treemapSelected = idx
			view, blocks = Treemap(canvasW, h, children, getSize, dm.treemapSelected, dm.treemapColorByLang)
			dm.treemapBlocks = blocks
		}
		dm.pendingSearchTarget = nil
	}

	if len(blocks) > 0 && dm.treemapSelected >= len(blocks) {
		dm.treemapSelected = len(blocks) - 1
		view, blocks = Treemap(canvasW, h, children, getSize, dm.treemapSelected, dm.treemapColorByLang)
		dm.treemapBlocks = blocks
	}

	if showLegend {
		legend := buildTreemapLegend(dm.treemapBlocks, h, getSize)
		view = lipgloss.JoinHorizontal(lipgloss.Top, view, legend)
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

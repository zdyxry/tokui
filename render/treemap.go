package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zdyxry/tokui/structure"

	"github.com/charmbracelet/lipgloss"
)

// treemapRect defines a rectangle in terminal cells.
type treemapRect struct {
	x, y, w, h int
}

// treemapItem is a single item to be laid out in the treemap.
type treemapItem struct {
	entry *structure.Entry
	size  int64
}

// treemapBlock associates a laid-out rectangle with its source entry and label.
type treemapBlock struct {
	entry *structure.Entry
	rect  treemapRect
	label string
}

// treemapColors is the color cycle used for treemap tiles.
// These are deliberately low-saturation, terminal-friendly colors so the
// treemap is easy on the eyes while still making adjacent blocks distinguishable.
var treemapColors = []lipgloss.Color{
	"#7B9E89", // sage green
	"#7A8B99", // slate blue
	"#A98B6A", // muted amber
	"#8F7B9A", // dusty purple
	"#6A9A9A", // teal
	"#B88A7A", // terracotta
	"#8E9B7B", // olive
	"#9A8B7A", // warm gray
	"#7B8FA3", // steel blue
}

// treemapSelectedBorder is used to outline the currently selected tile.
var treemapSelectedBorder = lipgloss.Color("#ebbd34")

// treemapEmptyStyle is shown when the treemap has nothing to render.
var treemapEmptyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#696868")).
	Italic(true)

// cell stores one terminal cell of the treemap grid.
type treemapCell struct {
	ch   rune
	fg   lipgloss.Color
	bg   lipgloss.Color
	bold bool
}

// Treemap renders a squarified treemap for the current directory's children.
// It returns the rendered string and the list of layout blocks, which the
// caller can use for keyboard/mouse selection.
func Treemap(width, height int, children []*structure.Entry, getSize func(*structure.Entry) int64, selectedIdx int) (string, []treemapBlock) {
	if width <= 0 || height <= 0 {
		return "", nil
	}

	items := make([]treemapItem, 0, len(children))
	var total int64
	for _, c := range children {
		if c == nil {
			continue
		}
		sz := getSize(c)
		if sz > 0 {
			items = append(items, treemapItem{entry: c, size: sz})
			total += sz
		}
	}

	if len(items) == 0 {
		return treemapEmptyStyle.Render(" (no items to display)"), nil
	}

	// Sort descending so the largest items get laid out first.
	sort.Slice(items, func(i, j int) bool { return items[i].size > items[j].size })

	// Limit the number of visible tiles so small files do not turn the map
	// into unreadable speckles. The threshold is proportional to the canvas
	// area, with a small minimum so tiny terminals still show a few blocks.
	maxItems := (width * height) / 8
	if maxItems < 5 {
		maxItems = 5
	}
	if len(items) > maxItems {
		var otherSize int64
		for i := maxItems - 1; i < len(items); i++ {
			otherSize += items[i].size
		}
		items = items[:maxItems-1]
		items = append(items, treemapItem{entry: nil, size: otherSize})
	}

	// Compute squarified layout.
	rects := squarify(items, total, treemapRect{0, 0, width, height})

	// Build display blocks (labels, etc.).
	blocks := make([]treemapBlock, 0, len(items))
	for i, it := range items {
		r := rects[i]
		if r.w <= 0 || r.h <= 0 {
			continue
		}

		var label string
		if it.entry != nil {
			name := it.entry.Name()
			if it.entry.IsDir {
				name += "/"
			}
			label = fmt.Sprintf("%s %s", name, formatNumber(it.size))
		} else {
			label = fmt.Sprintf("other %s", formatNumber(it.size))
		}
		blocks = append(blocks, treemapBlock{
			entry: it.entry,
			rect:  r,
			label: label,
		})
	}

	// Draw the grid.
	grid := make([][]treemapCell, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]treemapCell, width)
		for x := 0; x < width; x++ {
			grid[y][x] = treemapCell{ch: ' '}
		}
	}

	for i, b := range blocks {
		color := treemapColors[i%len(treemapColors)]
		selected := i == selectedIdx
		fillRect(grid, b.rect, color)
		drawBorder(grid, b.rect, selected)
		placeLabel(grid, b.rect, b.label, selected)
	}

	// Convert grid to a styled string.
	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var sb strings.Builder
		for x := 0; x < width; x++ {
			cell := grid[y][x]
			style := lipgloss.NewStyle().Background(cell.bg)
			if cell.bold {
				style = style.Bold(true)
			}
			if cell.fg != "" {
				style = style.Foreground(cell.fg)
			}
			sb.WriteString(style.Render(string(cell.ch)))
		}
		lines[y] = sb.String()
	}

	return strings.Join(lines, "\n"), blocks
}

// squarify recursively lays out items into near-square rectangles.
func squarify(items []treemapItem, total int64, bounds treemapRect) []treemapRect {
	result := make([]treemapRect, len(items))
	if len(items) == 0 || bounds.w <= 0 || bounds.h <= 0 || total <= 0 {
		return result
	}
	layoutRow(items, result, 0, len(items), total, bounds)
	return result
}

// layoutRow splits a slice of items into two groups, choosing the split that
// produces rectangles with the aspect ratio closest to a square.
func layoutRow(items []treemapItem, result []treemapRect, start, end int, total int64, bounds treemapRect) {
	if start >= end || bounds.w <= 0 || bounds.h <= 0 || total <= 0 {
		return
	}
	if end-start == 1 {
		result[start] = bounds
		return
	}

	horizontal := bounds.w >= bounds.h

	var running int64
	bestSplit := start + 1
	bestRatio := 1e18

	for i := start; i < end-1; i++ {
		running += items[i].size
		fraction := float64(running) / float64(total)

		var dim1, dim2 float64
		if horizontal {
			dim1 = fraction * float64(bounds.w)
			dim2 = float64(bounds.h)
		} else {
			dim1 = float64(bounds.w)
			dim2 = fraction * float64(bounds.h)
		}

		aspect := dim1 / dim2
		if dim2 > dim1 {
			aspect = dim2 / dim1
		}
		if aspect < bestRatio {
			bestRatio = aspect
			bestSplit = i + 1
		}
	}

	var leftSize int64
	for i := start; i < bestSplit; i++ {
		leftSize += items[i].size
	}
	fraction := float64(leftSize) / float64(total)

	var leftBounds, rightBounds treemapRect
	if horizontal {
		splitX := int(fraction * float64(bounds.w))
		if splitX < 1 {
			splitX = 1
		}
		if splitX >= bounds.w {
			splitX = bounds.w - 1
		}
		leftBounds = treemapRect{bounds.x, bounds.y, splitX, bounds.h}
		rightBounds = treemapRect{bounds.x + splitX, bounds.y, bounds.w - splitX, bounds.h}
	} else {
		splitY := int(fraction * float64(bounds.h))
		if splitY < 1 {
			splitY = 1
		}
		if splitY >= bounds.h {
			splitY = bounds.h - 1
		}
		leftBounds = treemapRect{bounds.x, bounds.y, bounds.w, splitY}
		rightBounds = treemapRect{bounds.x, bounds.y + splitY, bounds.w, bounds.h - splitY}
	}

	rightSize := total - leftSize
	layoutRow(items, result, start, bestSplit, leftSize, leftBounds)
	layoutRow(items, result, bestSplit, end, rightSize, rightBounds)
}

func fillRect(grid [][]treemapCell, r treemapRect, color lipgloss.Color) {
	startY := r.y
	if startY < 0 {
		startY = 0
	}
	startX := r.x
	if startX < 0 {
		startX = 0
	}
	for y := startY; y < r.y+r.h && y < len(grid); y++ {
		for x := startX; x < r.x+r.w && x < len(grid[y]); x++ {
			grid[y][x].bg = color
		}
	}
}

func drawBorder(grid [][]treemapCell, r treemapRect, selected bool) {
	if r.w < 2 || r.h < 2 {
		return
	}
	h := len(grid)
	w := len(grid[0])

	fg := lipgloss.Color("")
	if selected {
		fg = treemapSelectedBorder
	}

	for x := r.x; x < r.x+r.w && x < w; x++ {
		if r.y < h {
			setCell(grid, x, r.y, borderRune(x, r.x, r.x+r.w-1, '┌', '┐', '─'), fg, "")
		}
		by := r.y + r.h - 1
		if by < h {
			setCell(grid, x, by, borderRune(x, r.x, r.x+r.w-1, '└', '┘', '─'), fg, "")
		}
	}
	for y := r.y + 1; y < r.y+r.h-1 && y < h; y++ {
		if r.x < w {
			setCell(grid, r.x, y, '│', fg, "")
		}
		rx := r.x + r.w - 1
		if rx < w {
			setCell(grid, rx, y, '│', fg, "")
		}
	}
}

func borderRune(x, left, right int, leftCorner, rightCorner, mid rune) rune {
	if x == left {
		return leftCorner
	}
	if x == right {
		return rightCorner
	}
	return mid
}

func placeLabel(grid [][]treemapCell, r treemapRect, label string, selected bool) {
	innerW := r.w - 2
	innerH := r.h - 2
	if innerW <= 0 || innerH <= 0 {
		return
	}

	runes := []rune(label)
	maxRunes := innerW
	if len(runes) > maxRunes {
		if maxRunes > 3 {
			runes = append(runes[:maxRunes-3], '…')
		} else {
			runes = runes[:maxRunes]
		}
	}

	// Center vertically (single line for now).
	y := r.y + 1
	x := r.x + 1
	if y >= len(grid) {
		return
	}

	fg := lipgloss.Color("#262626")
	if selected {
		fg = lipgloss.Color("#FFFFFF")
	}

	for i, ch := range runes {
		pos := x + i
		if pos >= len(grid[y]) {
			break
		}
		grid[y][pos].ch = ch
		grid[y][pos].fg = fg
		grid[y][pos].bold = selected
	}
}

func setCell(grid [][]treemapCell, x, y int, ch rune, fg, bg lipgloss.Color) {
	if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
		return
	}
	grid[y][x].ch = ch
	if fg != "" {
		grid[y][x].fg = fg
	}
	if bg != "" {
		grid[y][x].bg = bg
	}
}

// treemapBlockAt returns the index of the block containing the given cell
// coordinates, or -1 if none.
func treemapBlockAt(blocks []treemapBlock, x, y int) int {
	for i, b := range blocks {
		r := b.rect
		if x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h {
			return i
		}
	}
	return -1
}

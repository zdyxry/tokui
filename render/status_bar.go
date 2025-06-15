package render

import (
	"math"

	"github.com/charmbracelet/lipgloss"
)

const (
	DefaultBorder     = '\ue0b0'
	DefaultBarBGColor = "#353533"
	DynamicWidth      = -1
)

// BarItem represents a status bar item
// width -1 means take all available width, shared equally with other -1 width items
type BarItem struct {
	content string
	bgColor string
	width   int
	border  rune
}

// DefaultBarItem creates an item with default background color and width
func DefaultBarItem(content string) *BarItem {
	return &BarItem{
		content: content,
		bgColor: DefaultBarBGColor,
		border:  DefaultBorder,
	}
}

// NewBarItem creates an item with specified properties
func NewBarItem(content, bgColor string, width int) *BarItem {
	if bgColor == "" {
		bgColor = DefaultBarBGColor
	}

	return &BarItem{
		content: content,
		bgColor: bgColor,
		width:   width,
		border:  DefaultBorder,
	}
}

// NewStatusBar builds a status bar with the given items and total width
func NewStatusBar(items []*BarItem, totalWidth int) string {
	styles := make([]lipgloss.Style, 0, len(items))
	renderItems := make([]string, 0, len(items))
	toMaxWidth := make(map[int]struct{}, len(items))

	for i := range items {
		item := items[i]

		if i == len(items)-1 {
			item.border = 0
		}

		itemStyle := newBarBlockStyle(item)

		if item.width > 0 {
			itemStyle = itemStyle.Width(item.width)
		}

		// set the current item border bg color same as next bar item bg color.
		if i+1 < len(items) {
			itemStyle = itemStyle.BorderBackground(
				lipgloss.Color(items[i+1].bgColor),
			)
		}

		widthDiff := lipgloss.Width(itemStyle.Render(item.content))

		if item.width == DynamicWidth {
			toMaxWidth[i] = struct{}{}
			widthDiff = 1
		}

		totalWidth -= widthDiff
		styles = append(styles, itemStyle)
	}

	var maxItemWidth int

	if len(toMaxWidth) > 0 {
		maxItemWidth = int(
			math.Ceil(float64(totalWidth) / float64(len(toMaxWidth))),
		)
	}

	for i := range items {
		style := styles[i]

		if _, ok := toMaxWidth[i]; ok {
			style = style.Width(min(totalWidth, maxItemWidth))

			totalWidth -= style.GetWidth()
		}

		renderItems = append(renderItems, style.Render(items[i].content))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderItems...)
}

func newBarBlockStyle(bi *BarItem) lipgloss.Style {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color(bi.bgColor)).
		Padding(0, 1)

	if bi.border != 0 {
		style = style.Border(
			lipgloss.Border{Right: string(bi.border)}, false, true, false, false).
			BorderForeground(lipgloss.Color(bi.bgColor))
	}

	return style
}

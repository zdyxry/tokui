package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// diffLineKind classifies one line inside a diff hunk.
type diffLineKind int

const (
	diffContext diffLineKind = iota
	diffDelete
	diffAdd
)

// diffLine is one parsed hunk line. oldNo/newNo are 1-based line numbers; 0
// means the line does not exist on that side of the diff.
type diffLine struct {
	kind  diffLineKind
	oldNo int
	newNo int
	text  string
}

// diffHunk is one "@@ ... @@" section of a unified diff.
type diffHunk struct {
	header string
	lines  []diffLine
}

// parsedDiff is a single-file unified diff split into metadata headers (the
// "diff --git" block) and hunks.
type parsedDiff struct {
	meta  []string
	hunks []diffHunk
}

// parseUnifiedDiff parses the unified diff of a single file (as produced by
// gitx.DiffFile) into metadata lines and hunks with resolved line numbers.
func parseUnifiedDiff(diff string) parsedDiff {
	var p parsedDiff
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")

	i := 0
	for ; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "@@") {
			break
		}
		p.meta = append(p.meta, lines[i])
	}

	oldNo, newNo := 0, 0
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "@@") {
			oldNo, newNo = parseHunkRange(line)
			p.hunks = append(p.hunks, diffHunk{header: line})
			continue
		}
		if len(p.hunks) == 0 {
			p.meta = append(p.meta, line)
			continue
		}
		h := &p.hunks[len(p.hunks)-1]
		switch {
		case strings.HasPrefix(line, "+"):
			h.lines = append(h.lines, diffLine{kind: diffAdd, newNo: newNo, text: line[1:]})
			newNo++
		case strings.HasPrefix(line, "-"):
			h.lines = append(h.lines, diffLine{kind: diffDelete, oldNo: oldNo, text: line[1:]})
			oldNo++
		case strings.HasPrefix(line, `\`):
			// "\ No newline at end of file": no content of its own, skip.
		default:
			// Context line (leading space); tolerate empty lines.
			text := ""
			if len(line) > 0 {
				text = line[1:]
			}
			h.lines = append(h.lines, diffLine{kind: diffContext, oldNo: oldNo, newNo: newNo, text: text})
			oldNo++
			newNo++
		}
	}
	return p
}

// parseHunkRange extracts the starting line numbers from a hunk header of the
// form "@@ -old[,count] +new[,count] @@ ...". It returns (0, 0) when the
// header cannot be parsed.
func parseHunkRange(header string) (oldStart, newStart int) {
	fields := strings.Fields(header)
	parse := func(token string) int {
		if len(token) < 2 || (token[0] != '-' && token[0] != '+') {
			return 0
		}
		start, _, _ := strings.Cut(token[1:], ",")
		n, _ := strconv.Atoi(start)
		return n
	}
	for _, f := range fields {
		if strings.HasPrefix(f, "-") && oldStart == 0 {
			oldStart = parse(f)
		}
		if strings.HasPrefix(f, "+") && newStart == 0 {
			newStart = parse(f)
		}
	}
	return oldStart, newStart
}

// maxLineNumber returns the largest line number on either side of the diff,
// used to size the line-number gutters.
func (p parsedDiff) maxLineNumber() int {
	max := 0
	for _, h := range p.hunks {
		for _, l := range h.lines {
			if l.oldNo > max {
				max = l.oldNo
			}
			if l.newNo > max {
				max = l.newNo
			}
		}
	}
	return max
}

// Side-by-side cell styles: the changed cell gets a tinted background so the
// two columns read as continuous blocks, the empty side a dark gap.
var (
	splitDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Background(lipgloss.Color("52"))
	splitAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Background(lipgloss.Color("22"))
	splitGapStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	splitNumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	splitSepStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	splitNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
)

// minSplitCellWidth is the smallest per-column content width that still reads
// as a side-by-side diff; narrower viewports fall back to the unified layout.
const minSplitCellWidth = 20

// renderDiff renders the unified diff of a single file for the preview. When
// the viewport is wide enough the diff is shown side-by-side (S1 left, S2
// right); narrow viewports and diffs without hunks (binary files, pure
// renames) fall back to the colorized unified layout.
func renderDiff(diff string, width int) string {
	p := parseUnifiedDiff(diff)
	if len(p.hunks) == 0 {
		return styleDiff(diff)
	}

	gutter := len(strconv.Itoa(p.maxLineNumber()))
	if gutter < 2 {
		gutter = 2
	}
	// Row layout: <num> <cell> │ <num> <cell>
	cellWidth := (width - 2*gutter - 5) / 2
	if cellWidth < minSplitCellWidth {
		return styleDiff(diff)
	}

	var rows []string
	for _, m := range p.meta {
		rows = append(rows, splitNoteStyle.MaxWidth(width).Render(m))
	}
	for _, h := range p.hunks {
		rows = append(rows, diffHunkStyle.MaxWidth(width).Render(h.header))
		rows = append(rows, renderHunkSideBySide(h, gutter, cellWidth)...)
	}
	return strings.Join(rows, "\n")
}

// renderHunkSideBySide aligns one hunk into two-column rows: context lines
// span both columns, deletion and addition runs are paired row by row, and
// the shorter side of a change block is padded with a gap cell.
func renderHunkSideBySide(h diffHunk, gutter, cellWidth int) []string {
	var rows []string
	lines := h.lines
	for i := 0; i < len(lines); {
		l := lines[i]
		if l.kind == diffContext {
			rows = append(rows, splitRow(gutter, cellWidth, l.oldNo, l.text, diffContext, l.newNo, l.text, diffContext))
			i++
			continue
		}
		var dels, adds []diffLine
		for i < len(lines) && lines[i].kind == diffDelete {
			dels = append(dels, lines[i])
			i++
		}
		for i < len(lines) && lines[i].kind == diffAdd {
			adds = append(adds, lines[i])
			i++
		}
		n := len(dels)
		if len(adds) > n {
			n = len(adds)
		}
		for k := 0; k < n; k++ {
			var left, right diffLine
			if k < len(dels) {
				left = dels[k]
			}
			if k < len(adds) {
				right = adds[k]
			}
			rows = append(rows, splitRow(gutter, cellWidth,
				left.oldNo, left.text, left.kind, right.newNo, right.text, right.kind))
		}
	}
	return rows
}

// splitRow renders one two-column row. A kind other than diffDelete/diffAdd
// on a side renders that side as a gap (no line number, dark cell) unless it
// is a shared context line.
func splitRow(gutter, cellWidth, leftNo int, leftText string, leftKind diffLineKind, rightNo int, rightText string, rightKind diffLineKind) string {
	leftNum, leftCell := splitSide(leftNo, leftText, leftKind, diffDelete, gutter, cellWidth, splitDelStyle)
	rightNum, rightCell := splitSide(rightNo, rightText, rightKind, diffAdd, gutter, cellWidth, splitAddStyle)
	return leftNum + " " + leftCell + splitSepStyle.Render(" │ ") + rightNum + " " + rightCell
}

// splitSide renders one side's line-number gutter and content cell. A side
// with no line number (0) comes out as a gap; changeKind is the kind this
// side changes on and gets the tinted cell.
func splitSide(no int, text string, kind, changeKind diffLineKind, gutter, cellWidth int, changeStyle lipgloss.Style) (num, cell string) {
	width := lipgloss.NewStyle().Width(cellWidth).MaxWidth(cellWidth)
	if no == 0 {
		return splitNumStyle.Render(strings.Repeat(" ", gutter)),
			splitGapStyle.Width(cellWidth).Render("")
	}
	num = splitNumStyle.Render(fmt.Sprintf("%*d", gutter, no))
	cellText := strings.ReplaceAll(text, "\t", "    ")
	if kind == changeKind {
		cell = changeStyle.Width(cellWidth).MaxWidth(cellWidth).Render(cellText)
	} else {
		cell = width.Render(cellText)
	}
	return num, cell
}

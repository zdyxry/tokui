package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestParseUnifiedDiff(t *testing.T) {
	raw := "diff --git a/x.go b/x.go\n" +
		"index 111..222 100644\n" +
		"--- a/x.go\n" +
		"+++ b/x.go\n" +
		"@@ -1,3 +1,4 @@ func x\n" +
		" ctx\n" +
		"-old1\n" +
		"-old2\n" +
		"+new1\n" +
		" last\n" +
		"@@ -10 +10,2 @@\n" +
		"-a\n" +
		"+b\n" +
		"+c\n" +
		"\\ No newline at end of file\n"

	p := parseUnifiedDiff(raw)

	if len(p.meta) != 4 {
		t.Errorf("len(meta) = %d, want 4: %v", len(p.meta), p.meta)
	}
	if len(p.hunks) != 2 {
		t.Fatalf("len(hunks) = %d, want 2", len(p.hunks))
	}

	h1 := p.hunks[0]
	if h1.header != "@@ -1,3 +1,4 @@ func x" {
		t.Errorf("hunk header = %q", h1.header)
	}
	want := []diffLine{
		{kind: diffContext, oldNo: 1, newNo: 1, text: "ctx"},
		{kind: diffDelete, oldNo: 2, text: "old1"},
		{kind: diffDelete, oldNo: 3, text: "old2"},
		{kind: diffAdd, newNo: 2, text: "new1"},
		{kind: diffContext, oldNo: 4, newNo: 3, text: "last"},
	}
	if len(h1.lines) != len(want) {
		t.Fatalf("hunk1 lines = %d, want %d: %+v", len(h1.lines), len(want), h1.lines)
	}
	for i, w := range want {
		if h1.lines[i] != w {
			t.Errorf("hunk1 line %d = %+v, want %+v", i, h1.lines[i], w)
		}
	}

	h2 := p.hunks[1]
	if len(h2.lines) != 3 {
		t.Fatalf("hunk2 lines = %d, want 3: %+v", len(h2.lines), h2.lines)
	}
	if h2.lines[0] != (diffLine{kind: diffDelete, oldNo: 10, text: "a"}) {
		t.Errorf("hunk2 line 0 = %+v", h2.lines[0])
	}
	if h2.lines[1] != (diffLine{kind: diffAdd, newNo: 10, text: "b"}) {
		t.Errorf("hunk2 line 1 = %+v", h2.lines[1])
	}
	if h2.lines[2] != (diffLine{kind: diffAdd, newNo: 11, text: "c"}) {
		t.Errorf("hunk2 line 2 = %+v", h2.lines[2])
	}

	if got := p.maxLineNumber(); got != 11 {
		t.Errorf("maxLineNumber = %d, want 11", got)
	}
}

func TestRenderDiffSideBySide(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "diff --git a/x.go b/x.go\n" +
		"--- a/x.go\n" +
		"+++ b/x.go\n" +
		"@@ -1,3 +1,3 @@\n" +
		" ctx\n" +
		"-old1\n" +
		"-old2\n" +
		"+new1\n" +
		" end\n"

	out := renderDiff(raw, 80)
	rows := strings.Split(out, "\n")

	// 3 meta lines + hunk header + 4 hunk rows (ctx, paired old1/new1,
	// old2 against a gap, ctx).
	if len(rows) != 8 {
		t.Fatalf("expected 8 rows, got %d:\n%s", len(rows), out)
	}

	// Every row fits the viewport width.
	for i, row := range rows {
		if w := lipgloss.Width(row); w > 80 {
			t.Errorf("row %d is %d cells wide, exceeds 80: %q", i, w, row)
		}
	}

	// The change block pairs old1 with new1 on one row, old2 with a gap.
	paired := rows[5]
	if !strings.Contains(paired, "old1") || !strings.Contains(paired, "new1") {
		t.Errorf("expected old1 and new1 on the same row, got %q", paired)
	}
	gap := rows[6]
	if !strings.Contains(gap, "old2") || strings.Contains(gap, "new") {
		t.Errorf("expected old2 against an empty right cell, got %q", gap)
	}
	// Context rows carry the text on both sides.
	ctx := rows[4]
	if strings.Count(ctx, "ctx") != 2 {
		t.Errorf("expected the context line on both sides, got %q", ctx)
	}
}

func TestRenderDiffLineNumbers(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "@@ -41,2 +41,2 @@\n-foo\n+bar\n"
	out := renderDiff(raw, 80)
	if !strings.Contains(out, "41") {
		t.Errorf("expected line number 41 in the gutters, got:\n%s", out)
	}
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Errorf("expected both sides rendered, got:\n%s", out)
	}
}

func TestRenderDiffNarrowFallsBackToUnified(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "@@ -1 +1 @@\n-old\n+new\n"
	// width 45 leaves 18 cells per column, below minSplitCellWidth.
	if got, want := renderDiff(raw, 45), styleDiff(raw); got != want {
		t.Errorf("narrow viewport must fall back to the unified layout\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderDiffWithoutHunksFallsBackToUnified(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "diff --git a/b.bin b/b.bin\nindex 111..222 100644\nBinary files a/b.bin and b/b.bin differ\n"
	if got, want := renderDiff(raw, 120), styleDiff(raw); got != want {
		t.Errorf("binary diff must fall back to the unified layout\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderDiffTabsExpanded(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "@@ -1 +1 @@\n-\tindented\n+\tindented\n"
	out := renderDiff(raw, 80)
	if strings.Contains(out, "\t") {
		t.Errorf("tabs must be expanded for stable alignment, got %q", out)
	}
	if !strings.Contains(out, "    indented") {
		t.Errorf("expected the tab expanded to spaces, got %q", out)
	}
}

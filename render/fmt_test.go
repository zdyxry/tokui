package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-1, "-1"},
		{-12, "-12"},
		{-123, "-123"},
		{-1234, "-1,234"},
		{-123456, "-123,456"},
		{-1234567, "-1,234,567"},
	}

	for _, c := range cases {
		if got := formatNumber(c.in); got != c.want {
			t.Errorf("formatNumber(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTruncateVisual(t *testing.T) {
	cases := []struct {
		s       string
		max     int
		want    string
		wantLen int
	}{
		{"hello", 0, "", 0},
		{"hello", 2, "he", 2},
		{"hello", 5, "hello", 5},
		{"hello", 10, "hello", 5},
		{"中文测试", 4, "中文", 4},
	}

	for _, c := range cases {
		got := truncateVisual(c.s, c.max)
		if got != c.want {
			t.Errorf("truncateVisual(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
		}
		if w := lipgloss.Width(got); w != c.wantLen {
			t.Errorf("truncateVisual(%q, %d) width = %d, want %d", c.s, c.max, w, c.wantLen)
		}
	}
}

func TestNewStatusBarDynamicWidthDoesNotWrap(t *testing.T) {
	items := []*BarItem{
		NewBarItem("PATH", "#FF5F87", 0),
		NewBarItem("/very/long/path/that/could/wrap", "", -1),
		NewBarItem("MODE", "#06b6d4", 0),
		DefaultBarItem("Nav"),
	}

	const totalWidth = 40
	rendered := NewStatusBar(items, totalWidth)

	// The rendered status bar should not exceed the requested total width,
	// and should not contain newline characters from wrapping content.
	if lipgloss.Width(rendered) > totalWidth {
		t.Errorf("status bar width %d exceeds total width %d", lipgloss.Width(rendered), totalWidth)
	}
	if strings.Contains(rendered, "\n") {
		t.Errorf("status bar should not wrap to multiple lines:\n%s", rendered)
	}
}

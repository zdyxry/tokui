package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultBarItem(t *testing.T) {
	item := DefaultBarItem("content")

	if item.content != "content" {
		t.Errorf("content = %q, want %q", item.content, "content")
	}
	if item.bgColor != DefaultBarBGColor {
		t.Errorf("bgColor = %q, want %q", item.bgColor, DefaultBarBGColor)
	}
	if item.width != 0 {
		t.Errorf("width = %d, want 0", item.width)
	}
	if item.border != DefaultBorder {
		t.Errorf("border = %q, want %q", item.border, DefaultBorder)
	}
}

func TestNewBarItemEmptyBgColorFallsBack(t *testing.T) {
	item := NewBarItem("content", "", 10)

	if item.bgColor != DefaultBarBGColor {
		t.Errorf("bgColor = %q, want default %q", item.bgColor, DefaultBarBGColor)
	}
}

func TestNewStatusBarRendersAllItems(t *testing.T) {
	items := []*BarItem{
		NewBarItem("ONE", "#FF5F87", 5),
		NewBarItem("TWO", "#06b6d4", 5),
		DefaultBarItem("THREE"),
	}

	rendered := NewStatusBar(items, 40)

	for _, want := range []string{"ONE", "TWO", "THREE"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered output missing %q", want)
		}
	}
}

func TestNewStatusBarOnlyFixedWidthItems(t *testing.T) {
	items := []*BarItem{
		NewBarItem("ONE", "#FF5F87", 8),
		NewBarItem("TWO", "#06b6d4", 8),
	}

	const totalWidth = 30
	rendered := NewStatusBar(items, totalWidth)

	if lipgloss.Width(rendered) > totalWidth {
		t.Errorf("rendered width %d exceeds total width %d", lipgloss.Width(rendered), totalWidth)
	}
	if strings.Contains(rendered, "\n") {
		t.Errorf("status bar should not wrap")
	}
}

func TestNewStatusBarMultipleDynamicWidthItems(t *testing.T) {
	items := []*BarItem{
		NewBarItem("LEFT", "#FF5F87", -1),
		NewBarItem("RIGHT", "#06b6d4", -1),
	}

	const totalWidth = 40
	rendered := NewStatusBar(items, totalWidth)

	for _, want := range []string{"LEFT", "RIGHT"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered output missing %q", want)
		}
	}
	if lipgloss.Width(rendered) > totalWidth {
		t.Errorf("rendered width %d exceeds total width %d", lipgloss.Width(rendered), totalWidth)
	}
	if strings.Contains(rendered, "\n") {
		t.Errorf("status bar should not wrap")
	}
}

func TestNewStatusBarTotalWidthNotExceeded(t *testing.T) {
	items := []*BarItem{
		NewBarItem("A", "#FF5F87", 5),
		NewBarItem("/some/path", "", -1),
		NewBarItem("B", "#06b6d4", 5),
	}

	const totalWidth = 30
	rendered := NewStatusBar(items, totalWidth)

	if lipgloss.Width(rendered) > totalWidth {
		t.Errorf("rendered width %d exceeds total width %d", lipgloss.Width(rendered), totalWidth)
	}
}

func TestNewStatusBarDynamicWidthItemTruncated(t *testing.T) {
	longContent := "/very/long/path/that/should/be/truncated"
	items := []*BarItem{
		NewBarItem(longContent, "", -1),
	}

	const totalWidth = 12
	rendered := NewStatusBar(items, totalWidth)

	if lipgloss.Width(rendered) > totalWidth {
		t.Errorf("rendered width %d exceeds total width %d", lipgloss.Width(rendered), totalWidth)
	}
	if strings.Contains(rendered, "truncated") {
		t.Errorf("dynamic content was not truncated")
	}
}

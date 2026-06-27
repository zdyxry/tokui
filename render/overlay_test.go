package render

import (
	"strings"
	"testing"
)

func TestOverlayCenter(t *testing.T) {
	background := "aaaa\naaaa\naaaa\naaaa\naaaa"
	overlay := "X"

	got := OverlayCenter(8, 5, background, overlay)
	lines := strings.Split(got, "\n")

	if len(lines) != 5 {
		t.Fatalf("expected 5 output lines, got %d", len(lines))
	}

	found := false
	for i, line := range lines {
		if strings.Contains(line, "X") {
			// Overlay should be vertically centered around row 2.
			if i != 2 {
				t.Errorf("expected overlay near row 2, found at row %d", i)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("overlay content not found in output")
	}
}

func TestOverlay(t *testing.T) {
	t.Run("places overlay at requested row/col", func(t *testing.T) {
		background := "........\n........\n........"
		overlay := "XX"

		got := Overlay(8, background, overlay, 1, 2)
		lines := strings.Split(got, "\n")

		if !strings.Contains(lines[1], "XX") {
			t.Errorf("expected overlay at row 1, got %q", lines[1])
		}
		if strings.Contains(lines[0], "XX") || strings.Contains(lines[2], "XX") {
			t.Errorf("overlay appeared on unexpected rows")
		}
	})

	t.Run("does not panic when overlay rows exceed background rows", func(t *testing.T) {
		background := "short"
		overlay := "line1\nline2\nline3\nline4\nline5"

		got := Overlay(10, background, overlay, 0, 0)
		if strings.Count(got, "\n") > 4 {
			t.Errorf("output unexpectedly grew to fit overlay")
		}
	})

	t.Run("handles negative row/col gracefully", func(t *testing.T) {
		background := "background\nbackground"
		overlay := "X"

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Overlay panicked with negative row/col: %v", r)
			}
		}()

		_ = Overlay(10, background, overlay, -1, -1)
	})
}

func TestTruncateLeft(t *testing.T) {
	t.Run("panics on input containing newlines", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for input containing newline")
			}
		}()

		truncateLeft("a\nb", 10)
	})

	t.Run("returns empty string when line fits within padding", func(t *testing.T) {
		got := truncateLeft("short", 80)
		if got != "" {
			t.Errorf("truncateLeft() = %q, want empty string", got)
		}
	})
}

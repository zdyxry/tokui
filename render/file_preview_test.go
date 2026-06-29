package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestNewFilePreview(t *testing.T) {
	t.Run("text file content loaded", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTempFile(t, dir, "sample.txt", "hello preview")

		fp := NewFilePreview(p, 80, 24)

		if fp.fileName != "sample.txt" {
			t.Errorf("fileName = %q, want sample.txt", fp.fileName)
		}
		if fp.width < 50 || fp.height < 15 {
			t.Errorf("dimensions too small: %dx%d", fp.width, fp.height)
		}
		if !strings.Contains(fp.content, "hello preview") {
			t.Errorf("content missing file text: %q", fp.content)
		}
		if !fp.ready {
			t.Error("expected ready to be true")
		}
	})

	t.Run("minimum dimensions enforced", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTempFile(t, dir, "x.txt", "x")

		fp := NewFilePreview(p, 10, 5)

		if fp.width != 50 {
			t.Errorf("width = %d, want 50", fp.width)
		}
		if fp.height != 15 {
			t.Errorf("height = %d, want 15", fp.height)
		}
		if fp.viewport.Width != 40 || fp.viewport.Height != 7 {
			t.Errorf("viewport size = %dx%d, want 40x7", fp.viewport.Width, fp.viewport.Height)
		}
	})

	t.Run("non-existent file shows error", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "missing.txt")
		fp := NewFilePreview(p, 80, 24)

		if fp.errorMsg == "" {
			t.Error("expected error message for missing file")
		}
		if !strings.Contains(fp.content, "Error reading file") {
			t.Errorf("content missing error text: %q", fp.content)
		}
	})

	t.Run("binary extension detected", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTempFile(t, dir, "img.png", "not-really-binary")

		fp := NewFilePreview(p, 80, 24)

		if !strings.Contains(fp.content, "Binary file detected") {
			t.Errorf("content missing binary detection: %q", fp.content)
		}
	})

	t.Run("oversized file shows warning", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "big.txt")
		data := make([]byte, 11*1024*1024)
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatalf("write oversized file: %v", err)
		}

		fp := NewFilePreview(p, 80, 24)

		if !strings.Contains(fp.content, "File too large") {
			t.Errorf("content missing oversized warning: %q", fp.content)
		}
	})
}

func TestFilePreviewUpdate(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("line\n", 100)
	p := writeTempFile(t, dir, "scroll.txt", content)
	fp := NewFilePreview(p, 40, 20)

	t.Run("window size message resizes preview", func(t *testing.T) {
		fp.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

		if fp.width != 80 || fp.height != 32 {
			t.Errorf("size = %dx%d, want 80x32", fp.width, fp.height)
		}
		if fp.viewport.Width != 70 || fp.viewport.Height != 24 {
			t.Errorf("viewport size = %dx%d, want 70x24", fp.viewport.Width, fp.viewport.Height)
		}
	})

	t.Run("key message forwarded to viewport", func(t *testing.T) {
		before := fp.viewport.YOffset
		fp.Update(tea.KeyMsg{Type: tea.KeyDown})
		if fp.viewport.YOffset <= before {
			t.Errorf("expected viewport to scroll down, got YOffset %d -> %d", before, fp.viewport.YOffset)
		}
	})

	t.Run("mouse message forwarded to viewport", func(t *testing.T) {
		before := fp.viewport.YOffset
		fp.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
		if fp.viewport.YOffset < before {
			t.Errorf("expected viewport to scroll down, got YOffset %d -> %d", before, fp.viewport.YOffset)
		}
	})

	t.Run("home key jumps to top", func(t *testing.T) {
		fp.viewport.YOffset = 10
		fp.Update(tea.KeyMsg{Type: tea.KeyHome})
		if fp.viewport.YOffset != 0 {
			t.Errorf("expected home to jump to top, got YOffset %d", fp.viewport.YOffset)
		}
	})

	t.Run("end key jumps to bottom", func(t *testing.T) {
		fp.viewport.YOffset = 0
		fp.Update(tea.KeyMsg{Type: tea.KeyEnd})
		want := max(0, fp.viewport.TotalLineCount()-fp.viewport.VisibleLineCount())
		if fp.viewport.YOffset != want {
			t.Errorf("expected end to jump to bottom offset %d, got %d", want, fp.viewport.YOffset)
		}
	})
}

func TestFilePreviewView(t *testing.T) {
	t.Run("renders file name and content", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTempFile(t, dir, "view.txt", "content line")
		fp := NewFilePreview(p, 80, 24)

		v := fp.View()
		if v == "" {
			t.Fatal("expected non-empty view")
		}
		if !strings.Contains(v, "view.txt") {
			t.Errorf("view missing file name: %q", v)
		}
		if !strings.Contains(v, "content line") {
			t.Errorf("view missing content: %q", v)
		}
	})

	t.Run("renders error content", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "missing.txt")
		fp := NewFilePreview(p, 80, 24)

		v := fp.View()
		if !strings.Contains(v, "Error reading file") {
			t.Errorf("view missing error: %q", v)
		}
	})
}

func TestFilePreviewReadFileContent(t *testing.T) {
	dir := t.TempDir()
	fp := NewFilePreview(filepath.Join(dir, "dummy.txt"), 80, 24)

	t.Run("text file", func(t *testing.T) {
		p := writeTempFile(t, dir, "plain.txt", "plain text")
		got, err := fp.readFileContent(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "plain text" {
			t.Errorf("got %q, want plain text", got)
		}
	})

	t.Run("binary extension", func(t *testing.T) {
		p := writeTempFile(t, dir, "app.exe", "MZ header")
		got, err := fp.readFileContent(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Binary file detected") {
			t.Errorf("missing binary detection: %q", got)
		}
	})

	t.Run("null byte content", func(t *testing.T) {
		p := filepath.Join(dir, "null.dat")
		if err := os.WriteFile(p, []byte("a\x00b"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		got, err := fp.readFileContent(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Binary file detected") {
			t.Errorf("missing binary detection: %q", got)
		}
	})
}

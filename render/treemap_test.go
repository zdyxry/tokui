package render

import (
	"strings"
	"testing"

	"github.com/zdyxry/tokui/structure"
)

func TestTreemapBasic(t *testing.T) {
	children := []*structure.Entry{
		{Path: "a.go", IsDir: false, TotalStats: structure.CodeStats{Code: 100}},
		{Path: "b.go", IsDir: false, TotalStats: structure.CodeStats{Code: 200}},
		{Path: "cmd", IsDir: true, TotalStats: structure.CodeStats{Code: 300}},
	}

	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }
	view, blocks := Treemap(40, 20, children, getSize, 0)

	if view == "" {
		t.Fatal("Treemap returned empty view")
	}
	if len(blocks) == 0 {
		t.Fatal("Treemap returned no blocks")
	}

	// The largest item should be first.
	if blocks[0].entry == nil || blocks[0].entry.Path != "cmd" {
		t.Fatalf("expected largest block to be cmd, got %v", blocks[0].entry)
	}

	// Selected block should be highlighted.
	if !strings.Contains(view, "cmd") {
		t.Fatal("expected view to contain the selected block label")
	}
}

func TestTreemapEmpty(t *testing.T) {
	view, blocks := Treemap(40, 20, nil, func(e *structure.Entry) int64 { return 0 }, 0)
	if view == "" {
		t.Fatal("expected non-empty empty-state view")
	}
	if len(blocks) != 0 {
		t.Fatalf("expected no blocks, got %d", len(blocks))
	}
}

func TestTreemapBlockAt(t *testing.T) {
	blocks := []treemapBlock{
		{rect: treemapRect{0, 0, 10, 10}},
		{rect: treemapRect{10, 0, 10, 10}},
	}

	if got := treemapBlockAt(blocks, 5, 5); got != 0 {
		t.Fatalf("expected block 0, got %d", got)
	}
	if got := treemapBlockAt(blocks, 15, 5); got != 1 {
		t.Fatalf("expected block 1, got %d", got)
	}
	if got := treemapBlockAt(blocks, 25, 5); got != -1 {
		t.Fatalf("expected no block, got %d", got)
	}
}

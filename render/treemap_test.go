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

func TestTreemapNestedBlocks(t *testing.T) {
	bigDir := &structure.Entry{
		Path:       "big",
		IsDir:      true,
		TotalStats: structure.CodeStats{Code: 1000},
		Child: []*structure.Entry{
			{Path: "a.go", IsDir: false, TotalStats: structure.CodeStats{Code: 400}},
			{Path: "b.go", IsDir: false, TotalStats: structure.CodeStats{Code: 300}},
			{Path: "c.go", IsDir: false, TotalStats: structure.CodeStats{Code: 300}},
		},
	}
	smallDir := &structure.Entry{
		Path:       "small",
		IsDir:      true,
		TotalStats: structure.CodeStats{Code: 100},
		Child: []*structure.Entry{
			{Path: "x.go", IsDir: false, TotalStats: structure.CodeStats{Code: 100}},
		},
	}
	children := []*structure.Entry{bigDir, smallDir}

	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }
	view, blocks := Treemap(60, 30, children, getSize, 0)

	if view == "" {
		t.Fatal("Treemap returned empty view")
	}

	// Should contain top-level blocks plus nested children of the large directory.
	if len(blocks) <= len(children) {
		t.Fatalf("expected nested blocks, got %d total blocks", len(blocks))
	}

	// Find the nested child blocks.
	hasA, hasB, hasC := false, false, false
	for _, b := range blocks {
		if b.entry == nil {
			continue
		}
		switch b.entry.Path {
		case "a.go":
			hasA = true
		case "b.go":
			hasB = true
		case "c.go":
			hasC = true
		}
	}
	if !hasA || !hasB || !hasC {
		t.Fatalf("expected nested a.go/b.go/c.go blocks, got a=%v b=%v c=%v", hasA, hasB, hasC)
	}

	// Find the top-level block for the big directory.
	var bigRect treemapRect
	for _, b := range blocks {
		if b.entry == bigDir {
			bigRect = b.rect
			break
		}
	}
	if bigRect.w == 0 {
		t.Fatal("could not find top-level block for big directory")
	}

	// Clicking inside the nested area should select the inner block, not the parent.
	inner := treemapBlockAt(blocks, bigRect.x+3, bigRect.y+4)
	if inner < 0 || blocks[inner].level == 0 {
		t.Fatalf("expected nested block at inner coordinates, got %d (level %d)", inner, blocks[inner].level)
	}

	// Each nested block should remember which top-level block it belongs to.
	for _, b := range blocks {
		if b.level > 0 && b.topIdx < 0 {
			t.Fatalf("nested block %v missing topIdx", b.entry)
		}
	}
}

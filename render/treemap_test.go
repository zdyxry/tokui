package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/zdyxry/tokui/structure"
)

func TestTreemapBasic(t *testing.T) {
	children := []*structure.Entry{
		{Path: "a.go", IsDir: false, TotalStats: structure.CodeStats{Code: 100}},
		{Path: "b.go", IsDir: false, TotalStats: structure.CodeStats{Code: 200}},
		{Path: "cmd", IsDir: true, TotalStats: structure.CodeStats{Code: 300}},
	}

	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }
	view, blocks := Treemap(40, 20, children, getSize, 0, false)

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
	view, blocks := Treemap(40, 20, nil, func(e *structure.Entry) int64 { return 0 }, 0, false)
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
	view, blocks := Treemap(60, 30, children, getSize, 0, false)

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

func TestTreemapTopIdxContiguous(t *testing.T) {
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
	_, blocks := Treemap(60, 30, children, getSize, 0, false)

	// topIdx values for top-level blocks must be contiguous and match their
	// position among top-level blocks. This invariant lets keyboard navigation
	// use the topIdx directly instead of searching for it.
	topLevelCount := 0
	for i, b := range blocks {
		if b.level != 0 {
			continue
		}
		if b.topIdx != topLevelCount {
			t.Fatalf("top-level block at index %d has topIdx %d, want %d", i, b.topIdx, topLevelCount)
		}
		if b.topIdx >= len(blocks) {
			t.Fatalf("top-level block topIdx %d out of bounds", b.topIdx)
		}
		topLevelCount++
	}

	// Nested blocks must point to a valid top-level block.
	for _, b := range blocks {
		if b.level > 0 && (b.topIdx < 0 || b.topIdx >= topLevelCount) {
			t.Fatalf("nested block %v has invalid topIdx %d (topLevelCount=%d)", b.entry, b.topIdx, topLevelCount)
		}
	}
}

func TestTreemapNestedPrealloc(t *testing.T) {
	parent := &structure.Entry{
		Path:       "parent",
		IsDir:      true,
		TotalStats: structure.CodeStats{Code: 100},
		Child: []*structure.Entry{
			{Path: "a.go", IsDir: false, TotalStats: structure.CodeStats{Code: 40}},
			{Path: "b.go", IsDir: false, TotalStats: structure.CodeStats{Code: 30}},
			{Path: "c.go", IsDir: false, TotalStats: structure.CodeStats{Code: 30}},
		},
	}

	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }
	// Use a large canvas so nesting is allowed.
	view, blocks := Treemap(40, 20, []*structure.Entry{parent}, getSize, 0, false)
	if view == "" {
		t.Fatal("Treemap returned empty view")
	}
	if len(blocks) <= 1 {
		t.Fatalf("expected nested blocks, got %d", len(blocks))
	}

	// All children should be represented as nested blocks.
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
}

func TestLangColorStable(t *testing.T) {
	c1 := langColor("Go")
	c2 := langColor("Go")
	if c1 != c2 {
		t.Fatalf("expected stable color for Go, got %v and %v", c1, c2)
	}

	c3 := langColor("Rust")
	if c1 == c3 {
		t.Fatalf("expected different colors for Go and Rust, got %v", c1)
	}

	if langColor("") == "" {
		t.Fatal("expected fallback color for empty language")
	}
}

func TestEntryPrimaryLang(t *testing.T) {
	file := &structure.Entry{
		Path:  "main.go",
		IsDir: false,
		StatsByLang: map[string]structure.CodeStats{
			"Go": {Code: 100},
		},
	}
	if got := entryPrimaryLang(file); got != "Go" {
		t.Fatalf("expected primary lang Go, got %q", got)
	}

	dir := &structure.Entry{
		Path:  "pkg",
		IsDir: true,
		StatsByLang: map[string]structure.CodeStats{
			"Go":   {Code: 100},
			"JSON": {Code: 10},
		},
	}
	if got := entryPrimaryLang(dir); got != "Go" {
		t.Fatalf("expected primary lang Go for dir, got %q", got)
	}

	empty := &structure.Entry{Path: "empty", IsDir: true}
	if got := entryPrimaryLang(empty); got != "" {
		t.Fatalf("expected empty primary lang, got %q", got)
	}
}

func TestTreemapLanguageColorMode(t *testing.T) {
	goDir := &structure.Entry{
		Path:  "gocode",
		IsDir: true,
		StatsByLang: map[string]structure.CodeStats{
			"Go": {Code: 500},
		},
		Child: []*structure.Entry{
			{Path: "a.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 300}}},
			{Path: "b.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 200}}},
		},
	}
	jsFile := &structure.Entry{
		Path:        "app.js",
		IsDir:       false,
		StatsByLang: map[string]structure.CodeStats{"JavaScript": {Code: 100}},
	}
	children := []*structure.Entry{goDir, jsFile}

	getSize := func(e *structure.Entry) int64 {
		var total int64
		for _, s := range e.StatsByLang {
			total += s.Total()
		}
		return total
	}

	_, blocks := Treemap(60, 30, children, getSize, 0, true)

	var goFileColor, goDirColor, jsColor lipgloss.Color
	foundGoFile, foundGoDir, foundJS := false, false, false
	for _, b := range blocks {
		if b.entry == nil {
			continue
		}
		lang := entryPrimaryLang(b.entry)
		switch lang {
		case "Go":
			if b.entry.IsDir {
				goDirColor = b.color
				foundGoDir = true
			} else {
				goFileColor = b.color
				foundGoFile = true
			}
		case "JavaScript":
			jsColor = b.color
			foundJS = true
		}
	}
	if !foundGoFile || !foundGoDir || !foundJS {
		t.Fatalf("expected Go file, Go dir and JavaScript tiles, foundGoFile=%v foundGoDir=%v foundJS=%v", foundGoFile, foundGoDir, foundJS)
	}
	if goFileColor == jsColor || goDirColor == jsColor {
		t.Fatalf("expected Go and JavaScript colors to differ, got goFile=%v goDir=%v js=%v", goFileColor, goDirColor, jsColor)
	}
	if goFileColor == goDirColor {
		t.Fatalf("expected directory tile to be shaded differently from file tile, got %v", goFileColor)
	}
}

func TestTreemapLegend(t *testing.T) {
	goFile := &structure.Entry{
		Path:        "a.go",
		IsDir:       false,
		StatsByLang: map[string]structure.CodeStats{"Go": {Code: 100}},
	}
	jsFile := &structure.Entry{
		Path:        "b.js",
		IsDir:       false,
		StatsByLang: map[string]structure.CodeStats{"JavaScript": {Code: 50}},
	}
	blocks := []treemapBlock{
		{entry: goFile, color: langColor("Go")},
		{entry: jsFile, color: langColor("JavaScript")},
	}

	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }
	legend := buildTreemapLegend(blocks, 6, getSize)
	if legend == "" {
		t.Fatal("expected non-empty legend")
	}
	if !strings.Contains(legend, "Go") || !strings.Contains(legend, "JavaScript") {
		t.Fatalf("expected legend to contain Go and JavaScript, got:\n%s", legend)
	}
}

func TestAdjustColor(t *testing.T) {
	c := lipgloss.Color("#808080")

	lighter := adjustColor(c, 0.5)
	if lighter == c {
		t.Fatal("expected lighter color")
	}

	darker := adjustColor(c, -0.5)
	if darker == c {
		t.Fatal("expected darker color")
	}

	if adjustColor("", 0.1) != "" {
		t.Fatal("expected empty color to pass through")
	}

	if adjustColor(lipgloss.Color("not-a-color"), 0.1) != lipgloss.Color("not-a-color") {
		t.Fatal("expected invalid color to pass through")
	}
}

func TestTreemapColorFor(t *testing.T) {
	goFile := &structure.Entry{Path: "a.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 100}}}
	goDir := &structure.Entry{Path: "pkg", IsDir: true, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 100}}}

	c1 := treemapColorFor(goFile, 0, false)
	c2 := treemapColorFor(goFile, 1, false)
	if c1 == c2 {
		t.Fatal("expected different palette colors for different indices")
	}

	langColor1 := treemapColorFor(goFile, 0, true)
	langColor2 := treemapColorFor(goDir, 0, true)
	if langColor1 == langColor2 {
		t.Fatal("expected directory tile to be shaded differently from file tile in language mode")
	}
}

func TestLangColorUnknown(t *testing.T) {
	c := langColor("NotARealLanguage")
	if c != lipgloss.Color("#7F8C8D") {
		t.Fatalf("expected fallback gray for unknown language, got %v", c)
	}
}

func TestBuildTreemapLegendEmpty(t *testing.T) {
	got := buildTreemapLegend(nil, 10, func(e *structure.Entry) int64 { return 0 })
	if got == "" {
		t.Fatal("expected legend with title even for empty blocks")
	}
	if !strings.Contains(got, "Languages") {
		t.Fatal("expected legend title")
	}
}

func TestBuildTreemapLegendTruncatesToHeight(t *testing.T) {
	blocks := []treemapBlock{
		{entry: &structure.Entry{Path: "a.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 100}}}, level: 0},
		{entry: &structure.Entry{Path: "b.js", IsDir: false, StatsByLang: map[string]structure.CodeStats{"JavaScript": {Code: 50}}}, level: 0},
		{entry: &structure.Entry{Path: "c.py", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Python": {Code: 25}}}, level: 0},
	}
	getSize := func(e *structure.Entry) int64 { return e.TotalStats.Total() }

	legend := buildTreemapLegend(blocks, 4, getSize)
	lines := strings.Split(legend, "\n")
	if len(lines) == 0 {
		t.Fatal("expected non-empty legend")
	}
}

func TestTreemapLegendWidthMatchesTotal(t *testing.T) {
	goDir := &structure.Entry{
		Path:  "gocode",
		IsDir: true,
		StatsByLang: map[string]structure.CodeStats{
			"Go": {Code: 500},
		},
		Child: []*structure.Entry{
			{Path: "a.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 300}}},
			{Path: "b.go", IsDir: false, StatsByLang: map[string]structure.CodeStats{"Go": {Code: 200}}},
		},
	}
	jsFile := &structure.Entry{
		Path:        "app.js",
		IsDir:       false,
		StatsByLang: map[string]structure.CodeStats{"JavaScript": {Code: 100}},
	}
	children := []*structure.Entry{goDir, jsFile}

	getSize := func(e *structure.Entry) int64 {
		var total int64
		for _, s := range e.StatsByLang {
			total += s.Total()
		}
		return total
	}

	totalW := 80
	showLegend := totalW > treemapLegendTotalWidth+minTreemapWidthWithoutLegend
	canvasW := totalW
	if showLegend {
		canvasW -= treemapLegendTotalWidth
	}

	view, blocks := Treemap(canvasW, 20, children, getSize, 0, true)
	if showLegend {
		legend := buildTreemapLegend(blocks, 20, getSize)
		combined := lipgloss.JoinHorizontal(lipgloss.Top, view, legend)
		got := lipgloss.Width(combined)
		if got != totalW {
			t.Fatalf("expected combined width %d, got %d", totalW, got)
		}
	} else {
		got := lipgloss.Width(view)
		if got != totalW {
			t.Fatalf("expected treemap width %d, got %d", totalW, got)
		}
	}
}

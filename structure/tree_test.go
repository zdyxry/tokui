package structure

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAddFileToTree_SingleFile(t *testing.T) {
	root := NewDirEntry("root")
	stats := map[string]CodeStats{"Go": {Code: 10}}
	tr := NewTree(root)

	tr.addFileToTree(root, "main.go", stats)

	if len(root.Child) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Child))
	}
	child := root.Child[0]
	if child.Name() != "main.go" {
		t.Errorf("expected child name main.go, got %q", child.Name())
	}
	if child.IsDir {
		t.Error("expected file entry")
	}
	if child.TotalStats.Code != 10 {
		t.Errorf("expected code 10, got %d", child.TotalStats.Code)
	}
}

func TestAddFileToTree_NestedDirectories(t *testing.T) {
	root := NewDirEntry("root")
	stats := map[string]CodeStats{"Go": {Code: 42}}
	tr := NewTree(root)

	tr.addFileToTree(root, "src/pkg/foo.go", stats)

	if len(root.Child) != 1 {
		t.Fatalf("expected 1 top-level child, got %d", len(root.Child))
	}
	src := root.Child[0]
	if src.Name() != "src" || !src.IsDir {
		t.Errorf("expected src dir, got %+v", src)
	}

	if len(src.Child) != 1 {
		t.Fatalf("expected 1 child under src, got %d", len(src.Child))
	}
	pkg := src.Child[0]
	if pkg.Name() != "pkg" || !pkg.IsDir {
		t.Errorf("expected pkg dir, got %+v", pkg)
	}

	if len(pkg.Child) != 1 {
		t.Fatalf("expected 1 child under pkg, got %d", len(pkg.Child))
	}
	foo := pkg.Child[0]
	if foo.Name() != "foo.go" || foo.IsDir {
		t.Errorf("expected foo.go file, got %+v", foo)
	}
	if foo.TotalStats.Code != 42 {
		t.Errorf("expected foo.go code 42, got %d", foo.TotalStats.Code)
	}
}

func TestAddFileToTree_MultipleFilesSameDir(t *testing.T) {
	root := NewDirEntry("root")
	tr := NewTree(root)

	tr.addFileToTree(root, "src/a.go", map[string]CodeStats{"Go": {Code: 1}})
	tr.addFileToTree(root, "src/b.go", map[string]CodeStats{"Go": {Code: 2}})
	tr.addFileToTree(root, "c.py", map[string]CodeStats{"Python": {Code: 3}})

	if len(root.Child) != 2 {
		t.Fatalf("expected 2 top-level children, got %d", len(root.Child))
	}

	src := root.GetChild("src")
	if src == nil {
		t.Fatal("expected src dir")
	}
	if len(src.Child) != 2 {
		t.Errorf("expected 2 files under src, got %d", len(src.Child))
	}

	c := root.GetChild("c.py")
	if c == nil {
		t.Fatal("expected c.py file")
	}
}

func TestAggregateStats_NestedTree(t *testing.T) {
	root := NewDirEntry("root")
	tr := NewTree(root)

	tr.addFileToTree(root, "a.go", map[string]CodeStats{"Go": {Code: 10, Comments: 1, Blanks: 1}})
	tr.addFileToTree(root, "src/b.go", map[string]CodeStats{"Go": {Code: 20, Comments: 2, Blanks: 2}})
	tr.addFileToTree(root, "src/c.py", map[string]CodeStats{"Python": {Code: 5, Comments: 1, Blanks: 1}})

	root.AggregateStats()

	wantTotal := CodeStats{Code: 35, Comments: 4, Blanks: 4}
	if root.TotalStats != wantTotal {
		t.Errorf("root total stats mismatch: got %+v, want %+v", root.TotalStats, wantTotal)
	}

	wantGo := CodeStats{Code: 30, Comments: 3, Blanks: 3}
	if root.StatsByLang["Go"] != wantGo {
		t.Errorf("root Go stats mismatch: got %+v, want %+v", root.StatsByLang["Go"], wantGo)
	}

	wantPython := CodeStats{Code: 5, Comments: 1, Blanks: 1}
	if root.StatsByLang["Python"] != wantPython {
		t.Errorf("root Python stats mismatch: got %+v, want %+v", root.StatsByLang["Python"], wantPython)
	}

	src := root.GetChild("src")
	if src == nil {
		t.Fatal("expected src dir")
	}
	wantSrcTotal := CodeStats{Code: 25, Comments: 3, Blanks: 3}
	if src.TotalStats != wantSrcTotal {
		t.Errorf("src total stats mismatch: got %+v, want %+v", src.TotalStats, wantSrcTotal)
	}
}

func TestBuildFromStdin_ValidTokeiJSON(t *testing.T) {
	input := []byte(`{
		"Go": {
			"blanks": 1,
			"code": 10,
			"comments": 1,
			"reports": [
				{
					"name": "main.go",
					"stats": {"blanks": 1, "code": 10, "comments": 1}
				}
			]
		}
	}`)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	defer r.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	go func() {
		defer w.Close()
		_, _ = w.Write(input)
	}()

	tree := NewTree(nil)
	if err := tree.BuildFromStdin(); err != nil {
		t.Fatalf("BuildFromStdin failed: %v", err)
	}

	root := tree.Root()
	if root == nil {
		t.Fatal("expected root entry")
	}
	if root.TotalStats.Code != 10 {
		t.Errorf("expected root code 10, got %d", root.TotalStats.Code)
	}

	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go file")
	}
}

func TestBuildFromStdin_NestedTokeiJSON(t *testing.T) {
	input := []byte(`{
		"project": {
			"Go": {
				"blanks": 2,
				"code": 20,
				"comments": 2,
				"reports": [
					{
						"name": "./a.go",
						"stats": {"blanks": 2, "code": 20, "comments": 2}
					}
				]
			}
		}
	}`)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	defer r.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	go func() {
		defer w.Close()
		_, _ = w.Write(input)
	}()

	tree := NewTree(nil)
	if err := tree.BuildFromStdin(); err != nil {
		t.Fatalf("BuildFromStdin failed: %v", err)
	}

	root := tree.Root()
	if root.TotalStats.Code != 20 {
		t.Errorf("expected root code 20, got %d", root.TotalStats.Code)
	}
}

func TestBuildFromTokei_Smoke(t *testing.T) {
	if _, err := exec.LookPath("tokei"); err != nil {
		t.Skip("tokei binary not available, skipping smoke test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tree := NewTree(nil)
	if err := tree.BuildFromTokei(dir); err != nil {
		t.Fatalf("BuildFromTokei failed: %v", err)
	}

	root := tree.Root()
	if root == nil {
		t.Fatal("expected root entry")
	}
	if root.TotalStats.Code == 0 {
		t.Error("expected non-zero code stats")
	}

	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go file")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		root string
		raw  string
		want string
	}{
		{"/project", "/project/main.go", "main.go"},
		{"/project", "/project/src/foo.go", "src/foo.go"},
		{"/project", "src/foo.go", "src/foo.go"},
		{"/project", "./main.go", "main.go"},
		{"/project", "//main.go", "/main.go"},
	}

	for _, tt := range tests {
		got := normalizePath(tt.root, tt.raw)
		if got != tt.want {
			t.Errorf("normalizePath(%q, %q) = %q, want %q", tt.root, tt.raw, got, tt.want)
		}
	}
}

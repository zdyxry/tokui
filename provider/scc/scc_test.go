package scc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zdyxry/tokui/provider"
)

var _ provider.Provider = (*SCCProvider)(nil)

func TestProviderInfo(t *testing.T) {
	p := New()
	info := p.Info()
	if info.Name != "scc" {
		t.Errorf("expected name scc, got %q", info.Name)
	}
	wantCaps := provider.CapLines | provider.CapComplexity | provider.CapBytes
	if info.Capabilities != wantCaps {
		t.Errorf("expected capabilities %v, got %v", wantCaps, info.Capabilities)
	}
}

func TestAnalyze_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	content := "package main\n\nfunc main() {\n\tif true {\n\t\treturn\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := New()
	result, err := p.Analyze(path)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	f := result.Files[0]
	if f.Language != "Go" {
		t.Errorf("expected language Go, got %q", f.Language)
	}
	if f.Code == 0 {
		t.Error("expected non-zero code count")
	}
	if f.Complexity == 0 {
		t.Error("expected non-zero complexity")
	}
	if f.Bytes == 0 {
		t.Error("expected non-zero bytes")
	}
}

func TestAnalyze_Directory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc a() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.py"), []byte("def b():\n    pass\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := New()
	result, err := p.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result.Files))
	}

	langs := make(map[string]bool)
	for _, f := range result.Files {
		langs[f.Language] = true
	}
	if !langs["Go"] || !langs["Python"] {
		t.Errorf("expected Go and Python, got %v", langs)
	}
}

func TestParseStdin(t *testing.T) {
	data := []byte(`[
		{
			"Name": "Go",
			"Bytes": 200,
			"Code": 10,
			"Comment": 1,
			"Blank": 1,
			"Complexity": 3,
			"Files": [
				{
					"Filename": "main.go",
					"Location": "./main.go",
					"Language": "Go",
					"Code": 10,
					"Comment": 1,
					"Blank": 1,
					"Complexity": 3,
					"Bytes": 200
				}
			]
		}
	]`)

	p := New()
	result, err := p.ParseStdin(data)
	if err != nil {
		t.Fatalf("ParseStdin failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	f := result.Files[0]
	if f.Path != "./main.go" {
		t.Errorf("expected path ./main.go, got %q", f.Path)
	}
	if f.Language != "Go" {
		t.Errorf("expected language Go, got %q", f.Language)
	}
	if f.Complexity != 3 {
		t.Errorf("expected complexity 3, got %d", f.Complexity)
	}
}

func TestParseStdin_Empty(t *testing.T) {
	p := New()
	_, err := p.ParseStdin([]byte{})
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
}

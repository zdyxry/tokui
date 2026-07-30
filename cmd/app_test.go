package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/provider/scc"
	"github.com/zdyxry/tokui/tokei"
)

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringVar(
		&providerName,
		"provider",
		"tokei",
		"Stats provider: tokei|scc.",
	)
	return cmd
}

func TestSelectProvider(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"tokei", "tokei", false},
		{"scc", "scc", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := selectProvider(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unknown provider")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Info().Name != tt.want {
				t.Errorf("expected provider %q, got %q", tt.want, p.Info().Name)
			}
		})
	}
}

func TestResolveProvider(t *testing.T) {
	t.Run("default to tokei", func(t *testing.T) {
		t.Setenv("TOKUI_PROVIDER", "")
		cmd := newTestCommand()
		if got := resolveProvider(cmd); got != "tokei" {
			t.Errorf("expected tokei, got %q", got)
		}
	})

	t.Run("env variable overrides default", func(t *testing.T) {
		t.Setenv("TOKUI_PROVIDER", "scc")
		cmd := newTestCommand()
		if got := resolveProvider(cmd); got != "scc" {
			t.Errorf("expected scc, got %q", got)
		}
	})

	t.Run("flag overrides env variable", func(t *testing.T) {
		t.Setenv("TOKUI_PROVIDER", "scc")
		cmd := newTestCommand()
		_ = cmd.ParseFlags([]string{"--provider", "tokei"})
		if got := resolveProvider(cmd); got != "tokei" {
			t.Errorf("expected tokei from flag, got %q", got)
		}
	})

	t.Run("invalid env falls through to default", func(t *testing.T) {
		t.Setenv("TOKUI_PROVIDER", "")
		cmd := newTestCommand()
		_ = cmd.ParseFlags([]string{"--provider", "tokei"})
		if got := resolveProvider(cmd); got != "tokei" {
			t.Errorf("expected tokei, got %q", got)
		}
	})
}

func TestParseStdinWithProvider_TokeiData(t *testing.T) {
	data := []byte(`{
		"Go": {
			"blanks": 5,
			"code": 20,
			"comments": 5,
			"reports": [
				{
					"name": "main.go",
					"stats": {"blanks": 5, "code": 20, "comments": 5}
				}
			]
		}
	}`)

	result, used, err := parseStdinWithProvider(tokei.New(), data, "tokei")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used.Info().Name != "tokei" {
		t.Errorf("expected used provider tokei, got %q", used.Info().Name)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
}

func TestParseStdinWithProvider_AutoDetectSCC(t *testing.T) {
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

	result, used, err := parseStdinWithProvider(tokei.New(), data, "tokei")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used.Info().Name != "scc" {
		t.Errorf("expected auto-detected provider scc, got %q", used.Info().Name)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
}

func TestParseStdinWithProvider_ExplicitProviderNoAutoDetect(t *testing.T) {
	// Pass tokei-shaped data to the explicitly selected scc provider.
	data := []byte(`{
		"Go": {
			"blanks": 5,
			"code": 20,
			"comments": 5,
			"reports": []
		}
	}`)

	_, _, err := parseStdinWithProvider(scc.New(), data, "scc")
	if err == nil {
		t.Fatal("expected error when explicit scc provider receives tokei data")
	}
}

func TestParseStdinWithProvider_UnrecognizedFormat(t *testing.T) {
	_, _, err := parseStdinWithProvider(tokei.New(), []byte(`not json`), "tokei")
	if err == nil {
		t.Fatal("expected error for unrecognized stdin format")
	}
}

// Ensure the concrete providers satisfy the abstract interface at compile time.
var _ provider.Provider = tokei.New()
var _ provider.Provider = scc.New()

// Verify the real app command exposes the provider flag for resolveProvider.
var _ = appCmd.Flags().Lookup("provider")

func TestSplitRange(t *testing.T) {
	tests := []struct {
		rangeSpec      string
		wantS1, wantS2 string
		wantWorktree   bool
	}{
		{"main...HEAD", "main", "HEAD", false},
		{"main..HEAD", "main", "HEAD", false},
		{"v1.0..v2.0", "v1.0", "v2.0", false},
		{"main...", "main", "HEAD", false},
		{"..HEAD", "HEAD", "HEAD", false},
		{"HEAD", "HEAD", "", true},
		{"HEAD~3", "HEAD~3", "", true},
		{"abc123", "abc123", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.rangeSpec, func(t *testing.T) {
			s1, s2, worktree := splitRange(tt.rangeSpec)
			if s1 != tt.wantS1 || s2 != tt.wantS2 || worktree != tt.wantWorktree {
				t.Errorf("splitRange(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.rangeSpec, s1, s2, worktree, tt.wantS1, tt.wantS2, tt.wantWorktree)
			}
		})
	}
}

func TestReanchorResult(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	result := provider.Result{Files: []provider.FileStats{
		{Path: filepath.Join(srcRoot, "src", "main.go"), Language: "Go", Code: 10},
		{Path: "relative.go", Language: "Go", Code: 5},
	}}

	got := reanchorResult(result, srcRoot, dstRoot)

	if want := filepath.Join(dstRoot, "src", "main.go"); got.Files[0].Path != want {
		t.Errorf("expected absolute path reanchored to %q, got %q", want, got.Files[0].Path)
	}
	if want := filepath.Join(dstRoot, "relative.go"); got.Files[1].Path != want {
		t.Errorf("expected relative path reanchored to %q, got %q", want, got.Files[1].Path)
	}
}

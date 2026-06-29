package cmd

import (
	"testing"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/provider/scc"
	"github.com/zdyxry/tokui/tokei"
)

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

func TestParseStdinWithProvider_TokeiData(t *testing.T) {
	providerName = "tokei"
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

	result, used, err := parseStdinWithProvider(tokei.New(), data)
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
	providerName = "tokei"
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

	result, used, err := parseStdinWithProvider(tokei.New(), data)
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
	providerName = "scc"
	// Pass tokei-shaped data to the explicitly selected scc provider.
	data := []byte(`{
		"Go": {
			"blanks": 5,
			"code": 20,
			"comments": 5,
			"reports": []
		}
	}`)

	_, _, err := parseStdinWithProvider(scc.New(), data)
	if err == nil {
		t.Fatal("expected error when explicit scc provider receives tokei data")
	}
}

func TestParseStdinWithProvider_UnrecognizedFormat(t *testing.T) {
	providerName = "tokei"
	_, _, err := parseStdinWithProvider(tokei.New(), []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for unrecognized stdin format")
	}
}

// Ensure the concrete providers satisfy the abstract interface at compile time.
var _ provider.Provider = tokei.New()
var _ provider.Provider = scc.New()

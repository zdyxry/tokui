package tokei

import (
	"testing"

	"github.com/zdyxry/tokui/provider"
)

var _ provider.Provider = (*TokeiProvider)(nil)

func TestProviderInterface(t *testing.T) {
	p := New()
	info := p.Info()
	if info.Name != "tokei" {
		t.Errorf("expected provider name tokei, got %q", info.Name)
	}
	if info.Capabilities != provider.CapLines {
		t.Errorf("expected CapLines, got %v", info.Capabilities)
	}
}

func TestParseReport_DirectFormat(t *testing.T) {
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

	report, err := parseReport(data)
	if err != nil {
		t.Fatalf("parseReport failed: %v", err)
	}

	goStats, ok := report["Go"]
	if !ok {
		t.Fatal("expected Go stats in report")
	}
	if goStats.Blanks != 5 || goStats.Code != 20 || goStats.Comments != 5 {
		t.Errorf("Go stats mismatch: got %+v", goStats)
	}
	if len(goStats.Reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(goStats.Reports))
	}
	if goStats.Reports[0].Name != "main.go" {
		t.Errorf("expected report name main.go, got %q", goStats.Reports[0].Name)
	}
}

func TestParseReport_NestedFormat(t *testing.T) {
	data := []byte(`{
		"project": {
			"Go": {
				"blanks": 3,
				"code": 10,
				"comments": 2,
				"reports": [
					{
						"name": "foo.go",
						"stats": {"blanks": 3, "code": 10, "comments": 2}
					}
				]
			}
		}
	}`)

	report, err := parseReport(data)
	if err != nil {
		t.Fatalf("parseReport failed: %v", err)
	}

	goStats, ok := report["Go"]
	if !ok {
		t.Fatal("expected Go stats in report")
	}
	if goStats.Code != 10 {
		t.Errorf("expected Go code 10, got %d", goStats.Code)
	}
}

func TestParseReport_InvalidJSON(t *testing.T) {
	data := []byte(`not json`)
	_, err := parseReport(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseReport_EmptyObject(t *testing.T) {
	data := []byte(`{}`)
	report, err := parseReport(data)
	if err != nil {
		t.Fatalf("parseReport({}) failed: %v", err)
	}
	if len(report) != 0 {
		t.Errorf("expected empty report, got %d languages", len(report))
	}
}

func TestToProviderResult(t *testing.T) {
	report := LanguageReport{
		"Go": {
			Code:     20,
			Comments: 5,
			Blanks:   5,
			Reports: []FileReport{
				{Name: "main.go", Stats: InnerStats{Code: 20, Comments: 5, Blanks: 5}},
			},
		},
		"Total": {
			Code: 20,
		},
	}

	result := toProviderResult(report)
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	f := result.Files[0]
	if f.Path != "main.go" || f.Language != "Go" || f.Code != 20 {
		t.Errorf("unexpected file stats: %+v", f)
	}
}

func TestParseStdin_ValidInput(t *testing.T) {
	input := []byte(`{
		"Python": {
			"blanks": 1,
			"code": 5,
			"comments": 1,
			"reports": [
				{
					"name": "script.py",
					"stats": {"blanks": 1, "code": 5, "comments": 1}
				}
			]
		}
	}`)

	p := New()
	result, err := p.ParseStdin(input)
	if err != nil {
		t.Fatalf("ParseStdin failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	f := result.Files[0]
	if f.Language != "Python" || f.Code != 5 {
		t.Errorf("unexpected file stats: %+v", f)
	}
}

func TestParseStdin_EmptyInput(t *testing.T) {
	p := New()
	_, err := p.ParseStdin([]byte{})
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
}

func TestGetVersion_ParseOutput(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"tokei 12.1.0\n", "12.1.0", false},
		{"tokei 12.1.0", "12.1.0", false},
		{"unexpected", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := parseVersionOutput([]byte(tt.input))
		if (err != nil) != tt.wantErr {
			t.Errorf("parseVersionOutput(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseVersionOutput(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

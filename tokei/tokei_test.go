package tokei

import (
	"os"
	"testing"
)

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

func TestAnalyzeFromStdin_ValidInput(t *testing.T) {
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

	report, err := AnalyzeFromStdin()
	if err != nil {
		t.Fatalf("AnalyzeFromStdin failed: %v", err)
	}

	pyStats, ok := report["Python"]
	if !ok {
		t.Fatal("expected Python stats in report")
	}
	if pyStats.Code != 5 {
		t.Errorf("expected Python code 5, got %d", pyStats.Code)
	}
}

func TestAnalyzeFromStdin_EmptyInput(t *testing.T) {
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
		_, _ = w.Write([]byte{})
	}()

	_, err = AnalyzeFromStdin()
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

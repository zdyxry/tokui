package render

import (
	"errors"
	"strings"
	"testing"
)

func TestReportError(t *testing.T) {
	err := errors.New("something went wrong")
	out := ReportError(err, nil)

	if out == "" {
		t.Fatalf("ReportError() returned empty string")
	}
	if !strings.Contains(out, "something went wrong") {
		t.Errorf("ReportError() output missing error message")
	}
	if !strings.Contains(out, headerMessage) {
		t.Errorf("ReportError() output missing header message")
	}
}

func TestFmtStackTrace(t *testing.T) {
	t.Run("returns no stack trace data for empty input", func(t *testing.T) {
		got := fmtStackTrace(nil)
		if got != "no stack trace data" {
			t.Errorf("fmtStackTrace(nil) = %q, want %q", got, "no stack trace data")
		}
	})

	t.Run("trims content after cobra", func(t *testing.T) {
		input := []byte("main.run\nruntime.go\ngithub.com/spf13/cobra/command.go\nother")
		got := fmtStackTrace(input)

		if strings.Contains(got, "cobra") {
			t.Errorf("output should not contain cobra stack frame")
		}
		if !strings.Contains(got, "main.run") {
			t.Errorf("output missing expected application frames")
		}
	})

	t.Run("extracts content from panic onward", func(t *testing.T) {
		input := []byte("runtime error\n\npanic: oh no\nfoo.go:1\nbar.go:2\n")
		got := fmtStackTrace(input)

		if strings.Contains(got, "panic:") {
			t.Errorf("output should not contain the panic line itself")
		}
		if !strings.Contains(got, "foo.go:1") || !strings.Contains(got, "bar.go:2") {
			t.Errorf("output missing frames after panic")
		}
	})
}

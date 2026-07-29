package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPreprocessDiffArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"no args", nil, []string{}},
		{"bare diff", []string{"--diff"}, []string{"--diff"}},
		{"bare diff at end", []string{"subdir", "--diff"}, []string{"subdir", "--diff"}},
		{"space value", []string{"--diff", "HEAD~3"}, []string{"--diff=HEAD~3"}},
		{"equals form untouched", []string{"--diff=HEAD~3"}, []string{"--diff=HEAD~3"}},
		{"equals empty untouched", []string{"--diff="}, []string{"--diff="}},
		{"value then subdir", []string{"--diff", "HEAD~3", "subdir"}, []string{"--diff=HEAD~3", "subdir"}},
		{"positional before flag", []string{"subdir", "--diff", "main...HEAD"}, []string{"subdir", "--diff=main...HEAD"}},
		{"next token is a flag", []string{"--diff", "--tree"}, []string{"--diff", "--tree"}},
		{"next token is terminator", []string{"--diff", "--", "x"}, []string{"--diff", "--", "x"}},
		{"after terminator untouched", []string{"--", "--diff", "x"}, []string{"--", "--diff", "x"}},
		{"after terminator with prior flags", []string{"--tree", "--", "--diff", "x"}, []string{"--tree", "--", "--diff", "x"}},
		{"other flags untouched", []string{"--compare", "a..b"}, []string{"--compare", "a..b"}},
		{"double diff", []string{"--diff", "A", "--diff", "B"}, []string{"--diff=A", "--diff=B"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preprocessDiffArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("preprocessDiffArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("preprocessDiffArgs(%v) = %v, want %v", tt.args, got, tt.want)
				}
			}
		})
	}
}

// parseOnlyCmd returns a fresh root command whose RunE only records the
// positional args, so flag parsing can be tested without entering the TUI.
func parseOnlyCmd(captured *[]string) *cobra.Command {
	cmd := newAppCmd()
	cmd.RunE = func(_ *cobra.Command, args []string) error {
		*captured = args
		return nil
	}
	return cmd
}

func TestCobraFlagParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantDiff    string
		wantCompare string
		wantRef     string
		wantArgs    []string
	}{
		{"diff space value", []string{"--diff", "HEAD~3"}, "HEAD~3", "", "", nil},
		{"bare diff means HEAD", []string{"--diff"}, "HEAD", "", "", nil},
		{"diff equals form", []string{"--diff=HEAD~3"}, "HEAD~3", "", "", nil},
		{"diff value and subdir", []string{"--diff", "HEAD~3", "subdir"}, "HEAD~3", "", "", []string{"subdir"}},
		{"bare diff and subdir", []string{"--diff", "subdir"}, "subdir", "", "", nil}, // see note below
		{"compare", []string{"--compare", "a..b"}, "", "a..b", "", nil},
		{"ref", []string{"--ref", "v1.0"}, "", "", "v1.0", nil},
		{"ref with dir", []string{"--ref", "v1.0", "subdir"}, "", "", "v1.0", []string{"subdir"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			cmd := parseOnlyCmd(&gotArgs)
			cmd.SetArgs(preprocessDiffArgs(tt.args))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if diffRange != tt.wantDiff {
				t.Errorf("diffRange = %q, want %q", diffRange, tt.wantDiff)
			}
			if compareRange != tt.wantCompare {
				t.Errorf("compareRange = %q, want %q", compareRange, tt.wantCompare)
			}
			if refName != tt.wantRef {
				t.Errorf("refName = %q, want %q", refName, tt.wantRef)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("positional args = %v, want %v", gotArgs, tt.wantArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("positional args = %v, want %v", gotArgs, tt.wantArgs)
				}
			}
		})
	}
}

// Note on "--diff subdir": by design a token after --diff is consumed as the
// range value; scoping to a subdirectory requires a range first
// ("--diff HEAD~3 subdir") or the equals form. This pins the behavior.

func TestCobraGitFlagsMutuallyExclusive(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"diff and compare", []string{"--diff", "HEAD", "--compare", "a..b"}},
		{"compare and ref", []string{"--compare", "a..b", "--ref", "v1"}},
		{"diff and ref", []string{"--diff=HEAD", "--ref", "v1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			cmd := parseOnlyCmd(&gotArgs)
			cmd.SetArgs(preprocessDiffArgs(tt.args))
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected mutual exclusion error")
			}
			if !strings.Contains(err.Error(), "if any flags in the group") {
				t.Errorf("unexpected error: %v", err)
			}
			var cliErr *CLIError
			if errors.As(err, &cliErr) {
				t.Errorf("cobra validation error should not be a CLIError: %v", err)
			}
		})
	}
}

func TestRunAppPipeWithGitFlagRejected(t *testing.T) {
	for _, flag := range []string{"diff", "compare", "ref"} {
		t.Run(flag, func(t *testing.T) {
			cmd := newAppCmd() // also resets the package-level flag variables
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			oldStdin := os.Stdin
			os.Stdin = r
			defer func() {
				os.Stdin = oldStdin
				r.Close()
				w.Close()
			}()

			switch flag {
			case "diff":
				diffRange = "HEAD"
			case "compare":
				compareRange = "a..b"
			case "ref":
				refName = "v1"
			}
			defer func() { diffRange, compareRange, refName = "", "", "" }()

			err = runApp(cmd, nil)
			if err == nil {
				t.Fatal("expected pipe/git-flag conflict error")
			}
			var cliErr *CLIError
			if !errors.As(err, &cliErr) {
				t.Errorf("expected CLIError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), "cannot be combined with pipe mode") {
				t.Errorf("unexpected message: %v", err)
			}
		})
	}
}

func TestGitModesWithoutGitBinary(t *testing.T) {
	// An empty PATH makes every git invocation fail with "not found".
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()

	t.Run("diff", func(t *testing.T) {
		err := runGitModeExpectingInstallHint(t, func() error {
			_, err := runDiffMode(nil, stubProvider{}, dir, "HEAD")
			return err
		})
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("compare", func(t *testing.T) {
		err := runGitModeExpectingInstallHint(t, func() error {
			_, err := runCompareMode(nil, stubProvider{}, dir, "a..b")
			return err
		})
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("ref", func(t *testing.T) {
		err := runGitModeExpectingInstallHint(t, func() error {
			_, _, err := runRefMode(nil, stubProvider{}, dir, "v1")
			return err
		})
		if err != nil {
			t.Error(err)
		}
	})
}

// runGitModeExpectingInstallHint asserts the git-missing error is a concise,
// single-line CLIError carrying the install hint (no crash report, no
// duplicated hint).
func runGitModeExpectingInstallHint(t *testing.T, run func() error) error {
	t.Helper()
	err := run()
	if err == nil {
		return errors.New("expected an error when git is not in PATH")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		return errors.New("expected CLIError, got " + err.Error())
	}
	msg := err.Error()
	if !strings.Contains(msg, "please install git") {
		return errors.New("expected install hint, got: " + msg)
	}
	if strings.Count(msg, "please install git") != 1 {
		return errors.New("install hint repeated: " + msg)
	}
	if strings.Contains(msg, "\n") || strings.Contains(msg, "Something went terribly wrong") {
		return errors.New("error should be a concise single line, got: " + msg)
	}
	return nil
}

package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// useDevNullStdin points os.Stdin at /dev/null (a character device) so the
// pipe-mode check passes and subcommand execution reaches mode dispatch.
func useDevNullStdin(t *testing.T) {
	t.Helper()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = devNull.Close()
	})
}

// usePipeStdin points os.Stdin at a pipe so the pipe-mode check triggers.
func usePipeStdin(t *testing.T) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
		_ = w.Close()
	})
}

func TestNewDiffSpec(t *testing.T) {
	tests := []struct {
		name     string
		rangeArg string
		staged   bool
		wantSpec diffSpec
	}{
		{
			name: "bare diff is worktree vs index",
			wantSpec: diffSpec{
				worktree:   true,
				rangeLabel: "worktree (unstaged)",
			},
		},
		{
			name:   "staged defaults to HEAD",
			staged: true,
			wantSpec: diffSpec{
				cached:     true,
				worktree:   true,
				rangeLabel: "--staged",
			},
		},
		{
			name:     "staged with rev",
			rangeArg: "HEAD~2",
			staged:   true,
			wantSpec: diffSpec{
				rev:        "HEAD~2",
				cached:     true,
				worktree:   true,
				rangeLabel: "HEAD~2 --staged",
			},
		},
		{
			name:     "single rev diffs against worktree",
			rangeArg: "HEAD~3",
			wantSpec: diffSpec{
				rev:        "HEAD~3",
				worktree:   true,
				rangeLabel: "HEAD~3",
			},
		},
		{
			name:     "two-dot range diffs two refs",
			rangeArg: "v1.0..v2.0",
			wantSpec: diffSpec{
				rev:        "v1.0..v2.0",
				s2Ref:      "v2.0",
				rangeLabel: "v1.0..v2.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newDiffSpec(tt.rangeArg, tt.staged); got != tt.wantSpec {
				t.Errorf("newDiffSpec(%q, %v) = %+v, want %+v", tt.rangeArg, tt.staged, got, tt.wantSpec)
			}
		})
	}
}

func TestRootPositionalDirectoryStillWorks(t *testing.T) {
	// With subcommands registered, a plain directory argument must still reach
	// the root command (not be rejected as an "unknown command").
	for _, args := range [][]string{{"somedir"}, {"--tree", "somedir"}} {
		var gotArgs []string
		cmd := newAppCmd()
		cmd.RunE = func(_ *cobra.Command, args []string) error {
			gotArgs = args
			return nil
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v): %v", args, err)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "somedir" {
			t.Errorf("Execute(%v): positional args = %v, want [somedir]", args, gotArgs)
		}
	}
}

func TestSubcommandPipeConflict(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"diff", []string{"diff", "HEAD"}},
		{"bare diff", []string{"diff"}},
		{"staged diff", []string{"diff", "--staged"}},
		{"show", []string{"show"}},
		{"compare", []string{"compare", "a..b"}},
		{"ref", []string{"ref", "v1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usePipeStdin(t)
			cmd := newAppCmd() // also resets the package-level flag variables
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected pipe conflict error")
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

func TestSubcommandTrailingDirectory(t *testing.T) {
	// A non-repo directory as the trailing positional argument must reach the
	// mode runner: the error names that directory, proving it overrode --root.
	dir := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{"diff", []string{"diff", "HEAD", dir}},
		{"show", []string{"show", "HEAD", dir}},
		{"compare", []string{"compare", "a..b", dir}},
		{"ref", []string{"ref", "v1", dir}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useDevNullStdin(t)
			cmd := newAppCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected not-a-repo error")
			}
			if !strings.Contains(err.Error(), "not a git repository: "+dir) {
				t.Errorf("expected error naming %q, got %v", dir, err)
			}
		})
	}
}

func TestDiffRangeArgumentParsed(t *testing.T) {
	// In a real repo, a bogus range argument must surface as a git revision
	// error (not "not a git repository"), proving the range/dir split.
	useDevNullStdin(t)
	repo := initCompareRepo(t)
	cmd := newAppCmd()
	cmd.SetArgs([]string{"diff", "no-such-rev", repo})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for bogus range")
	}
	if !strings.Contains(err.Error(), "no-such-rev") {
		t.Errorf("expected git revision error mentioning the range, got %v", err)
	}
}

func TestShowRejectsRangeArgument(t *testing.T) {
	useDevNullStdin(t)
	cmd := newAppCmd()
	cmd.SetArgs([]string{"show", "a..b"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected range rejection error")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Errorf("expected CLIError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "tokui diff a..b") {
		t.Errorf("expected a hint to use 'tokui diff', got %v", err)
	}
}

func TestCompareAndRefRequireArgument(t *testing.T) {
	for _, args := range [][]string{{"compare"}, {"ref"}} {
		t.Run(args[0], func(t *testing.T) {
			useDevNullStdin(t)
			cmd := newAppCmd()
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected missing-argument error")
			}
			// Cobra argument validation errors are plain errors, not CLIErrors.
			var cliErr *CLIError
			if errors.As(err, &cliErr) {
				t.Errorf("cobra validation error should not be a CLIError: %v", err)
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
			_, err := runDiffMode(nil, stubProvider{}, dir, newDiffSpec("HEAD", false))
			return err
		})
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("show", func(t *testing.T) {
		err := runGitModeExpectingInstallHint(t, func() error {
			_, err := runShowMode(nil, stubProvider{}, dir, "HEAD")
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

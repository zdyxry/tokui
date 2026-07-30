package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/provider/scc"
	"github.com/zdyxry/tokui/render"
	"github.com/zdyxry/tokui/structure"
	"github.com/zdyxry/tokui/tokei"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	ErrUnknown   = errors.New("unknown error")
	root         string
	treeMode     bool
	treemapMode  bool
	providerName string

	appCmd = newAppCmd()
)

// newAppCmd builds the root command. It is a factory (rather than init-time
// registration) so tests can construct fresh instances with clean flag state;
// the flags are still bound to the package-level variables above.
func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokui [directory]",
		Short: "An interactive terminal tool for visualizing code statistics.",
		// With subcommands registered, cobra's default args validation
		// (legacyArgs) would reject any positional argument as an "unknown
		// command"; accept arbitrary args so "tokui [directory]" keeps working.
		Args: cobra.ArbitraryArgs,
		Long: `
📊 A terminal-based user interface for visualizing and analyzing directory code statistics.

Usage:
  1. Pipe mode: tokei -o json [directory] | tokui
  2. Direct mode: tokui [directory] (requires tokei to be installed on the system)
  3. Git modes: tokui diff|show|compare|ref ... (run inside a git repository)

Pipe mode (recommended):
  tokei -o json . | tokui
  tokei -o json /path/to/project | tokui

Direct mode:
  tokui .
  tokui /path/to/project

Git modes:
  tokui diff [range] [directory]        Churn of a diff; bare = unstaged changes (like "git diff")
  tokui diff --staged [rev] [directory] Staged changes against [rev|HEAD] (like "git diff --cached")
  tokui show [commit] [directory]       Churn of a single commit (default HEAD, like "git show")
  tokui compare <a..b> [directory]      Net statistics change between two refs
  tokui ref <ref> [directory]           Browse the full snapshot of a single ref

Note:
- Pipe mode requires running the 'tokei' command separately
- Direct mode requires 'tokei' to be installed and available in your system PATH
- Install tokei: https://github.com/XAMPPRocky/tokei

🔗 Learn more: https://github.com/zdyxry/tokui`,
		// Errors are printed by Execute: expected user errors (CLIError and
		// flag parsing/validation failures) plainly, anything else as a crash
		// report. Cobra's own full-screen usage dump is silenced.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runApp(cmd, args); err != nil {
				return &runError{err: err}
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVarP(
		&root,
		"root",
		"r",
		".",
		`Specify the root directory to analyze. Defaults to current directory.`,
	)
	cmd.PersistentFlags().BoolVarP(
		&treeMode,
		"tree",
		"t",
		false,
		`Start in tree mode (expandable directories instead of navigation).`,
	)
	cmd.PersistentFlags().BoolVar(
		&treemapMode,
		"treemap",
		false,
		`Start in treemap mode (proportional blocks instead of a table).`,
	)
	cmd.PersistentFlags().StringVar(
		&providerName,
		"provider",
		"tokei",
		`Stats provider: tokei|scc. Defaults to "tokei"; can be overridden with the TOKUI_PROVIDER environment variable.`,
	)
	cmd.MarkFlagsMutuallyExclusive("tree", "treemap")
	cmd.AddCommand(newDiffCmd(), newShowCmd(), newCompareCmd(), newRefCmd())
	return cmd
}

// newDiffCmd builds the "diff" subcommand: churn for a git diff. A bare
// invocation diffs the worktree against the index (unstaged changes, like
// "git diff"); --staged diffs the index against [rev|HEAD] (like
// "git diff --cached"); a range argument diffs that rev/range directly.
func newDiffCmd() *cobra.Command {
	var staged bool
	cmd := &cobra.Command{
		Use:   "diff [range] [directory]",
		Short: "Show code churn for a git diff.",
		Long: `Show code churn for a git diff.

With no range, shows unstaged changes (worktree vs index, like "git diff").
With --staged, shows staged changes against [rev] (default HEAD, like
"git diff --cached"). With a range argument ("HEAD~3", "main...HEAD"),
shows the churn of that range.

An optional trailing directory scopes the change set to a subdirectory of
the repository (overrides --root).`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var rangeArg, dir string
			if len(args) > 0 {
				rangeArg = args[0]
			}
			if len(args) > 1 {
				dir = args[1]
			}
			if err := runDiffCmd(cmd, rangeArg, dir, staged); err != nil {
				return &runError{err: err}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(
		&staged,
		"staged",
		false,
		`Diff the staged (index) changes against [rev] (default HEAD), like "git diff --cached".`,
	)
	return cmd
}

// newShowCmd builds the "show" subcommand: the churn of a single commit
// against its parent (like "git show").
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [commit] [directory]",
		Short: "Show code churn of a single commit.",
		Long: `Show code churn of a single commit (default HEAD) against its
parent, like "git show <commit>". A root commit is diffed against the
empty tree, so all of its files show up as added.

An optional trailing directory scopes the change set to a subdirectory of
the repository (overrides --root).`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commit := "HEAD"
			var dir string
			if len(args) > 0 {
				commit = args[0]
			}
			if len(args) > 1 {
				dir = args[1]
			}
			if strings.Contains(commit, "..") {
				return &runError{err: NewCLIError(fmt.Errorf(
					"%q looks like a range; 'tokui show' takes a single commit — use 'tokui diff %s' for ranges",
					commit, commit))}
			}
			if err := runShowCmd(cmd, commit, dir); err != nil {
				return &runError{err: err}
			}
			return nil
		},
	}
}

// newCompareCmd builds the "compare" subcommand: net statistics change
// between two refs.
func newCompareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <a..b> [directory]",
		Short: "Compare statistics between two git refs.",
		Long: `Compare code statistics between two git refs, e.g. "v1.0..v2.0"
or "main...HEAD". An optional trailing directory overrides --root.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var dir string
			if len(args) > 1 {
				dir = args[1]
			}
			if err := runCompareCmd(cmd, args[0], dir); err != nil {
				return &runError{err: err}
			}
			return nil
		},
	}
}

// newRefCmd builds the "ref" subcommand: browse the full snapshot of a
// single ref.
func newRefCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ref <ref> [directory]",
		Short: "Show statistics for a single git ref snapshot.",
		Long: `Show statistics for a single git ref snapshot, e.g. "v1.0". The
ref is unpacked into a temporary directory and analyzed like a normal
directory. An optional trailing directory overrides --root.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var dir string
			if len(args) > 1 {
				dir = args[1]
			}
			if err := runRefCmd(cmd, args[0], dir); err != nil {
				return &runError{err: err}
			}
			return nil
		},
	}
}

// Execute runs the root command. version is the version string to report via
// the --version/-v flag; it is resolved against Go build info when it is empty
// or the "dev" placeholder.
func Execute(version string) {
	appCmd.Version = resolveVersion(version)
	appCmd.SetArgs(os.Args[1:])
	if err := appCmd.Execute(); err != nil {
		var cliErr *CLIError
		var rErr *runError
		switch {
		case errors.As(err, &cliErr):
			// Expected user error: print the message as-is.
			printError(cliErr.Error())
		case errors.As(err, &rErr):
			// An unexpected error escaped runApp: full crash report.
			printError(render.ReportError(rErr.err, debug.Stack()))
		default:
			// Flag parsing/validation failed before the command ran; that is
			// also a user error, not a crash.
			printError("Error: " + err.Error())
		}
		os.Exit(1)
	}
}

// resolveVersion returns the version to report. It prefers an explicit,
// build-injected version and otherwise falls back to Go module build info so
// that binaries produced by `go install ...@version` still report something
// useful instead of a bare "dev".
func resolveVersion(version string) string {
	if version != "" && version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var revision, suffix string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				suffix = "-dirty"
			}
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		return "dev+" + revision + suffix
	}

	return "dev"
}

func runApp(cmd *cobra.Command, args []string) error {
	defer recoverCrash()

	selectedProvider := resolveProvider(cmd)
	p, err := selectProvider(selectedProvider)
	if err != nil {
		return err
	}

	// Check if there is stdin input (pipe mode)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to check standard input: %w", err)
	}
	isPipe := (stat.Mode() & os.ModeCharDevice) == 0

	// A positional argument overrides --root in every non-pipe mode.
	if len(args) > 0 {
		root = args[0]
	}
	analysisPath := filepath.Clean(root)

	tree := structure.NewTree(nil)
	modeInfo := render.ModeInfo{Kind: render.ModeFull}

	if isPipe {
		// If there is pipe input, use pipe mode
		if err := runPipeMode(tree, p, selectedProvider); err != nil {
			return fmt.Errorf("error reading provider output from pipe: %w", err)
		}
	} else {
		// Direct mode: need to specify directory

		// Validate the path before shelling out to the provider so users get a
		// clear message instead of a raw provider failure and stack trace.
		if _, statErr := os.Stat(analysisPath); statErr != nil {
			switch {
			case errors.Is(statErr, os.ErrNotExist):
				printError(fmt.Sprintf("Path %q does not exist. Please provide a valid file or directory to analyze.", analysisPath))
			case errors.Is(statErr, os.ErrPermission):
				printError(fmt.Sprintf("Permission denied accessing %q. Please check its permissions.", analysisPath))
			default:
				printError(fmt.Sprintf("Cannot access %q: %v", analysisPath, statErr))
			}
			os.Exit(1)
		}

		if err := tree.BuildFromProvider(p, analysisPath); err != nil {
			// Provide a more friendly error message if the provider binary is not installed
			if strings.Contains(err.Error(), "executable file not found") {
				var pipeExample string
				if p.Info().Name == "scc" {
					pipeExample = "scc --by-file -f json . | tokui"
				} else {
					pipeExample = "tokei -o json . | tokui"
				}
				errMsg := fmt.Sprintf("Command '%s' not found. Please install it and ensure it's in your system PATH environment variable.\n"+
					"Or use pipe mode: %s", p.Info().Name, pipeExample)
				printError(errMsg)
				os.Exit(1)
			}
			return fmt.Errorf("error during analysis with %s: %w", p.Info().Name, err)
		}
	}

	return launchTUI(p, tree, modeInfo)
}

// recoverCrash renders a panic as a crash report instead of a raw stack
// dump. It is meant to be deferred at the top of a command runner.
func recoverCrash() {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("unknown panic: %v", r)
		}
		printError(render.ReportError(err, debug.Stack()))
	}
}

// launchTUI initializes the view model for the built tree and runs the
// Bubble Tea program.
func launchTUI(p provider.Provider, tree *structure.Tree, modeInfo render.ModeInfo) error {
	// Initialize view model
	info := p.Info()
	if modeInfo.Diff() {
		// Diff mode adds the churn columns on top of the provider's own
		// capabilities (see design-git-diff.md §4.5).
		info.Capabilities |= provider.CapChurn
	}
	if modeInfo.Compare() {
		info.Capabilities |= provider.CapDelta
	}
	vm, err := initViewModel(tree, info, modeInfo, treeMode, treemapMode)
	if err != nil {
		return err
	}

	// Create and run the Bubble Tea program
	teaProg := tea.NewProgram(
		vm,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutCatchPanics(),
	)

	if _, err = teaProg.Run(); err != nil {
		return err
	}

	return nil
}

// runGitMode is the shared driver for the git subcommands: it resolves the
// provider, rejects pipe mode (the TUI needs the terminal), resolves the
// analysis directory (the subcommand's positional directory overrides
// --root) and hands the mode builder a fresh tree. A non-nil cleanup keeps
// temporary archives alive until the TUI exits (previews read them).
func runGitMode(cmd *cobra.Command, name, dir string, build func(tree *structure.Tree, p provider.Provider, path string) (render.ModeInfo, func(), error)) error {
	defer recoverCrash()

	selectedProvider := resolveProvider(cmd)
	p, err := selectProvider(selectedProvider)
	if err != nil {
		return err
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to check standard input: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return NewCLIError(fmt.Errorf("'tokui %s' cannot be combined with pipe mode (stdin); run 'tokui %s' in a terminal inside a git repository", name, name))
	}

	if dir == "" {
		dir = root
	}
	analysisPath := filepath.Clean(dir)

	tree := structure.NewTree(nil)
	modeInfo, cleanup, err := build(tree, p, analysisPath)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	return launchTUI(p, tree, modeInfo)
}

// runDiffCmd drives the "diff" subcommand.
func runDiffCmd(cmd *cobra.Command, rangeArg, dir string, staged bool) error {
	spec := newDiffSpec(rangeArg, staged)
	return runGitMode(cmd, "diff", dir, func(tree *structure.Tree, p provider.Provider, path string) (render.ModeInfo, func(), error) {
		mode, err := runDiffMode(tree, p, path, spec)
		return mode, nil, err
	})
}

// runShowCmd drives the "show" subcommand.
func runShowCmd(cmd *cobra.Command, commit, dir string) error {
	return runGitMode(cmd, "show", dir, func(tree *structure.Tree, p provider.Provider, path string) (render.ModeInfo, func(), error) {
		mode, err := runShowMode(tree, p, path, commit)
		return mode, nil, err
	})
}

// runCompareCmd drives the "compare" subcommand.
func runCompareCmd(cmd *cobra.Command, rangeArg, dir string) error {
	return runGitMode(cmd, "compare", dir, func(tree *structure.Tree, p provider.Provider, path string) (render.ModeInfo, func(), error) {
		mode, err := runCompareMode(tree, p, path, rangeArg)
		return mode, nil, err
	})
}

// runRefCmd drives the "ref" subcommand.
func runRefCmd(cmd *cobra.Command, ref, dir string) error {
	return runGitMode(cmd, "ref", dir, func(tree *structure.Tree, p provider.Provider, path string) (render.ModeInfo, func(), error) {
		return runRefMode(tree, p, path, ref)
	})
}

// selectProvider returns the Provider implementation matching the given name.
func selectProvider(name string) (provider.Provider, error) {
	switch name {
	case "tokei":
		return tokei.New(), nil
	case "scc":
		return scc.New(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// resolveProvider returns the effective provider name using the precedence:
// 1. Explicit --provider flag, 2. TOKUI_PROVIDER environment variable,
// 3. Default "tokei".
func resolveProvider(cmd *cobra.Command) string {
	if cmd.Flags().Changed("provider") {
		return providerName
	}
	if env := os.Getenv("TOKUI_PROVIDER"); env != "" {
		return env
	}
	return "tokei"
}

// runPipeMode reads stdin once and either uses the selected provider or
// attempts to auto-detect the format.
func runPipeMode(tree *structure.Tree, p provider.Provider, explicitProvider string) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	result, _, err := parseStdinWithProvider(p, data, explicitProvider)
	if err != nil {
		return err
	}

	// Pipe mode has no explicit analysis root; use the current directory so
	// absolute paths from the provider output can be normalized relative to it.
	return tree.BuildFromProviderResult(result, ".")
}

// parseStdinWithProvider tries to parse stdin data with the requested provider.
// If parsing fails and the user kept the default "tokei" provider, it attempts
// auto-detection across all known providers before returning a clear error.
func parseStdinWithProvider(p provider.Provider, data []byte, explicitProvider string) (provider.Result, provider.Provider, error) {
	result, err := p.ParseStdin(data)
	if err == nil {
		return result, p, nil
	}

	// If the user explicitly selected a non-default provider, do not auto-detect.
	if explicitProvider != "tokei" {
		return provider.Result{}, nil, fmt.Errorf(
			"stdin could not be parsed with provider %q: %w; ensure the pipe output matches the provider's expected format",
			p.Info().Name, err,
		)
	}

	// Auto-detect: try every other known provider.
	candidates := []provider.Provider{tokei.New(), scc.New()}
	for _, candidate := range candidates {
		if candidate.Info().Name == p.Info().Name {
			continue
		}
		result, err := candidate.ParseStdin(data)
		if err == nil {
			return result, candidate, nil
		}
	}

	return provider.Result{}, nil, fmt.Errorf(
		"unrecognized stdin format; expected tokei JSON (tokei -o json ...) or scc JSON (scc --by-file -f json ...)",
	)
}

func initViewModel(tree *structure.Tree, info provider.Info, modeInfo render.ModeInfo, treeMode, treemapMode bool) (*render.ViewModel, error) {
	nav := render.NewCodeNavigation(tree)
	dirModel := render.NewDirModelWithMode(nav, info, modeInfo, treeMode, treemapMode)
	vm := render.NewViewModel(
		nav,
		dirModel,
	)
	vm.Update(render.ScanFinished{})
	return vm, nil
}

// diffSpec describes one "tokui diff" invocation: which rev to ask git for,
// how to build the S2 snapshot and how to label both sides for the status
// bar and the S1/S2 previews.
type diffSpec struct {
	rev        string // rev/range for the numstat diff; "" = worktree vs index (unstaged)
	cached     bool   // --staged: diff the index against rev (default HEAD)
	s1Ref      string // S1 preview ref for "git show"; "" = the index (staged blob)
	s1Label    string // human-readable S1 marker, e.g. "main" or "index"
	s2Ref      string // S2 preview ref; "" = the working tree
	s2Label    string // human-readable S2 marker, e.g. "worktree"
	worktree   bool   // analyze the worktree as S2 (vs archiving s2Ref)
	rangeLabel string // status-bar label for the diff
}

// newDiffSpec builds the diffSpec for "tokui diff [range]": a bare
// invocation diffs the worktree against the index (unstaged changes, like
// "git diff"); with --staged it diffs the index against [rev|HEAD] (like
// "git diff --cached"); a range argument keeps the classic rev/range form
// (a single rev diffs against the worktree, "a..b"/"a...b" diff two refs).
func newDiffSpec(rangeArg string, staged bool) diffSpec {
	if staged {
		s1 := rangeArg
		if s1 == "" {
			s1 = "HEAD"
		}
		label := "--staged"
		if rangeArg != "" {
			label = rangeArg + " --staged"
		}
		// The true S2 here is the index, which has no filesystem tree, so
		// the worktree is analyzed as the S2 approximation.
		return diffSpec{
			rev:        rangeArg,
			cached:     true,
			s1Ref:      s1,
			s1Label:    s1,
			s2Label:    "worktree",
			worktree:   true,
			rangeLabel: label,
		}
	}
	if rangeArg == "" {
		return diffSpec{
			s1Label:    "index",
			s2Label:    "worktree",
			worktree:   true,
			rangeLabel: "worktree (unstaged)",
		}
	}
	s1, s2, worktree := splitRange(rangeArg)
	s2Label := s2
	if worktree {
		s2Label = "worktree"
	}
	return diffSpec{
		rev:        rangeArg,
		s1Ref:      s1,
		s1Label:    s1,
		s2Ref:      s2,
		s2Label:    s2Label,
		worktree:   worktree,
		rangeLabel: rangeArg,
	}
}

// runDiffMode builds the tree for "tokui diff": it locates the repository,
// collects the churn described by spec with git, analyzes the S2 snapshot
// with the current provider and joins both into the tree. When path points
// into a subdirectory of the repository, the change set is scoped to that
// subdirectory (the S2 analysis still covers the whole repository). It
// returns the ModeInfo the view layer needs for the status bar and S1/S2
// previews.
func runDiffMode(tree *structure.Tree, p provider.Provider, path string, spec diffSpec) (render.ModeInfo, error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\nHint: run 'tokui' without a subcommand to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, NewCLIError(err)
	}

	changes, err := gitx.Numstat(repoRoot, spec.rev, spec.cached)
	if err != nil {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\n\nExamples:\n  tokui diff main...HEAD\n  tokui diff HEAD~3\n  tokui diff --staged", err))
	}
	changes = scopeChangesToSubdir(changes, repoRoot, path)

	s1Ref := spec.s1Ref
	if !spec.cached && strings.Contains(spec.rev, "...") {
		// "git diff A...B" diffs the merge-base of A and B against B; resolve
		// the merge-base so S1 previews ("git show S1:path") match the diff
		// base. Fall back to A when no common ancestor exists.
		s1, s2, _ := splitRange(spec.rev)
		if base, mbErr := gitx.MergeBase(repoRoot, s1, s2); mbErr == nil {
			s1Ref = base
		}
	}

	var s2 provider.Result
	if spec.worktree {
		// The diff target is the working tree: analyze it in place. (In
		// --staged mode the true S2 is the index, which has no filesystem
		// tree; the worktree is the closest analyzable approximation.)
		s2, err = p.Analyze(repoRoot)
	} else {
		// Historical S2: unpack the ref into a temporary directory. The
		// archive is only needed for the analysis itself (S2 previews go
		// through "git show"), so it is cleaned up when this function returns.
		var cleanup func()
		s2, cleanup, err = analyzeRef(p, repoRoot, spec.s2Ref)
		if cleanup != nil {
			defer cleanup()
		}
	}
	if err != nil {
		return render.ModeInfo{}, fmt.Errorf("error during analysis with %s: %w", p.Info().Name, err)
	}

	if err := tree.BuildFromDiff(changes, s2, repoRoot); err != nil {
		return render.ModeInfo{}, err
	}

	return render.ModeInfo{
		Kind:     render.ModeDiff,
		Range:    spec.rangeLabel,
		RepoRoot: repoRoot,
		S1Ref:    s1Ref,
		S1Label:  spec.s1Label,
		S2Ref:    spec.s2Ref,
		S2Label:  spec.s2Label,
	}, nil
}

// runShowMode builds the tree for "tokui show": the churn of a single commit
// against its parent, presented as a diff mode over the archived commit
// snapshot. Root commits are diffed against the empty tree (see
// gitx.ShowRange), so every file comes out as added.
func runShowMode(tree *structure.Tree, p provider.Provider, path, commit string) (render.ModeInfo, error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\nHint: run 'tokui' without a subcommand to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, NewCLIError(err)
	}

	rangeSpec, s1Ref, s2Ref, err := gitx.ShowRange(repoRoot, commit)
	if err != nil {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\n\nExample:\n  tokui show HEAD", err))
	}

	return runDiffMode(tree, p, path, diffSpec{
		rev:        rangeSpec,
		s1Ref:      s1Ref,
		s1Label:    s1Ref,
		s2Ref:      s2Ref,
		s2Label:    s2Ref,
		rangeLabel: commit,
	})
}

// scopeChangesToSubdir narrows a repo-wide change set to the entries under
// path when path points into a subdirectory of the repository ("tokui diff
// HEAD~3 some/subdir"). A path at (or outside) the repository root leaves
// the change set untouched.
func scopeChangesToSubdir(changes []gitx.FileChange, repoRoot, path string) []gitx.FileChange {
	abs, err := filepath.Abs(path)
	if err != nil {
		return changes
	}
	// RepoRoot comes from git (already resolved); resolve path the same way
	// so symlinked temp dirs still compare equal.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return changes
	}
	rel = filepath.ToSlash(rel)
	prefix := rel + "/"
	inScope := func(p string) bool {
		return p == rel || strings.HasPrefix(p, prefix)
	}
	scoped := make([]gitx.FileChange, 0, len(changes))
	for _, c := range changes {
		if inScope(c.Path) || (c.OldPath != "" && inScope(c.OldPath)) {
			scoped = append(scoped, c)
		}
	}
	return scoped
}

// runCompareMode builds the tree for "tokui compare": both refs are analyzed
// with the current provider (archived to temporary directories, except a
// HEAD/worktree S2 which is analyzed in place) and joined by path via
// BuildFromCompare.
func runCompareMode(tree *structure.Tree, p provider.Provider, path, compareRange string) (render.ModeInfo, error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\nHint: run 'tokui' without a subcommand to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, NewCLIError(err)
	}

	s1Ref, s2Ref, worktree := splitRange(compareRange)
	if worktree {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("'tokui compare' requires two refs, e.g. \"v1.0..v2.0\" or \"main...HEAD\""))
	}

	s1, cleanup1, err := analyzeRef(p, repoRoot, s1Ref)
	if cleanup1 != nil {
		defer cleanup1()
	}
	if err != nil {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\n\nExample:\n  tokui compare v1.0..v2.0", err))
	}

	s2Worktree := s2Ref == "HEAD"
	var s2 provider.Result
	if s2Worktree {
		// S2 is HEAD: analyze the working tree in place.
		s2, err = p.Analyze(repoRoot)
	} else {
		var cleanup2 func()
		s2, cleanup2, err = analyzeRef(p, repoRoot, s2Ref)
		if cleanup2 != nil {
			defer cleanup2()
		}
	}
	if err != nil {
		return render.ModeInfo{}, fmt.Errorf("error during analysis with %s: %w", p.Info().Name, err)
	}

	if err := tree.BuildFromCompare(s1, s2, repoRoot); err != nil {
		return render.ModeInfo{}, err
	}

	rangeLabel := compareRange
	s2Label := s2Ref
	s2PreviewRef := s2Ref
	if s2Worktree {
		// S2 == HEAD compares against the working tree, uncommitted changes
		// included (design-git-diff.md §4.3); say so in the status bar.
		rangeLabel += " (worktree)"
		s2Label = "worktree"
		s2PreviewRef = ""
	}
	return render.ModeInfo{
		Kind:     render.ModeCompare,
		Range:    rangeLabel,
		RepoRoot: repoRoot,
		S1Ref:    s1Ref,
		S1Label:  s1Ref,
		S2Ref:    s2PreviewRef,
		S2Label:  s2Label,
	}, nil
}

// runRefMode builds the tree for "tokui ref": the ref's snapshot is unpacked
// into a temporary directory and analyzed with the familiar full view. The
// cleanup is returned to the caller because file previews read the archive
// for the whole TUI session.
func runRefMode(tree *structure.Tree, p provider.Provider, path, ref string) (render.ModeInfo, func(), error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, nil, NewCLIError(fmt.Errorf("%w\nHint: run 'tokui' without a subcommand to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, nil, NewCLIError(err)
	}

	dir, cleanup, err := archiveInRepo(repoRoot, ref)
	if err != nil {
		return render.ModeInfo{}, nil, NewCLIError(fmt.Errorf("%w\n\nExample:\n  tokui ref v1.0", err))
	}

	if err := tree.BuildFromProvider(p, dir); err != nil {
		cleanup()
		return render.ModeInfo{}, nil, fmt.Errorf("error during analysis with %s: %w", p.Info().Name, err)
	}

	return render.ModeInfo{Kind: render.ModeRef, Range: ref}, cleanup, nil
}

// archiveInRepo archives ref from the repository at repoRoot. gitx.Archive
// resolves the ref in the process working directory, so this runs from the
// repository root and restores the cwd afterwards.
func archiveInRepo(repoRoot, ref string) (dir string, cleanup func(), err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}
	if err := os.Chdir(repoRoot); err != nil {
		return "", nil, err
	}
	defer func() { _ = os.Chdir(cwd) }()

	return gitx.Archive(ref)
}

// analyzeRef archives ref into a temporary directory and analyzes it with p.
// The result paths are reanchored onto repoRoot so they join with
// repo-relative paths. The returned cleanup removes the archive.
func analyzeRef(p provider.Provider, repoRoot, ref string) (provider.Result, func(), error) {
	dir, cleanup, err := archiveInRepo(repoRoot, ref)
	if err != nil {
		return provider.Result{}, nil, err
	}

	result, err := p.Analyze(dir)
	if err != nil {
		cleanup()
		return provider.Result{}, nil, err
	}
	return reanchorResult(result, dir, repoRoot), cleanup, nil
}

// splitRange splits a git diff range into its S1 (base) and S2 (target) refs.
// A two-dot/three-dot range diffs ref A against ref B; a single revision
// diffs the revision against the working tree, reported as worktree=true.
func splitRange(rangeSpec string) (s1, s2 string, worktree bool) {
	for _, sep := range []string{"...", ".."} {
		if idx := strings.Index(rangeSpec, sep); idx >= 0 {
			s1 = rangeSpec[:idx]
			s2 = rangeSpec[idx+len(sep):]
			if s1 == "" {
				s1 = "HEAD"
			}
			if s2 == "" {
				s2 = "HEAD"
			}
			return s1, s2, false
		}
	}
	return rangeSpec, "", true
}

// reanchorResult rewrites result paths from srcRoot onto dstRoot so a result
// produced by analyzing a temporary archive joins with repo-relative paths.
func reanchorResult(result provider.Result, srcRoot, dstRoot string) provider.Result {
	for i, f := range result.Files {
		rel := f.Path
		if filepath.IsAbs(f.Path) {
			if r, err := filepath.Rel(srcRoot, f.Path); err == nil {
				rel = r
			}
		}
		result.Files[i].Path = filepath.Join(dstRoot, rel)
	}
	return result
}

func printError(errMsg string) {
	if _, err := os.Stdout.WriteString(errMsg + "\n"); err != nil {
		// If printing the error message itself fails, there's nothing we can do
		return
	}
}

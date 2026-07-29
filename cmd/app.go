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
	diffRange    string
	compareRange string
	refName      string

	appCmd = newAppCmd()
)

// newAppCmd builds the root command. It is a factory (rather than init-time
// registration) so tests can construct fresh instances with clean flag state;
// the flags are still bound to the package-level variables above.
func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokui [directory]",
		Short: "An interactive terminal tool for visualizing code statistics.",
		Long: `
📊 A terminal-based user interface for visualizing and analyzing directory code statistics.

Usage:
  1. Pipe mode: tokei -o json [directory] | tokui
  2. Direct mode: tokui [directory] (requires tokei to be installed on the system)

Pipe mode (recommended):
  tokei -o json . | tokui
  tokei -o json /path/to/project | tokui

Direct mode:
  tokui .
  tokui /path/to/project

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
	cmd.PersistentFlags().StringVar(
		&diffRange,
		"diff",
		"",
		`Show churn for a git range, e.g. "main...HEAD" or "HEAD~3". Bare --diff means worktree changes (like "git diff").`,
	)
	// 裸 --diff（不带值）等价于 --diff HEAD，即工作区未提交的改动。
	cmd.PersistentFlags().Lookup("diff").NoOptDefVal = "HEAD"
	cmd.PersistentFlags().StringVar(
		&compareRange,
		"compare",
		"",
		`Compare statistics between two git refs, e.g. "v1.0..v2.0".`,
	)
	cmd.PersistentFlags().StringVar(
		&refName,
		"ref",
		"",
		`Show statistics for a single git ref snapshot, e.g. "v1.0".`,
	)
	cmd.MarkFlagsMutuallyExclusive("diff", "compare", "ref")
	cmd.MarkFlagsMutuallyExclusive("tree", "treemap")
	return cmd
}

// Execute runs the root command. version is the version string to report via
// the --version/-v flag; it is resolved against Go build info when it is empty
// or the "dev" placeholder.
func Execute(version string) {
	appCmd.Version = resolveVersion(version)
	appCmd.SetArgs(preprocessDiffArgs(os.Args[1:]))
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

// preprocessDiffArgs rewrites a bare "--diff" followed by a separate value
// token into the "--diff=<value>" form. The diff flag declares NoOptDefVal
// ("HEAD") so that a bare --diff means worktree changes, but pflag then never
// consumes a separate value token: "tokui --diff HEAD~3" would treat HEAD~3
// as a positional directory argument. Rewriting before parsing restores the
// usual "--flag value" spelling while keeping the bare form (and everything
// after a "--" terminator) intact.
func preprocessDiffArgs(args []string) []string {
	out := make([]string, 0, len(args))
	positionalOnly := false
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if positionalOnly {
			out = append(out, tok)
			continue
		}
		if tok == "--" {
			positionalOnly = true
			out = append(out, tok)
			continue
		}
		if tok == "--diff" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			out = append(out, "--diff="+args[i+1])
			i++
			continue
		}
		out = append(out, tok)
	}
	return out
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
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("unknown panic: %v", r)
			}
			printError(render.ReportError(err, debug.Stack()))
		}
	}()

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

	if gitFlag := activeGitFlag(); gitFlag != "" && isPipe {
		return NewCLIError(fmt.Errorf("--%s cannot be combined with pipe mode (stdin); run 'tokui --%s' in a terminal inside a git repository", gitFlag, gitFlag))
	}

	// A positional argument overrides --root in every non-pipe mode.
	if len(args) > 0 {
		root = args[0]
	}
	analysisPath := filepath.Clean(root)

	tree := structure.NewTree(nil)
	modeInfo := render.ModeInfo{Kind: render.ModeFull}

	switch {
	case diffRange != "":
		// Diff mode: git supplies the change set and the snapshot.
		modeInfo, err = runDiffMode(tree, p, analysisPath, diffRange)
		if err != nil {
			return err
		}
	case compareRange != "":
		// Compare mode: two snapshots analyzed and joined by path.
		modeInfo, err = runCompareMode(tree, p, analysisPath, compareRange)
		if err != nil {
			return err
		}
	case refName != "":
		// Ref mode: a single historical snapshot in the familiar full view.
		var cleanup func()
		modeInfo, cleanup, err = runRefMode(tree, p, analysisPath, refName)
		if err != nil {
			return err
		}
		// The archive must outlive the TUI session: file previews read it.
		defer cleanup()
	case isPipe:
		// If there is pipe input, use pipe mode
		if err := runPipeMode(tree, p, selectedProvider); err != nil {
			return fmt.Errorf("error reading provider output from pipe: %w", err)
		}
	default:
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

// runDiffMode builds the tree for --diff: it locates the repository, collects
// the churn of the range with git, analyzes the S2 snapshot with the current
// provider and joins both into the tree. When path points into a subdirectory
// of the repository, the change set is scoped to that subdirectory (the S2
// analysis still covers the whole repository). It returns the ModeInfo the
// view layer needs for the status bar and S1/S2 previews.
func runDiffMode(tree *structure.Tree, p provider.Provider, path, diffRange string) (render.ModeInfo, error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\nHint: remove --diff to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, NewCLIError(err)
	}

	changes, err := gitx.Numstat(repoRoot, diffRange)
	if err != nil {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\n\nExamples:\n  tokui --diff main...HEAD\n  tokui --diff HEAD~3\n  tokui --diff HEAD", err))
	}
	changes = scopeChangesToSubdir(changes, repoRoot, path)

	s1Ref, s2Ref, worktree := splitRange(diffRange)
	s1Label := s1Ref
	if strings.Contains(diffRange, "...") {
		// "git diff A...B" diffs the merge-base of A and B against B; resolve
		// the merge-base so S1 previews ("git show S1:path") match the diff
		// base. Fall back to A when no common ancestor exists.
		if base, mbErr := gitx.MergeBase(repoRoot, s1Ref, s2Ref); mbErr == nil {
			s1Ref = base
		}
	}

	var s2 provider.Result
	if worktree {
		// The diff target is the working tree: analyze it in place.
		s2, err = p.Analyze(repoRoot)
	} else {
		// Historical S2: unpack the ref into a temporary directory. The
		// archive is only needed for the analysis itself (S2 previews go
		// through "git show"), so it is cleaned up when this function returns.
		var cleanup func()
		s2, cleanup, err = analyzeRef(p, repoRoot, s2Ref)
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

	s2Label := s2Ref
	if worktree {
		s2Label = "worktree"
	}
	return render.ModeInfo{
		Kind:     render.ModeDiff,
		Range:    diffRange,
		RepoRoot: repoRoot,
		S1Ref:    s1Ref,
		S1Label:  s1Label,
		S2Ref:    s2Ref,
		S2Label:  s2Label,
	}, nil
}

// scopeChangesToSubdir narrows a repo-wide change set to the entries under
// path when path points into a subdirectory of the repository ("tokui --diff
// HEAD~3 some/subdir"). A path at (or outside) the repository root leaves the
// change set untouched.
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

// runCompareMode builds the tree for --compare: both refs are analyzed with
// the current provider (archived to temporary directories, except a HEAD/worktree
// S2 which is analyzed in place) and joined by path via BuildFromCompare.
func runCompareMode(tree *structure.Tree, p provider.Provider, path, compareRange string) (render.ModeInfo, error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\nHint: remove --compare to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, NewCLIError(err)
	}

	s1Ref, s2Ref, worktree := splitRange(compareRange)
	if worktree {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("--compare requires two refs, e.g. \"v1.0..v2.0\" or \"main...HEAD\""))
	}

	s1, cleanup1, err := analyzeRef(p, repoRoot, s1Ref)
	if cleanup1 != nil {
		defer cleanup1()
	}
	if err != nil {
		return render.ModeInfo{}, NewCLIError(fmt.Errorf("%w\n\nExample:\n  tokui --compare v1.0..v2.0", err))
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

// runRefMode builds the tree for --ref: the ref's snapshot is unpacked into a
// temporary directory and analyzed with the familiar full view. The cleanup
// is returned to the caller because file previews read the archive for the
// whole TUI session.
func runRefMode(tree *structure.Tree, p provider.Provider, path, ref string) (render.ModeInfo, func(), error) {
	repoRoot, err := gitx.RepoRoot(path)
	if err != nil {
		if errors.Is(err, gitx.ErrNotGitRepo) {
			return render.ModeInfo{}, nil, NewCLIError(fmt.Errorf("%w\nHint: remove --ref to analyze the directory in full mode", err))
		}
		// e.g. the git binary is not installed
		return render.ModeInfo{}, nil, NewCLIError(err)
	}

	dir, cleanup, err := archiveInRepo(repoRoot, ref)
	if err != nil {
		return render.ModeInfo{}, nil, NewCLIError(fmt.Errorf("%w\n\nExample:\n  tokui --ref v1.0", err))
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

// activeGitFlag returns the name of the selected git mode flag, or "" when
// none is set.
func activeGitFlag() string {
	switch {
	case diffRange != "":
		return "diff"
	case compareRange != "":
		return "compare"
	case refName != "":
		return "ref"
	}
	return ""
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

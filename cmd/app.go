package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/provider/scc"
	"github.com/zdyxry/tokui/render"
	"github.com/zdyxry/tokui/structure"
	"github.com/zdyxry/tokui/tokei"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	ErrUnknown  = errors.New("unknown error")
	root        string
	treeMode    bool
	treemapMode bool
	providerName string

	appCmd = &cobra.Command{
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
		RunE: runApp,
	}
)

func init() {
	appCmd.PersistentFlags().StringVarP(
		&root,
		"root",
		"r",
		".",
		`Specify the root directory to analyze. Defaults to current directory.`,
	)
	appCmd.PersistentFlags().BoolVarP(
		&treeMode,
		"tree",
		"t",
		false,
		`Start in tree mode (expandable directories instead of navigation).`,
	)
	appCmd.PersistentFlags().BoolVar(
		&treemapMode,
		"treemap",
		false,
		`Start in treemap mode (proportional blocks instead of a table).`,
	)
	appCmd.PersistentFlags().StringVar(
		&providerName,
		"provider",
		"tokei",
		`Stats provider: tokei|scc. Defaults to tokei for backward compatibility.`,
	)
	appCmd.MarkFlagsMutuallyExclusive("tree", "treemap")
}

func Execute() {
	if err := appCmd.Execute(); err != nil {
		var cliErr *CLIError
		if errors.As(err, &cliErr) {
			printError(cliErr.Error())
		} else {
			printError(render.ReportError(err, debug.Stack()))
		}
		os.Exit(1)
	}
}

func runApp(_ *cobra.Command, args []string) error {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("unknown panic: %v", r)
			}
			printError(render.ReportError(err, debug.Stack()))
		}
	}()

	p, err := selectProvider(providerName)
	if err != nil {
		return err
	}

	// Check if there is stdin input (pipe mode)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to check standard input: %w", err)
	}

	tree := structure.NewTree(nil)

	// If there is pipe input, use pipe mode
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		if err := runPipeMode(tree, p); err != nil {
			return fmt.Errorf("error reading provider output from pipe: %w", err)
		}
	} else {
		// Direct mode: need to specify directory
		if len(args) > 0 {
			root = args[0]
		}
		analysisPath := filepath.Clean(root)

		if err := tree.BuildFromProvider(p, analysisPath); err != nil {
			// Provide a more friendly error message if the provider binary is not installed
			if strings.Contains(err.Error(), "executable file not found") {
				errMsg := fmt.Sprintf("Command '%s' not found. Please install it and ensure it's in your system PATH environment variable.\n"+
					"Or use pipe mode: tokei -o json . | tokui", p.Info().Name)
				printError(errMsg)
				os.Exit(1)
			}
			return fmt.Errorf("error during analysis with %s: %w", p.Info().Name, err)
		}
	}

	// Initialize view model
	vm, err := initViewModel(tree, p.Info(), treeMode, treemapMode)
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

// runPipeMode reads stdin once and either uses the selected provider or
// attempts to auto-detect the format.
func runPipeMode(tree *structure.Tree, p provider.Provider) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	result, _, err := parseStdinWithProvider(p, data)
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
func parseStdinWithProvider(p provider.Provider, data []byte) (provider.Result, provider.Provider, error) {
	result, err := p.ParseStdin(data)
	if err == nil {
		return result, p, nil
	}

	// If the user explicitly selected a non-default provider, do not auto-detect.
	if providerName != "tokei" {
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

func initViewModel(tree *structure.Tree, info provider.Info, treeMode, treemapMode bool) (*render.ViewModel, error) {
	nav := render.NewCodeNavigation(tree)
	dirModel := render.NewDirModel(nav, info, treeMode, treemapMode)
	vm := render.NewViewModel(
		nav,
		dirModel,
	)
	vm.Update(render.ScanFinished{})
	return vm, nil
}

func printError(errMsg string) {
	if _, err := os.Stdout.WriteString(errMsg + "\n"); err != nil {
		// If printing the error message itself fails, there's nothing we can do
		return
	}
}

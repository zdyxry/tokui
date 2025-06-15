package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/zdyxry/tokui/render"
	"github.com/zdyxry/tokui/structure"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	ErrUnknown = errors.New("unknown error")
	root       string

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

	// Check if there is stdin input (pipe mode)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to check standard input: %w", err)
	}

	// Build data tree using tokei's output
	tree := structure.NewTree(nil)

	// If there is pipe input, use pipe mode
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Pipe mode: read tokei's JSON output from stdin
		if err := tree.BuildFromStdin(); err != nil {
			return fmt.Errorf("error reading tokei output from pipe: %w", err)
		}
	} else {
		// Direct mode: need to specify directory
		if len(args) > 0 {
			root = args[0]
		}

		// Clean the path
		analysisPath := filepath.Clean(root)

		if err := tree.BuildFromTokei(analysisPath); err != nil {
			// Provide a more friendly error message if tokei is not installed
			if strings.Contains(err.Error(), "executable file not found") {
				errMsg := "Command 'tokei' not found. Please install it and ensure it's in your system PATH environment variable.\n" +
					"Reference: https://github.com/XAMPPRocky/tokei\n\n" +
					"Or use pipe mode: tokei -o json . | tokui"
				printError(errMsg)
				os.Exit(1)
			}
			// Other tokei errors
			return fmt.Errorf("error during analysis with tokei: %w", err)
		}
	}

	// Initialize view model
	vm, err := initViewModel(tree)
	if err != nil {
		return err
	}

	// Create and run the Bubble Tea program
	teaProg := tea.NewProgram(
		vm,
		tea.WithAltScreen(),
		tea.WithoutCatchPanics(),
	)

	render.SetTeaProgram(teaProg)

	if _, err = teaProg.Run(); err != nil {
		return err
	}

	return nil
}

func initViewModel(tree *structure.Tree) (*render.ViewModel, error) {
	nav := render.NewCodeNavigation(tree)
	dirModel := render.NewDirModel(nav)
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

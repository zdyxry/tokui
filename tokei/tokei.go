// tokei/tokei.go
package tokei

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/zdyxry/tokui/internal/binaries"
)

// LanguageReport maps language names to their statistics
type LanguageReport map[string]Stats

type Stats struct {
	Blanks   int64        `json:"blanks"`
	Code     int64        `json:"code"`
	Comments int64        `json:"comments"`
	Reports  []FileReport `json:"reports"`
}

type FileReport struct {
	Name  string     `json:"name"`
	Stats InnerStats `json:"stats"`
}

type InnerStats struct {
	Blanks   int64 `json:"blanks"`
	Code     int64 `json:"code"`
	Comments int64 `json:"comments"`
}

// Analyze runs tokei on the given path and parses its JSON output
func Analyze(path string) (LanguageReport, error) {
	tokeiPath, err := binaries.TokeiPath()
	if err != nil {
		return nil, fmt.Errorf("tokei binary not available: %w. Please install tokei (https://github.com/XAMPPRocky/tokei) or run 'make fetch-tokei-binaries'", err)
	}

	cmd := exec.Command(tokeiPath, "--output", "json", path)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf(
				"tokei command execution failed (exit code %d): %s\nStandard error output:\n%s",
				exitErr.ExitCode(),
				err,
				string(exitErr.Stderr),
			)
		}
		return nil, fmt.Errorf("failed to execute tokei (please ensure tokei is installed and in PATH environment variable): %w", err)
	}

	return parseReport(output)
}

// AnalyzeFromStdin parses tokei JSON from stdin
func AnalyzeFromStdin() (LanguageReport, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read standard input: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("standard input is empty, please ensure tokei's JSON output is provided through a pipe")
	}

	return parseReport(data)
}

// GetVersion returns the version of the available tokei binary.
func GetVersion() (string, error) {
	tokeiPath, err := binaries.TokeiPath()
	if err != nil {
		return "", err
	}

	output, err := exec.Command(tokeiPath, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get tokei version: %w", err)
	}

	fields := strings.Fields(string(output))
	if len(fields) >= 2 {
		return fields[1], nil
	}
	return "", fmt.Errorf("unexpected tokei version output: %s", string(output))
}

// parseReport handles both direct and nested tokei JSON formats
func parseReport(data []byte) (LanguageReport, error) {
	var report LanguageReport
	// Try to parse directly. If tokei's output is nested (e.g., {"dirname": {"Go": ...}}),
	// this will fail, then we handle the nested case.
	if err := json.Unmarshal(data, &report); err == nil {
		return report, nil // Success, it's a direct LanguageReport
	}

	// If direct parsing fails, try to parse as nested map[string]LanguageReport
	var nestedReport map[string]LanguageReport
	if err := json.Unmarshal(data, &nestedReport); err == nil {
		// Extract the first (and only) inner report
		for _, r := range nestedReport {
			return r, nil
		}
	}

	return nil, fmt.Errorf("failed to parse tokei JSON output, unrecognized format")
}

// Package tokei provides a Provider implementation that shells out to the
// external tokei binary and parses its JSON output.
package tokei

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/zdyxry/tokui/internal/binaries"
	"github.com/zdyxry/tokui/provider"
)

// LanguageReport maps language names to their statistics.
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

// TokeiProvider shells out to the tokei binary.
type TokeiProvider struct {
	mu       sync.Mutex
	version  string
	resolved bool
}

// New creates a new tokei Provider.
func New() *TokeiProvider {
	return &TokeiProvider{}
}

// Info returns metadata for the tokei Provider. The version is resolved lazily
// on first call by running "tokei --version".
func (p *TokeiProvider) Info() provider.Info {
	return provider.Info{
		Name:         "tokei",
		Version:      p.resolveVersion(),
		Capabilities: provider.CapLines,
	}
}

// resolveVersion resolves the tokei binary version lazily.
func (p *TokeiProvider) resolveVersion() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.resolved {
		return p.version
	}
	p.resolved = true

	v, err := GetVersion()
	if err != nil {
		p.version = "unknown"
	} else {
		p.version = v
	}
	return p.version
}

// Analyze runs tokei on the given path and parses its JSON output.
func (p *TokeiProvider) Analyze(path string) (provider.Result, error) {
	tokeiPath, err := binaries.TokeiPath()
	if err != nil {
		return provider.Result{}, fmt.Errorf("tokei binary not available: %w. Please install tokei (https://github.com/XAMPPRocky/tokei) or run 'make fetch-tokei-binaries'", err)
	}

	cmd := exec.Command(tokeiPath, "--output", "json", path)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return provider.Result{}, fmt.Errorf(
				"tokei command execution failed (exit code %d): %s\nStandard error output:\n%s",
				exitErr.ExitCode(),
				err,
				string(exitErr.Stderr),
			)
		}
		return provider.Result{}, fmt.Errorf("failed to execute tokei (please ensure tokei is installed and in PATH environment variable): %w", err)
	}

	report, err := parseReport(output)
	if err != nil {
		return provider.Result{}, err
	}
	return toProviderResult(report), nil
}

// ParseStdin parses tokei JSON from the supplied byte slice.
func (p *TokeiProvider) ParseStdin(data []byte) (provider.Result, error) {
	if len(data) == 0 {
		return provider.Result{}, fmt.Errorf("standard input is empty, please ensure tokei's JSON output is provided through a pipe")
	}

	report, err := parseReport(data)
	if err != nil {
		return provider.Result{}, err
	}
	return toProviderResult(report), nil
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

	return parseVersionOutput(output)
}

// parseVersionOutput extracts the version string from tokei --version output.
func parseVersionOutput(output []byte) (string, error) {
	fields := strings.Fields(string(output))
	if len(fields) >= 2 {
		return fields[1], nil
	}
	return "", fmt.Errorf("unexpected tokei version output: %s", string(output))
}

// parseReport handles both direct and nested tokei JSON formats.
//
// Direct format:  {"Go": {"blanks": ..., "code": ..., "comments": ..., "reports": [...]}}
// Nested format: {"dirname": {"Go": {"blanks": ..., ...}}}
func parseReport(data []byte) (LanguageReport, error) {
	var report LanguageReport
	// Because encoding/json ignores unknown fields, a nested report can
	// unmarshal into LanguageReport with all-zero Stats, so we validate that
	// the result actually contains stats.
	if err := json.Unmarshal(data, &report); err == nil && looksLikeLanguageReport(report) {
		return report, nil
	}

	// If direct parsing fails or yields empty stats, try the nested format.
	var nestedReport map[string]LanguageReport
	if err := json.Unmarshal(data, &nestedReport); err == nil {
		// Extract the first (and only) inner report
		for _, r := range nestedReport {
			return r, nil
		}
	}

	return nil, fmt.Errorf("failed to parse tokei JSON output, unrecognized format")
}

// looksLikeLanguageReport returns true when the report contains at least one
// language with non-zero statistics or file reports. This distinguishes a real
// direct report from a nested report that happened to unmarshal with all-zero
// values because unknown fields were ignored.
func looksLikeLanguageReport(report LanguageReport) bool {
	for _, stats := range report {
		if stats.Code > 0 || stats.Comments > 0 || stats.Blanks > 0 || len(stats.Reports) > 0 {
			return true
		}
	}
	return false
}

// toProviderResult converts a tokei LanguageReport into the provider.Result
// shape expected by the rest of the application.
func toProviderResult(report LanguageReport) provider.Result {
	result := provider.Result{}
	for lang, stats := range report {
		if lang == "Total" {
			continue
		}
		for _, fr := range stats.Reports {
			result.Files = append(result.Files, provider.FileStats{
				Path:     fr.Name,
				Language: lang,
				Code:     fr.Stats.Code,
				Comments: fr.Stats.Comments,
				Blanks:   fr.Stats.Blanks,
			})
		}
	}
	return result
}

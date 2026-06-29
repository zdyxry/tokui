// Package scc provides a Provider implementation backed by the scc counting
// engine (github.com/boyter/scc/v3/processor).
package scc

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/boyter/scc/v3/processor"
	"github.com/zdyxry/tokui/provider"
)

// SCCProvider uses scc's processor package to count lines and estimate
// complexity.
type SCCProvider struct {
	initOnce sync.Once
}

// New creates a new scc Provider.
func New() *SCCProvider {
	return &SCCProvider{}
}

// Info returns metadata and capabilities for the scc Provider.
func (p *SCCProvider) Info() provider.Info {
	return provider.Info{
		Name:         "scc",
		Version:      processor.Version,
		Capabilities: provider.CapLines | provider.CapComplexity | provider.CapBytes,
	}
}

// init ensures scc's language constants are loaded exactly once.
func (p *SCCProvider) init() {
	p.initOnce.Do(func() {
		processor.ProcessConstants()
	})
}

// Analyze walks the directory or file at path and returns per-file statistics.
func (p *SCCProvider) Analyze(path string) (provider.Result, error) {
	p.init()

	info, err := os.Stat(path)
	if err != nil {
		return provider.Result{}, fmt.Errorf("failed to stat path: %w", err)
	}

	result := provider.Result{}
	if !info.IsDir() {
		f, err := p.countFile(path)
		if err != nil {
			return provider.Result{}, err
		}
		result.Files = append(result.Files, f)
		return result, nil
	}

	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		f, err := p.countFile(filePath)
		if err != nil {
			return nil // best-effort: skip files we cannot process
		}
		result.Files = append(result.Files, f)
		return nil
	})

	return result, err
}

// ParseStdin parses scc JSON output (the format produced by
// `scc --format json`) from the supplied byte slice.
func (p *SCCProvider) ParseStdin(data []byte) (provider.Result, error) {
	if len(data) == 0 {
		return provider.Result{}, fmt.Errorf("standard input is empty")
	}

	var summaries []processor.LanguageSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		return provider.Result{}, fmt.Errorf("failed to parse scc JSON output: %w", err)
	}

	result := provider.Result{}
	for _, summary := range summaries {
		if summary.Name == "Total" {
			continue
		}
		for _, job := range summary.Files {
			result.Files = append(result.Files, provider.FileStats{
				Path:       job.Location,
				Language:   summary.Name,
				Code:       job.Code,
				Comments:   job.Comment,
				Blanks:     job.Blank,
				Complexity: job.Complexity,
				Bytes:      job.Bytes,
			})
		}
	}

	return result, nil
}

// countFile reads a single file, detects its language, and runs scc's
// CountStats.
func (p *SCCProvider) countFile(filePath string) (provider.FileStats, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return provider.FileStats{}, err
	}

	filename := filepath.Base(filePath)
	possibleLangs, lang := processor.DetectLanguage(filename)
	if lang == "" {
		return provider.FileStats{}, fmt.Errorf("unable to detect language for %s", filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return provider.FileStats{}, err
	}

	lang = processor.DetermineLanguage(filename, lang, possibleLangs, content)
	if lang == "" {
		return provider.FileStats{}, fmt.Errorf("unable to determine language for %s", filePath)
	}

	job := &processor.FileJob{
		Filename: filename,
		Location: filePath,
		Language: lang,
		Content:  content,
		Bytes:    info.Size(),
	}
	processor.CountStats(job)

	return provider.FileStats{
		Path:       filePath,
		Language:   lang,
		Code:       job.Code,
		Comments:   job.Comment,
		Blanks:     job.Blank,
		Complexity: job.Complexity,
		Bytes:      job.Bytes,
	}, nil
}

// shouldSkipDir reports whether a directory should be skipped during traversal.
func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

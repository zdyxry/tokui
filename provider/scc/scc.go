// Package scc provides a Provider implementation backed by the scc counting
// engine (github.com/boyter/scc/v3/processor).
package scc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/boyter/gocodewalker"
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
		Capabilities: provider.CapLines | provider.CapComplexity,
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

	files, err := p.walkDirectory(path)
	if err != nil {
		return provider.Result{}, err
	}
	result.Files = files
	return result, nil
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
	}, nil
}

// walkDirectory walks the directory tree using gocodewalker, which respects
// .gitignore, .ignore and .gitmodules files by default. It returns per-file
// stats for every file the walker yields.
func (p *SCCProvider) walkDirectory(path string) ([]provider.FileStats, error) {
	result := make([]provider.FileStats, 0)
	queue := make(chan *gocodewalker.File, 128)

	walker := gocodewalker.NewFileWalker(path, queue)
	// Always skip common VCS and dependency directories, matching the previous
	// filepath.WalkDir behaviour.
	walker.ExcludeDirectory = []string{".git", ".hg", ".svn", "node_modules", "vendor"}

	var wg sync.WaitGroup
	wg.Add(1)
	var walkErr error
	go func() {
		defer wg.Done()
		walkErr = walker.Start()
	}()

	for f := range queue {
		stats, err := p.countFile(f.Location)
		if err != nil {
			continue // best-effort: skip files we cannot process
		}
		result = append(result, stats)
	}

	wg.Wait()
	if walkErr != nil {
		return nil, walkErr
	}
	return result, nil
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

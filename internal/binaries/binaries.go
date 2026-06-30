package binaries

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	once     sync.Once
	tokeiPath string
	tokeiErr  error
)

// TokeiPath returns a usable tokei binary path.
// Priority:
//  1. TOKEI_PATH environment variable
//  2. tokei found in $PATH
//  3. Embedded tokei binary (extracted to temp directory)
func TokeiPath() (string, error) {
	once.Do(func() {
		tokeiPath, tokeiErr = findTokei()
	})
	return tokeiPath, tokeiErr
}

func findTokei() (string, error) {
	// 1. Environment variable override
	if envPath := os.Getenv("TOKEI_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// 2. System PATH
	if path, err := exec.LookPath("tokei"); err == nil {
		return path, nil
	}

	// 3. Embedded fallback
	return extractEmbeddedTokei()
}

// ClearCache removes the extracted tokei binary cache directory.
func ClearCache() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return os.RemoveAll(filepath.Join(cacheDir, "tokui"))
}

func extractEmbeddedTokei() (string, error) {
	if len(embeddedTokeiGz) == 0 {
		return "", fmt.Errorf(
			"tokei not found in PATH and no embedded binary available for %s/%s",
			runtime.GOOS, runtime.GOARCH,
		)
	}

	// Use UserCacheDir for persistent caching across restarts.
	// Falls back to TempDir if UserCacheDir is unavailable.
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	tokeiDir := filepath.Join(cacheDir, "tokui")
	if err := os.MkdirAll(tokeiDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	binaryName := "tokei"
	if runtime.GOOS == "windows" {
		binaryName = "tokei.exe"
	}
	binaryPath := filepath.Join(tokeiDir, binaryName)

	// If already extracted and looks valid, reuse it.
	if info, err := os.Stat(binaryPath); err == nil && info.Size() > 100*1024 {
		return binaryPath, nil
	}

	// Decompress gzip
	r, err := gzip.NewReader(bytes.NewReader(embeddedTokeiGz))
	if err != nil {
		return "", fmt.Errorf("failed to decompress embedded tokei: %w", err)
	}
	defer func() { _ = r.Close() }()

	// Write to a temporary file and atomically rename it to the target path.
	// This avoids "text file busy" errors when multiple concurrent processes
	// (e.g. parallel `go test` packages) extract the embedded binary to the
	// same cache location at the same time.
	tmpFile, err := os.CreateTemp(tokeiDir, binaryName+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp cache file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := func() { _ = os.Remove(tmpPath) }

	written, err := tmpFile.ReadFrom(r)
	if err != nil {
		_ = tmpFile.Close()
		cleanupTmp()
		return "", fmt.Errorf("failed to write embedded tokei: %w", err)
	}

	// Sanity check: real tokei binary should be at least 100KB.
	// Placeholder files are much smaller (~30 bytes).
	if written < 100*1024 {
		_ = tmpFile.Close()
		cleanupTmp()
		return "", fmt.Errorf(
			"embedded tokei binary is a placeholder; run 'make fetch-tokei-binaries' to download real binaries",
		)
	}

	if err := tmpFile.Close(); err != nil {
		cleanupTmp()
		return "", fmt.Errorf("failed to close temp cache file: %w", err)
	}

	if err := os.Rename(tmpPath, binaryPath); err != nil {
		cleanupTmp()
		// Another process (e.g. a parallel `go test` package) may have already
		// installed a valid binary at binaryPath. On Windows, renaming over an
		// existing/locked file fails with "Access is denied" rather than
		// replacing it atomically. If a valid binary is already present, use it.
		if info, statErr := os.Stat(binaryPath); statErr == nil && info.Size() > 100*1024 {
			return binaryPath, nil
		}
		return "", fmt.Errorf("failed to install cached tokei binary: %w", err)
	}

	// Ensure the final binary is executable (CreateTemp uses 0600 by default).
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make cached tokei binary executable: %w", err)
	}

	return binaryPath, nil
}

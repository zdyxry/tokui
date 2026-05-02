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

	f, err := os.OpenFile(binaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create cache file: %w", err)
	}
	defer func() { _ = f.Close() }()

	written, err := f.ReadFrom(r)
	if err != nil {
		_ = os.Remove(binaryPath)
		return "", fmt.Errorf("failed to write embedded tokei: %w", err)
	}

	// Sanity check: real tokei binary should be at least 100KB.
	// Placeholder files are much smaller (~30 bytes).
	if written < 100*1024 {
		_ = os.Remove(binaryPath)
		return "", fmt.Errorf(
			"embedded tokei binary is a placeholder; run 'make fetch-tokei-binaries' to download real binaries",
		)
	}

	return binaryPath, nil
}

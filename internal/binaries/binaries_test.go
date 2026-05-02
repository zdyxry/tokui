package binaries

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

func TestTokeiPath_SystemPath(t *testing.T) {
	// Ensure system tokei is found when available
	path, err := TokeiPath()
	if err != nil {
		t.Fatalf("TokeiPath failed: %v", err)
	}

	cmd := exec.Command(path, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tokei --version failed: %v", err)
	}
	if !strings.Contains(string(out), "tokei") {
		t.Fatalf("unexpected output: %s", string(out))
	}
	t.Logf("Found tokei at %s", path)
}

func TestTokeiPath_PlaceholderFallback(t *testing.T) {
	// If a real binary is embedded for this platform, skip this test.
	if len(embeddedTokeiGz) > 100*1024 {
		t.Skip("Real tokei binary is embedded for this platform, skipping placeholder test")
	}

	// Clean up any previously extracted binary from cache or temp
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "/tmp"
	}
	os.RemoveAll(cacheDir + "/tokui")
	os.RemoveAll("/tmp/tokui-tokei") // legacy cleanup

	// Reset cache
	once = sync.Once{}
	tokeiPath = ""
	tokeiErr = nil

	// Save original env
	origPath := os.Getenv("PATH")
	origTokeiPath := os.Getenv("TOKEI_PATH")
	defer func() {
		os.Setenv("PATH", origPath)
		os.Setenv("TOKEI_PATH", origTokeiPath)
	}()

	// Clear PATH and TOKEI_PATH to force embedded fallback
	os.Setenv("PATH", "/nonexistent")
	os.Unsetenv("TOKEI_PATH")

	path, err := TokeiPath()
	if err == nil {
		t.Fatalf("Expected error for placeholder, got path: %s", path)
	}

	if !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("Expected placeholder error, got: %v", err)
	}
	t.Logf("Correctly got placeholder error: %v", err)
}

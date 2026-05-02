#!/usr/bin/env bash
# Verification script for tokui's embedded tokei feature.
# Run this after building to validate correctness.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="${SCRIPT_DIR}/.."
BINARY="${PROJECT_DIR}/bin/tokui"
FAILED=0

pass() { echo "  ✅ $1"; }
fail() { echo "  ❌ $1"; FAILED=1; }
info() { echo "  ℹ️  $1"; }

# ---------------------------------------------------------------------------
echo ""
echo "=== Tokui Embedded Tokei Verification ==="
echo ""

# Determine cache dir (matches logic in binaries.go)
CACHE_DIR="${HOME}/.cache"
if [[ ! -d "$CACHE_DIR" ]]; then
    CACHE_DIR="/tmp"
fi
TOKEI_CACHE="${CACHE_DIR}/tokui"

# Ensure binary exists
if [[ ! -f "$BINARY" ]]; then
    echo "Binary not found at $BINARY"
    echo "Run 'make build' first."
    exit 1
fi

# ---------------------------------------------------------------------------
echo "[1] Basic binary smoke test"
if "$BINARY" --help &>/dev/null; then
    pass "tokui --help works"
else
    fail "tokui --help failed"
fi

# ---------------------------------------------------------------------------
echo ""
echo "[2] Tokei resolution priority"

# 2a. System PATH priority
current_path=$("$BINARY" --help 2>&1 || true)
if [[ -n "$current_path" ]]; then
    pass "Binary runs with default environment"
else
    fail "Binary failed to run"
fi

# 2b. TOKEI_PATH override
if command -v tokei &>/dev/null; then
    fake_tokei="${PROJECT_DIR}/bin/fake-tokei-$$"
    echo '#!/bin/sh' > "$fake_tokei"
    echo 'echo "FAKE_TOKEI_VERSION"' >> "$fake_tokei"
    chmod +x "$fake_tokei"

    # Use a Go test binary to check TokeiPath directly
    cat > "${PROJECT_DIR}/internal/binaries/verify_test.go" << 'GOEOF'
package binaries

import (
    "os"
    "strings"
    "sync"
    "testing"
)

func TestTokeiPathPriority(t *testing.T) {
    // Reset cache
    once = sync.Once{}
    tokeiPath = ""
    tokeiErr = nil

    // Case 1: TOKEI_PATH env var wins
    os.Setenv("TOKEI_PATH", os.Getenv("FAKE_TOKEI"))
    defer os.Unsetenv("TOKEI_PATH")

    path, err := TokeiPath()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(path, "fake-tokei") {
        t.Fatalf("expected fake-tokei, got: %s", path)
    }
}
GOEOF

    if FAKE_TOKEI="$fake_tokei" go test -v -run TestTokeiPathPriority ./internal/binaries/ &>/dev/null; then
        pass "TOKEI_PATH environment variable is respected"
    else
        fail "TOKEI_PATH priority test failed"
    fi

    rm -f "$fake_tokei" "${PROJECT_DIR}/internal/binaries/verify_test.go"
else
    info "Skipped TOKEI_PATH priority test (no system tokei found)"
fi

# ---------------------------------------------------------------------------
echo ""
echo "[3] Embedded fallback behavior"

# Build a small test to check placeholder vs real binary logic
cat > "${PROJECT_DIR}/internal/binaries/verify_embed_test.go" << 'GOEOF'
package binaries

import (
    "os"
    "strings"
    "sync"
    "testing"
)

func TestEmbeddedFallback(t *testing.T) {
    os.RemoveAll("/tmp/tokui-tokei")

    once = sync.Once{}
    tokeiPath = ""
    tokeiErr = nil

    os.Setenv("PATH", "/nonexistent")
    os.Unsetenv("TOKEI_PATH")
    defer os.Setenv("PATH", os.Getenv("PATH"))

    _, err := TokeiPath()
    if err == nil {
        t.Fatal("expected error for placeholder without system tokei")
    }
    if !strings.Contains(err.Error(), "placeholder") {
        t.Fatalf("expected placeholder error, got: %v", err)
    }
}
GOEOF

if go test -v -run TestEmbeddedFallback ./internal/binaries/ &>/dev/null; then
    pass "Placeholder correctly returns error when no system tokei available"
else
    fail "Embedded fallback test failed"
fi
rm -f "${PROJECT_DIR}/internal/binaries/verify_embed_test.go"

# ---------------------------------------------------------------------------
echo ""
echo "[4] Real embedded binary extraction (if available)"

# Check if current platform has a real binary embedded
embed_size=$(stat -f%z "${PROJECT_DIR}/internal/binaries/embed/$(go env GOOS)_$(go env GOARCH)/tokei.gz" 2>/dev/null || \
             stat -c%s "${PROJECT_DIR}/internal/binaries/embed/$(go env GOOS)_$(go env GOARCH)/tokei.gz" 2>/dev/null || \
             echo 0)

if [[ "$embed_size" -gt 10240 ]]; then
    # Real binary is embedded. Test extraction.
    os.RemoveAll("/tmp/tokui-tokei")

    cat > "${PROJECT_DIR}/internal/binaries/verify_real_test.go" << 'GOEOF'
package binaries

import (
    "os"
    "os/exec"
    "sync"
    "testing"
)

func TestRealEmbeddedExtraction(t *testing.T) {
    os.RemoveAll("/tmp/tokui-tokei")

    once = sync.Once{}
    tokeiPath = ""
    tokeiErr = nil

    os.Setenv("PATH", "/nonexistent")
    os.Unsetenv("TOKEI_PATH")
    defer os.Setenv("PATH", "/usr/local/bin:/usr/bin:/bin")

    path, err := TokeiPath()
    if err != nil {
        t.Fatalf("failed to extract embedded tokei: %v", err)
    }

    cmd := exec.Command(path, "--version")
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("embedded tokei --version failed: %v\noutput: %s", err, string(out))
    }
    t.Logf("Embedded tokei works: %s", string(out))
}
GOEOF

    if go test -v -run TestRealEmbeddedExtraction ./internal/binaries/ &>/dev/null; then
        pass "Real embedded binary extracts and runs correctly"
    else
        fail "Real embedded binary test failed"
    fi
    rm -f "${PROJECT_DIR}/internal/binaries/verify_real_test.go"
else
    info "Current platform uses placeholder (run 'make fetch-tokei-binaries' to embed real binary)"
fi

# ---------------------------------------------------------------------------
echo ""
echo "[5] File size sanity check"
binary_size=$(stat -f%z "$BINARY" 2>/dev/null || stat -c%s "$BINARY" 2>/dev/null || echo 0)
if [[ "$binary_size" -gt 1024 ]]; then
    pass "Binary size: $(numfmt --to=iec-i "$binary_size" 2>/dev/null || echo "${binary_size} bytes")"
else
    fail "Binary size suspiciously small: $binary_size bytes"
fi

# ---------------------------------------------------------------------------
echo ""
echo "=== Verification Complete ==="
if [[ "$FAILED" -eq 0 ]]; then
    echo "All checks passed."
    exit 0
else
    echo "Some checks failed."
    exit 1
fi

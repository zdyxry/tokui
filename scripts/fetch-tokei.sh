#!/usr/bin/env bash
set -eo pipefail

# Fetch tokei binaries for all supported platforms and embed them.
# Supports (in order of priority):
#   1. Reuse local system tokei if version matches
#   2. Reuse already-embedded binary if version matches (via version.txt)
#   3. Download from GitHub release
#   4. Build locally via cargo install (host platform only)

TOKEI_VERSION="${TOKEI_VERSION:-14.0.0}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EMBED_DIR="${SCRIPT_DIR}/../internal/binaries/embed"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Detect host platform for cargo fallback
HOST_TARGET=""
if command -v rustc &>/dev/null; then
    HOST_TARGET=$(rustc -vV | grep host | awk '{print $2}')
fi

# Cross-platform file size helper
file_size() {
    if stat -f%z "$1" 2>/dev/null; then
        return
    fi
    stat -c%s "$1" 2>/dev/null || echo 0
}

# Check if local system tokei matches target version
get_local_tokei_version() {
    if command -v tokei &>/dev/null; then
        tokei --version 2>/dev/null | awk '{print $2}'
    fi
}

LOCAL_TOKEI_VERSION=""
LOCAL_TOKEI_PATH=""
if command -v tokei &>/dev/null; then
    LOCAL_TOKEI_VERSION=$(get_local_tokei_version)
    LOCAL_TOKEI_PATH=$(command -v tokei)
fi

fetch_one() {
    local platform="$1"
    local target="$2"
    local dir="${EMBED_DIR}/${platform}"
    local output="${dir}/tokei.gz"
    local version_file="${dir}/version.txt"

    mkdir -p "$dir"

    # Priority 1: Already embedded with matching version
    if [[ -f "$version_file" && -f "$output" ]]; then
        local embedded_version
        embedded_version=$(cat "$version_file" 2>/dev/null | tr -d '[:space:]')
        if [[ "$embedded_version" == "$TOKEI_VERSION" && $(file_size "$output") -gt 10240 ]]; then
            echo "[$platform] Already embedded version $TOKEI_VERSION, skipping"
            return 0
        fi
    fi

    # Priority 2: Local system tokei with matching version (host platform only)
    if [[ -n "$LOCAL_TOKEI_VERSION" && "$LOCAL_TOKEI_VERSION" == "$TOKEI_VERSION" ]]; then
        # Determine if local tokei target matches this platform
        local local_target=""
        case "$platform" in
            linux_amd64)
                [[ "$HOST_TARGET" == "x86_64-unknown-linux"* ]] && local_target="match"
                ;;
            linux_arm64)
                [[ "$HOST_TARGET" == "aarch64-unknown-linux"* ]] && local_target="match"
                ;;
            darwin_amd64)
                [[ "$HOST_TARGET" == "x86_64-apple-darwin" ]] && local_target="match"
                ;;
            darwin_arm64)
                [[ "$HOST_TARGET" == "aarch64-apple-darwin" ]] && local_target="match"
                ;;
            windows_amd64)
                [[ "$HOST_TARGET" == "x86_64-pc-windows"* ]] && local_target="match"
                ;;
        esac

        if [[ "$local_target" == "match" ]]; then
            echo "[$platform] Reusing local tokei v${LOCAL_TOKEI_VERSION} (${LOCAL_TOKEI_PATH})"
            gzip -c "$LOCAL_TOKEI_PATH" > "$output"
            echo "$TOKEI_VERSION" > "$version_file"
            echo "[$platform] OK ($(file_size "$output") bytes, from local system)"
            return 0
        fi
    fi

    # Priority 3: Download from cargo-quickinstall (pre-built binaries)
    # Quickinstall provides reliable, fast pre-built Rust binaries.
    local qi_url="https://github.com/cargo-bins/cargo-quickinstall/releases/download/tokei-${TOKEI_VERSION}/tokei-${TOKEI_VERSION}-${target}.tar.gz"
    echo "[$platform] Downloading ${target} from cargo-quickinstall ..."

    local downloaded=false
    local tarball="${TMPDIR}/tokei-${platform}.tar.gz"

    if curl -fsL --connect-timeout 10 --max-time 60 -o "$tarball" "$qi_url" 2>/dev/null; then
        if tar -xzf "$tarball" -C "$TMPDIR" 2>/dev/null; then
            local binary_name="tokei"
            [[ "$platform" == windows_* ]] && binary_name="tokei.exe"

            local binary_path="${TMPDIR}/${binary_name}"
            if [[ -f "$binary_path" ]]; then
                gzip -c "$binary_path" > "$output"
                rm -f "$binary_path"
                echo "$TOKEI_VERSION" > "$version_file"
                echo "[$platform] OK ($(file_size "$output") bytes, from cargo-quickinstall)"
                downloaded=true
            else
                echo "[$platform] Binary not found in quickinstall tarball"
            fi
        else
            echo "[$platform] Failed to extract quickinstall tarball"
        fi
        rm -f "$tarball"
    else
        echo "[$platform] cargo-quickinstall download failed"
    fi

    # Priority 4: Fallback to tokei official GitHub release
    if [[ "$downloaded" == false ]]; then
        local url="https://github.com/XAMPPRocky/tokei/releases/download/v${TOKEI_VERSION}/tokei-${target}.tar.gz"
        echo "[$platform] Trying tokei official GitHub release ..."

        if curl -fsL --connect-timeout 10 --max-time 60 -o "$tarball" "$url" 2>/dev/null; then
            if tar -xzf "$tarball" -C "$TMPDIR" 2>/dev/null; then
                local binary_name="tokei"
                [[ "$platform" == windows_* ]] && binary_name="tokei.exe"

                local binary_path="${TMPDIR}/${binary_name}"
                if [[ -f "$binary_path" ]]; then
                    gzip -c "$binary_path" > "$output"
                    rm -f "$binary_path"
                    echo "$TOKEI_VERSION" > "$version_file"
                    echo "[$platform] OK ($(file_size "$output") bytes, from GitHub)"
                    downloaded=true
                fi
            fi
            rm -f "$tarball"
        else
            echo "[$platform] Official GitHub release download failed"
        fi
    fi

    # Priority 5: cargo install for host target only
    if [[ "$downloaded" == false && -n "$HOST_TARGET" && "$HOST_TARGET" == "$target" ]]; then
        echo "[$platform] Falling back to 'cargo install' ..."
        local cargo_root="${TMPDIR}/cargo-${platform}"
        if cargo install tokei --version "$TOKEI_VERSION" --root "$cargo_root" --quiet 2>/dev/null; then
            local binary_path="${cargo_root}/bin/tokei"
            [[ "$platform" == windows_* ]] && binary_path="${binary_path}.exe"
            if [[ -f "$binary_path" ]]; then
                gzip -c "$binary_path" > "$output"
                echo "$TOKEI_VERSION" > "$version_file"
                echo "[$platform] OK ($(file_size "$output") bytes, from cargo)"
                downloaded=true
            fi
        else
            echo "[$platform] cargo install failed"
        fi
    fi

    if [[ "$downloaded" == false ]]; then
        return 1
    fi
    return 0
}

failed_platforms=()

fetch_one "linux_amd64" "x86_64-unknown-linux-musl" || failed_platforms+=("linux_amd64 (x86_64-unknown-linux-musl)")
fetch_one "linux_arm64" "aarch64-unknown-linux-musl" || failed_platforms+=("linux_arm64 (aarch64-unknown-linux-musl)")
fetch_one "darwin_amd64" "x86_64-apple-darwin" || failed_platforms+=("darwin_amd64 (x86_64-apple-darwin)")
fetch_one "darwin_arm64" "aarch64-apple-darwin" || failed_platforms+=("darwin_arm64 (aarch64-apple-darwin)")
fetch_one "windows_amd64" "x86_64-pc-windows-msvc" || failed_platforms+=("windows_amd64 (x86_64-pc-windows-msvc)")

if [[ ${#failed_platforms[@]} -gt 0 ]]; then
    echo ""
    echo "WARNING: Failed to fetch tokei for the following platforms:"
    for fp in "${failed_platforms[@]}"; do
        echo "  - $fp"
    done
    echo ""
    echo "You can manually download them from:"
    echo "  https://github.com/XAMPPRocky/tokei/releases/tag/v${TOKEI_VERSION}"
    echo ""
    echo "Then place the extracted binary in the corresponding directory and gzip it:"
    for fp in "${failed_platforms[@]}"; do
        platform=$(echo "$fp" | cut -d' ' -f1)
        echo "  gzip -c tokei > internal/binaries/embed/${platform}/tokei.gz"
        echo "  echo '${TOKEI_VERSION}' > internal/binaries/embed/${platform}/version.txt"
    done
    echo ""
    echo "Placeholder files are already in place, so the build will still compile."
    echo "At runtime, tokui will fallback to system PATH if the embedded binary is a placeholder."
    exit 1
fi

echo ""
echo "All tokei binaries fetched successfully (version ${TOKEI_VERSION})."

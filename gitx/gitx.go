// Package gitx wraps the system git binary to provide change sets and
// historical snapshots for tokui's diff and compare modes. It shells out to
// git via os/exec and adds no third-party dependencies.
package gitx

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNotGitRepo is returned (wrapped) by RepoRoot when the given path is not
// inside a git repository. Use errors.Is to detect it.
var ErrNotGitRepo = errors.New("not a git repository")

// ErrFileTooLarge is returned (wrapped) by ShowFile and DiffFile when the
// contents at the given ref exceed the size limit.
var ErrFileTooLarge = errors.New("file too large")

// maxShowFileSize caps ShowFile and DiffFile output, matching the on-disk
// preview limit of the render layer.
const maxShowFileSize = 10 << 20 // 10 MB

// ChangeKind describes how a file changed between two git snapshots.
type ChangeKind int

const (
	Modified ChangeKind = iota
	Added
	Deleted
	Renamed
)

// FileChange describes the churn of a single file in a diff range.
type FileChange struct {
	Path    string // S2-side path relative to the repo root; the S1-side path when Kind is Deleted
	OldPath string // set only when Kind is Renamed
	Added   int64
	Deleted int64
	Kind    ChangeKind
}

// rejectOption rejects user-supplied values that would be interpreted as a
// git command-line option.
func rejectOption(what, value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("invalid %s %q: must not start with '-'", what, value)
	}
	return nil
}

// Numstat returns the per-file churn of a diff by running
// "git diff --numstat -z -M [--cached] [rev] --" joined with
// "--name-status -z -M" for accurate change kinds and "--raw -z -M" to
// identify submodule (gitlink) entries. An empty rev with cached=false diffs
// the worktree against the index (unstaged changes, like plain "git diff");
// an empty rev with cached=true diffs the index against HEAD (like
// "git diff --cached"). A rev that yields no differences produces an empty
// slice. Binary files are kept with zero Added/Deleted counts; submodule
// entries are skipped.
func Numstat(repoRoot, rev string, cached bool) ([]FileChange, error) {
	if err := rejectOption("diff range", rev); err != nil {
		return nil, err
	}
	// The trailing "--" after the rev forces revision interpretation: a
	// mistyped rev that matches a directory name must fail loudly instead of
	// silently degrading to a pathspec diff.
	diffArgs := func(outputFlag string) []string {
		args := []string{"diff", outputFlag, "-z", "-M"}
		if cached {
			args = append(args, "--cached")
		}
		if rev != "" {
			args = append(args, rev)
		}
		return append(args, "--")
	}
	numstatOut, err := gitOutput(repoRoot, diffArgs("--numstat")...)
	if err != nil {
		return nil, err
	}
	statusOut, err := gitOutput(repoRoot, diffArgs("--name-status")...)
	if err != nil {
		return nil, err
	}
	rawOut, err := gitOutput(repoRoot, diffArgs("--raw")...)
	if err != nil {
		return nil, err
	}

	churn := parseNumstat(numstatOut)
	statuses := parseNameStatus(statusOut)
	submodules := submodulePaths(rawOut)

	changes := make([]FileChange, 0, len(statuses))
	for _, st := range statuses {
		if submodules[st.path] {
			continue
		}
		entry := churn[st.path]
		changes = append(changes, FileChange{
			Path:    st.path,
			OldPath: st.oldPath,
			Added:   entry.added,
			Deleted: entry.deleted,
			Kind:    st.kind,
		})
	}
	return changes, nil
}

// RepoRoot returns the root directory of the git repository containing path.
// It returns an error wrapping ErrNotGitRepo when path is not inside a git
// repository.
func RepoRoot(path string) (string, error) {
	out, err := gitOutput(path, "rev-parse", "--show-toplevel")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("git not found in PATH (please install git): %w", err)
		}
		return "", fmt.Errorf("%w: %s", ErrNotGitRepo, path)
	}
	// git prints forward slashes even on Windows; normalize to OS separators.
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(string(out)))), nil
}

// MergeBase returns the best common ancestor of a and b ("git merge-base a b"),
// the diff base of a three-dot range. It returns an error when the two refs
// have no common ancestor.
func MergeBase(repoRoot, a, b string) (string, error) {
	if err := rejectOption("ref", a); err != nil {
		return "", err
	}
	if err := rejectOption("ref", b); err != nil {
		return "", err
	}
	out, err := gitOutput(repoRoot, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// emptyTree is the well-known hash of git's empty tree object. It plays the
// parent role for root commits so their churn can be expressed as a range.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// ShowRange expands a single commit into the diff range "commit^..commit" —
// the churn "git show <commit>" reports (for merge commits, against the
// first parent). It returns the range along with the S1 (base) and S2
// (target) preview refs. A root commit has no parent: the empty tree takes
// the S1 role instead, so every file in the commit comes out as added.
func ShowRange(repoRoot, commit string) (rangeSpec, s1Ref, s2Ref string, err error) {
	if err := rejectOption("commit", commit); err != nil {
		return "", "", "", err
	}
	if _, err := gitOutput(repoRoot, "rev-parse", "--verify", "-q", commit+"^{commit}"); err != nil {
		return "", "", "", fmt.Errorf("unknown commit %q", commit)
	}
	parent := commit + "^"
	if _, err := gitOutput(repoRoot, "rev-parse", "--verify", "-q", parent); err != nil {
		// No parent: a root commit, diffed against the empty tree.
		return emptyTree + ".." + commit, emptyTree, commit, nil
	}
	return parent + ".." + commit, parent, commit, nil
}

// Archive extracts the working tree of ref into a temporary directory and
// returns the directory path along with a cleanup function that removes it.
// The ref is resolved in the repository at the current working directory.
func Archive(ref string) (dir string, cleanup func(), err error) {
	if err := rejectOption("ref", ref); err != nil {
		return "", nil, err
	}
	dir, err = os.MkdirTemp("", "tokui-archive-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	// Write the archive to a file instead of streaming it through a pipe:
	// tar output is padded to whole records, and an extract-side early exit
	// can leave git blocked on a full pipe.
	tarFile, err := os.CreateTemp("", "tokui-archive-*.tar")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create temporary archive file: %w", err)
	}
	tarPath := tarFile.Name()
	_ = tarFile.Close()
	defer func() { _ = os.Remove(tarPath) }()

	var stderr bytes.Buffer
	cmd := exec.Command("git", "archive", "--output="+tarPath, ref)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git archive %s failed: %w: %s", ref, err, strings.TrimSpace(stderr.String()))
	}

	f, err := os.Open(tarPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to read git archive of %s: %w", ref, err)
	}
	defer func() { _ = f.Close() }()
	if err := extractTar(f, dir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to extract git archive of %s: %w", ref, err)
	}
	return dir, cleanup, nil
}

// ShowFile returns the contents of path at the given ref ("git show
// ref:path"). An empty ref reads the staged blob from the index ("git show
// :path"). Contents larger than 10 MB are rejected with an error wrapping
// ErrFileTooLarge.
func ShowFile(repoRoot, ref, path string) ([]byte, error) {
	if err := rejectOption("ref", ref); err != nil {
		return nil, err
	}
	if err := rejectOption("path", path); err != nil {
		return nil, err
	}
	data, err := gitOutput(repoRoot, "show", ref+":"+path)
	if err != nil {
		return nil, err
	}
	if len(data) > maxShowFileSize {
		return nil, fmt.Errorf("%w: %s at %s exceeds %d bytes", ErrFileTooLarge, path, ref, maxShowFileSize)
	}
	return data, nil
}

// DiffFile returns the unified diff of a single file ("git diff --no-color
// --no-ext-diff -M [--cached] [rev] -- path [oldPath]"), used by the render
// layer for diff previews. The rev/cached pair carries the same meaning as in
// Numstat: an empty rev with cached=false diffs the worktree against the
// index; cached=true diffs the index against rev (default HEAD). oldPath is
// only set for renames, so the pathspec covers both sides and -M can report
// the rename. Output larger than 10 MB is rejected with an error wrapping
// ErrFileTooLarge.
func DiffFile(repoRoot, rev string, cached bool, path, oldPath string) (string, error) {
	if err := rejectOption("diff range", rev); err != nil {
		return "", err
	}
	if err := rejectOption("path", path); err != nil {
		return "", err
	}
	if oldPath != "" {
		if err := rejectOption("path", oldPath); err != nil {
			return "", err
		}
	}
	args := []string{"diff", "--no-color", "--no-ext-diff", "-M"}
	if cached {
		args = append(args, "--cached")
	}
	if rev != "" {
		args = append(args, rev)
	}
	args = append(args, "--", path)
	if oldPath != "" {
		args = append(args, oldPath)
	}

	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	// Keep user-configured external diff drivers out of the output even when
	// they are forced through the environment rather than git config.
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_EXTERNAL_DIFF=") || strings.HasPrefix(kv, "GIT_DIFF_OPTS=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("failed to execute git: %w", err)
	}
	if len(out) > maxShowFileSize {
		return "", fmt.Errorf("%w: diff of %s exceeds %d bytes", ErrFileTooLarge, path, maxShowFileSize)
	}
	return string(out), nil
}

// gitOutput runs git with -C dir and returns its standard output.
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("failed to execute git: %w", err)
	}
	return out, nil
}

type numstatEntry struct {
	added   int64
	deleted int64
}

// parseNumstat parses "git diff --numstat -z -M" output into a map keyed by
// the S2-side path. With -z, each record is one NUL-terminated token of the
// form "added\tdeleted\tpath"; rename records have an empty path field
// ("added\tdeleted\t") followed by two more NUL-terminated tokens holding
// the old and new paths. Binary files show "- -" as counts and are kept
// with zero counts.
func parseNumstat(out []byte) map[string]numstatEntry {
	entries := make(map[string]numstatEntry)
	tokens := bytes.Split(out, []byte{0})
	for i := 0; i < len(tokens); i++ {
		fields := bytes.SplitN(tokens[i], []byte("\t"), 3)
		if len(fields) != 3 {
			continue
		}
		var path string
		if len(fields[2]) > 0 {
			path = string(fields[2])
		} else {
			// Rename: the old and new paths follow as separate tokens.
			if i+2 >= len(tokens) {
				break
			}
			path = string(tokens[i+2])
			i += 2
		}
		if path == "" {
			continue
		}
		if bytes.Equal(fields[0], []byte("-")) && bytes.Equal(fields[1], []byte("-")) {
			entries[path] = numstatEntry{}
			continue
		}
		added, _ := strconv.ParseInt(string(fields[0]), 10, 64)
		deleted, _ := strconv.ParseInt(string(fields[1]), 10, 64)
		entries[path] = numstatEntry{added: added, deleted: deleted}
	}
	return entries
}

type statusEntry struct {
	path    string
	oldPath string
	kind    ChangeKind
}

// parseNameStatus parses "git diff --name-status -z -M" output. With -z,
// each record is a NUL-terminated status token ("M", "A", "D", "R100", ...)
// followed by one path token, or two path tokens (old, new) for renames and
// copies. Rename entries are keyed by the new path so they join with the
// numstat map.
func parseNameStatus(out []byte) []statusEntry {
	var entries []statusEntry
	tokens := bytes.Split(out, []byte{0})
	for i := 0; i < len(tokens); i++ {
		status := tokens[i]
		if len(status) == 0 {
			continue
		}
		if status[0] == 'R' || status[0] == 'C' {
			if i+2 < len(tokens) {
				entries = append(entries, statusEntry{
					path:    string(tokens[i+2]),
					oldPath: string(tokens[i+1]),
					kind:    Renamed,
				})
			}
			i += 2
			continue
		}
		if i+1 < len(tokens) {
			kind := Modified
			switch status[0] {
			case 'A':
				kind = Added
			case 'D':
				kind = Deleted
			}
			entries = append(entries, statusEntry{path: string(tokens[i+1]), kind: kind})
		}
		i++
	}
	return entries
}

// submodulePaths parses "git diff --raw -z -M" output and returns the set of
// paths that are submodule (gitlink, mode 160000) entries on either side of
// the diff. Each record is a NUL-terminated header token of the form
// ":oldmode newmode oldsha newsha STATUS" followed by one path token, or two
// (old, new) for renames and copies.
func submodulePaths(out []byte) map[string]bool {
	subs := make(map[string]bool)
	tokens := bytes.Split(out, []byte{0})
	for i := 0; i < len(tokens); i++ {
		rec := tokens[i]
		if len(rec) == 0 || rec[0] != ':' {
			continue
		}
		fields := strings.Fields(string(rec))
		if len(fields) < 5 {
			continue
		}
		isSubmodule := fields[0] == ":160000" || fields[1] == "160000"
		twoPaths := len(fields[4]) > 0 && (fields[4][0] == 'R' || fields[4][0] == 'C')
		if isSubmodule {
			if i+1 < len(tokens) {
				subs[string(tokens[i+1])] = true
			}
			if twoPaths && i+2 < len(tokens) {
				subs[string(tokens[i+2])] = true
			}
		}
		if twoPaths {
			i += 2
		} else {
			i++
		}
	}
	return subs
}

// extractTar unpacks a tar stream (as produced by "git archive") into dest.
func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Resolve the link target relative to the link's directory and
			// refuse links that would point outside dest.
			resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(hdr.Linkname))
			if filepath.IsAbs(hdr.Linkname) ||
				!strings.HasPrefix(resolved, filepath.Clean(dest)+string(os.PathSeparator)) {
				return fmt.Errorf("archive symlink escapes destination: %s -> %s", hdr.Name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}

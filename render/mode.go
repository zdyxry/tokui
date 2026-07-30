package render

// ModeKind identifies which data mode the view is presenting.
type ModeKind int

const (
	// ModeFull is the default whole-snapshot view.
	ModeFull ModeKind = iota
	// ModeDiff shows git churn for a range on top of the S2 snapshot.
	ModeDiff
	// ModeCompare compares statistics between two git refs.
	ModeCompare
	// ModeRef shows a single git ref snapshot.
	ModeRef
)

// ModeInfo carries the CLI-selected data mode to the view layer. The git
// fields are only populated in Diff and Compare modes and drive the status
// bar summary and the per-file diff preview.
type ModeInfo struct {
	Kind  ModeKind
	Range string // status-bar label for the diff range or ref

	RepoRoot   string // repository root; entry paths live under it
	DiffRev    string // rev/range for per-file "git diff" previews; "" = worktree vs index
	DiffCached bool   // staged diff ("git diff --cached")
}

// Diff reports whether the view is in Diff mode.
func (m ModeInfo) Diff() bool {
	return m.Kind == ModeDiff
}

// Compare reports whether the view is in Compare mode.
func (m ModeInfo) Compare() bool {
	return m.Kind == ModeCompare
}

// Changed reports whether the view shows change information (Diff or Compare
// mode). It gates the changed-only filter, the "a" key, the per-file diff
// preview and the change-aware status bar.
func (m ModeInfo) Changed() bool {
	return m.Diff() || m.Compare()
}

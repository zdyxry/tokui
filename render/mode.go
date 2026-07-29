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
// bar summary and the S1/S2 preview switching.
type ModeInfo struct {
	Kind  ModeKind
	Range string // the --diff range as typed by the user

	RepoRoot string // repository root; entry paths live under it
	S1Ref    string // base ref for "git show" previews
	S1Label  string // human-readable S1 marker, e.g. "main"
	S2Ref    string // target ref; empty means the working tree
	S2Label  string // human-readable S2 marker, e.g. "worktree"
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
// mode). It gates the changed-only filter, the "a" key, the version-switching
// preview and the change-aware status bar.
func (m ModeInfo) Changed() bool {
	return m.Diff() || m.Compare()
}

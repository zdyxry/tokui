package structure

import "github.com/zdyxry/tokui/gitx"

// Change describes the churn of an entry between two git snapshots. It is
// attached to Entry alongside CodeStats: CodeStats remains the S2 snapshot
// statistics, while Change carries the diff-side information.
type Change struct {
	Added   int64
	Deleted int64
	Kind    gitx.ChangeKind
	// Present reports whether the entry is part of the change set. It is true
	// for every changed file — including binary files and pure renames whose
	// Added/Deleted churn is zero — and for directories that aggregate at
	// least one changed descendant. The changed-only view filters on Present
	// instead of the numeric fields so zero-churn changes stay visible.
	Present bool
	// OldPath is the S1-side path of a renamed file (Diff mode only), used by
	// the preview layer so the "git diff" pathspec covers both sides of the
	// rename. Empty for non-rename entries and in Compare mode, where a
	// rename appears as delete+add.
	OldPath string
	// Compare-mode only (populated by BuildFromCompare):
	PrevCode       int64 // S1-side Code
	PrevComplexity int64 // S1-side Complexity
}

// Delta returns the net line change of the entry.
func (c Change) Delta() int64 { return c.Added - c.Deleted }

// Add accumulates another Change into c. Kind and OldPath are not aggregated:
// they are meaningless for directory rows.
func (c *Change) Add(other Change) {
	c.Added += other.Added
	c.Deleted += other.Deleted
	c.PrevCode += other.PrevCode
	c.PrevComplexity += other.PrevComplexity
	c.Present = c.Present || other.Present
}

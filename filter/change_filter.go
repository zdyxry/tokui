package filter

import (
	"github.com/zdyxry/tokui/structure"
)

const (
	// ID for the changed-only filter used in Diff mode
	ChangedFilterID ID = "ChangedFilter"
)

// ChangedFilter hides entries without any change. It is used in Diff and
// Compare modes to toggle between "changed files only" and the full snapshot;
// toggling never rebuilds the tree. What counts as changed is mode-specific
// and supplied as a predicate: in Diff mode membership in the change set
// (Change.Present, which also covers zero-churn binary files and pure
// renames), in Compare mode a Code or Complexity delta. Directories aggregate
// their children, so a directory stays visible as long as any descendant
// changed.
type ChangedFilter struct {
	enabled bool
	changed func(*structure.Entry) bool
}

// NewChangedFilter creates a changed-only filter using the Diff-mode
// predicate (the entry is part of the change set). enabled sets the initial
// state (git modes start in changed-only view).
func NewChangedFilter(enabled bool) *ChangedFilter {
	return NewChangedFilterFunc(enabled, func(e *structure.Entry) bool {
		return e.Change.Present
	})
}

// NewChangedFilterFunc creates a changed-only filter with a custom predicate.
func NewChangedFilterFunc(enabled bool, changed func(*structure.Entry) bool) *ChangedFilter {
	return &ChangedFilter{enabled: enabled, changed: changed}
}

// ID returns the unique identifier of the filter.
func (cf *ChangedFilter) ID() ID {
	return ChangedFilterID
}

// Toggle switches the enabled state of the filter.
func (cf *ChangedFilter) Toggle() {
	cf.enabled = !cf.enabled
}

// IsEnabled returns whether the filter is enabled.
func (cf *ChangedFilter) IsEnabled() bool {
	return cf.enabled
}

// Filter passes changed entries (or all entries when disabled).
func (cf *ChangedFilter) Filter(e *structure.Entry) bool {
	if !cf.enabled || cf.changed == nil {
		return true
	}
	return cf.changed(e)
}

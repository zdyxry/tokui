// Package provider abstracts the code-statistics backend used by tokui.
//
// Implementations (e.g. tokei, scc) must satisfy the Provider interface. Each
// implementation advertises its capabilities via Capability bit flags so the
// UI can decide which columns to render.
package provider

// Capability describes a metric family that a Provider can produce.
type Capability uint

const (
	CapLines Capability = 1 << iota
	CapComplexity
	CapBytes
	CapULOC // reserved for future use
)

// Info describes a Provider implementation.
type Info struct {
	Name         string
	Version      string
	Capabilities Capability
}

// FileStats holds per-file statistics produced by a Provider.
type FileStats struct {
	Path       string
	Language   string
	Code       int64
	Comments   int64
	Blanks     int64
	Complexity int64 // valid when CapComplexity is set
	Bytes      int64 // valid when CapBytes is set
}

// Result is the top-level output of an analysis run.
type Result struct {
	Files []FileStats
}

// Provider is the abstraction for a code statistics backend.
type Provider interface {
	// Info returns metadata and capabilities for this Provider.
	Info() Info

	// Analyze scans the directory or file at path and returns per-file stats.
	Analyze(path string) (Result, error)

	// ParseStdin parses Provider-specific data from the supplied byte slice
	// (typically the contents of os.Stdin) and returns per-file stats.
	ParseStdin(data []byte) (Result, error)
}

package render

// Mode represents the current UI mode of the directory model.
type Mode string

const (
	PENDING Mode = "PENDING"
	READY   Mode = "READY"
	INPUT   Mode = "INPUT"
	PREVIEW Mode = "PREVIEW"
	SEARCH  Mode = "SEARCH"
)

const (
	SELECT_LANG Mode = "SELECT_LANG"
)

// CycleLangFilter is a message requesting to cycle the language filter.
type CycleLangFilter struct{}

// OpenFileInEditor represents a message to open a file in the default editor.
type OpenFileInEditor struct {
	Path string
}

// EditorFinished represents a message that the editor has finished.
type EditorFinished struct {
	Err error
}

// ErrorMsg represents a message containing an error.
type ErrorMsg struct {
	Err error
}

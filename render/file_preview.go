package render

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/structure"
)

// FilePreview represents the file preview component using viewport
type FilePreview struct {
	viewport viewport.Model
	filePath string
	fileName string
	width    int
	height   int
	ready    bool
	content  string
	errorMsg string

	// Diff/Compare modes show the file's diff instead of its contents;
	// diffRange is the mode's range label displayed in the title bar. Empty
	// outside Diff/Compare modes.
	diffRange string
	// rawDiff keeps the unstyled diff so the layout can be recomputed when
	// the terminal is resized (side-by-side vs unified depends on width).
	rawDiff string
}

// NewFilePreview creates a new file preview component
func NewFilePreview(filePath string, width, height int) *FilePreview {
	return newFilePreview(filePath, width, height, ModeInfo{}, structure.Change{})
}

// NewFilePreviewDiff creates a preview for Diff and Compare modes. Instead of
// the file contents it shows the file's diff for the mode's range ("git diff"
// style: additions green, deletions red), so the change itself is visible
// directly. Renamed files resolve through Change.OldPath so the pathspec
// covers both sides of the rename.
func NewFilePreviewDiff(filePath string, width, height int, mode ModeInfo, change structure.Change) *FilePreview {
	return newFilePreview(filePath, width, height, mode, change)
}

func newFilePreview(filePath string, width, height int, mode ModeInfo, change structure.Change) *FilePreview {
	// Calculate preview window dimensions (80% of terminal size)
	previewWidth := int(float64(width) * 0.8)
	previewHeight := int(float64(height) * 0.8)

	// Ensure minimum size
	if previewWidth < 50 {
		previewWidth = 50
	}
	if previewHeight < 15 {
		previewHeight = 15
	}

	fp := &FilePreview{
		filePath: filePath,
		fileName: filepath.Base(filePath),
		width:    previewWidth,  // Use preview width instead of terminal width
		height:   previewHeight, // Use preview height instead of terminal height
	}

	// Initialize viewport - leave space for borders, title, footer and padding
	viewportWidth := previewWidth - 10  // Leave space for box borders and padding
	viewportHeight := previewHeight - 8 // Leave space for title, footer and padding
	fp.viewport = viewport.New(viewportWidth, viewportHeight)

	// Load content: the per-file diff in Diff/Compare modes, the file
	// contents otherwise.
	if mode.Changed() && mode.RepoRoot != "" {
		fp.diffRange = mode.Range
		fp.loadDiffContent(mode, change)
	} else {
		fp.loadFileContent()
	}

	return fp
}

// loadDiffContent loads the file's diff for the mode's range via git and sets
// it, colorized, in the viewport.
func (fp *FilePreview) loadDiffContent(mode ModeInfo, change structure.Change) {
	rel, err := filepath.Rel(mode.RepoRoot, fp.filePath)
	if err != nil {
		fp.errorMsg = fmt.Sprintf("Error resolving file path: %v", err)
		fp.content = fp.errorMsg
		fp.viewport.SetContent(fp.content)
		fp.ready = true
		return
	}
	path := filepath.ToSlash(rel)

	diff, err := gitx.DiffFile(mode.RepoRoot, mode.DiffRev, mode.DiffCached, path, change.OldPath)
	switch {
	case errors.Is(err, gitx.ErrFileTooLarge):
		fp.content = "Diff too large to preview (> 10 MB)"
	case err != nil:
		fp.errorMsg = fmt.Sprintf("Error loading diff: %v", err)
		fp.content = fp.errorMsg
	case diff == "":
		// Entries in the change set normally always produce a diff; a pure
		// rename still prints its headers, so this is only a safety net.
		fp.content = "No textual diff for this file"
	default:
		fp.rawDiff = diff
		fp.content = renderDiff(diff, fp.viewport.Width)
	}

	fp.viewport.SetContent(fp.content)
	fp.ready = true
}

// Diff colorization styles, close to "git diff" colors.
var (
	diffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	diffDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	diffHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	diffPathStyle = lipgloss.NewStyle().Bold(true)
	diffMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
)

// styleDiff colorizes unified diff output: additions green, deletions red,
// hunk headers cyan, the +++/--- file lines bold and the remaining metadata
// headers faint. Context lines pass through unchanged.
func styleDiff(diff string) string {
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines[i] = diffPathStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffDelStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = diffHunkStyle.Render(line)
		case isDiffMetaLine(line):
			lines[i] = diffMetaStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

// isDiffMetaLine reports whether line is a diff metadata header (everything
// git prints before or instead of hunks).
func isDiffMetaLine(line string) bool {
	for _, prefix := range []string{
		"diff --git", "index ", "old mode", "new mode", "new file mode",
		"deleted file mode", "similarity index", "dissimilarity index",
		"rename from ", "rename to ", "copy from ", "copy to ",
		"Binary files", "GIT binary patch", `\ No newline`,
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// loadFileContent reads the file content and sets it in the viewport
func (fp *FilePreview) loadFileContent() {
	content, err := fp.readFileContent(fp.filePath)
	if err != nil {
		fp.errorMsg = fmt.Sprintf("Error reading file: %v", err)
		fp.content = fp.errorMsg
	} else {
		fp.errorMsg = ""
		fp.content = content
	}

	fp.viewport.SetContent(fp.content)
	fp.ready = true
}

// readFileContent reads file content with size limits for safety
func (fp *FilePreview) readFileContent(filePath string) (string, error) {
	// Check if file exists and get its info
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// Limit file size to 10MB for safety
	const maxSize = 10 * 1024 * 1024
	if info.Size() > maxSize {
		return fmt.Sprintf("File too large to preview (%.2f MB > 10 MB)", float64(info.Size())/(1024*1024)), nil
	}

	// Check if it's likely a binary file by extension
	if fp.isBinaryFile(filePath) {
		return fmt.Sprintf("Binary file detected: %s\nFile size: %.2f KB\nUse appropriate tools to view this file.",
			filepath.Ext(filePath), float64(info.Size())/1024), nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close() // Explicitly ignore the error
	}()

	// Read content
	contentBytes, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	// Convert to string and check for binary content
	content := string(contentBytes)
	if fp.containsBinaryData(content) {
		return fmt.Sprintf("Binary file detected\nFile size: %.2f KB\nUse appropriate tools to view this file.",
			float64(len(contentBytes))/1024), nil
	}

	return content, nil
}

// isBinaryFile checks if file is likely binary based on extension
func (fp *FilePreview) isBinaryFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".bin", ".obj", ".o",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico", ".svg",
		".mp3", ".mp4", ".avi", ".mov", ".wav", ".flac",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}
	return false
}

// containsBinaryData checks if content contains binary data
func (fp *FilePreview) containsBinaryData(content string) bool {
	// Check for null bytes which indicate binary content
	return strings.Contains(content, "\x00")
}

// Init initializes the file preview component
func (fp *FilePreview) Init() tea.Cmd {
	return nil
}

// Update handles key events for the file preview
func (fp *FilePreview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Recalculate preview dimensions when terminal size changes
		previewWidth := int(float64(msg.Width) * 0.8)
		previewHeight := int(float64(msg.Height) * 0.8)

		// Ensure minimum size
		if previewWidth < 50 {
			previewWidth = 50
		}
		if previewHeight < 15 {
			previewHeight = 15
		}

		fp.width = previewWidth
		fp.height = previewHeight
		fp.viewport.Width = previewWidth - 10
		fp.viewport.Height = previewHeight - 8

		// The diff layout (side-by-side vs unified) depends on the width;
		// re-render it for the new viewport.
		if fp.rawDiff != "" {
			fp.content = renderDiff(fp.rawDiff, fp.viewport.Width)
			fp.viewport.SetContent(fp.content)
		}

	case tea.KeyMsg:
		// Handle viewport navigation keys
		switch msg.String() {
		case "home":
			fp.viewport.GotoTop()
			return fp, nil
		case "end":
			fp.viewport.GotoBottom()
			return fp, nil
		}
		fp.viewport, cmd = fp.viewport.Update(msg)
	case tea.MouseMsg:
		// Forward mouse events (e.g. scroll wheel) to the viewport.
		fp.viewport, cmd = fp.viewport.Update(msg)
	}

	return fp, cmd
}

// View renders the file preview
func (fp *FilePreview) View() string {
	if !fp.ready {
		return fp.renderBox("Loading...")
	}

	if fp.errorMsg != "" {
		return fp.renderBox(fp.errorMsg)
	}

	// Create the title: Diff/Compare modes show the range label next to the
	// file name.
	title := fmt.Sprintf(" File Preview: %s ", fp.fileName)
	if fp.diffRange != "" {
		title = fmt.Sprintf(" Diff: %s · %s ", fp.fileName, fp.diffRange)
	}

	// Create the content with viewport
	content := fp.viewport.View()

	// Add scroll indicators
	scrollInfo := ""
	if len(strings.Split(fp.content, "\n")) > 0 {
		currentLine := fp.viewport.YOffset + 1
		totalLines := len(strings.Split(fp.content, "\n"))
		scrollInfo = fmt.Sprintf(" %d/%d ", currentLine, totalLines)
	}

	return fp.renderBoxWithContent(title, content, scrollInfo)
}

// renderBox renders a simple box with content
func (fp *FilePreview) renderBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(fp.width - 4).
		Height(fp.height - 4)

	return boxStyle.Render(content)
}

// renderBoxWithContent renders a box with title, content and scroll info
func (fp *FilePreview) renderBoxWithContent(title, content, scrollInfo string) string {
	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	// Help text style
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Faint(true)

	// Scroll info style
	scrollStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Faint(true)

	// Calculate inner width for consistent alignment
	innerWidth := fp.width - 6 // Account for border and padding

	// Create the header with title (centered)
	header := titleStyle.Width(innerWidth).Align(lipgloss.Center).Render(title)

	// Create the footer with help text and scroll info
	helpText := "Press q/Esc to close, ↑/↓/j/k to scroll, PgUp/PgDn/Home/End to navigate"
	var footer string
	if scrollInfo != "" {
		// Calculate spacing for justified layout
		availableSpace := innerWidth - lipgloss.Width(helpText) - lipgloss.Width(scrollInfo)
		if availableSpace > 0 {
			footer = helpStyle.Render(helpText) + strings.Repeat(" ", availableSpace) + scrollStyle.Render(scrollInfo)
		} else {
			footer = helpStyle.Render(helpText)
		}
	} else {
		footer = helpStyle.Render(helpText)
	}

	// Ensure footer width matches inner width
	footer = lipgloss.NewStyle().Width(innerWidth).Render(footer)

	// Box content style with consistent padding
	contentStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(0, 1)

	// Combine all parts with consistent alignment
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"", // Add a blank line separator
		contentStyle.Render(content),
		"", // Add a blank line separator
		footer,
	)

	// Final box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(fp.width-4).
		Padding(1, 1)

	return boxStyle.Render(fullContent)
}

// GetViewportModel returns the underlying viewport model for external control
func (fp *FilePreview) GetViewportModel() *viewport.Model {
	return &fp.viewport
}

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

	// Diff-mode S1/S2 version switching. s2Ref empty means the working tree.
	repoRoot  string
	relPath   string // repo-relative path used for "git show"
	s1Ref     string
	s1Label   string
	s2Ref     string
	s2Label   string
	showingS1 bool
	s2Missing bool // no S2 version (deleted file); preview is S1-only

	// Change metadata of the previewed file (Diff/Compare modes).
	kind    gitx.ChangeKind
	oldPath string // S1-side path for renamed files
}

// NewFilePreview creates a new file preview component
func NewFilePreview(filePath string, width, height int) *FilePreview {
	return newFilePreview(filePath, width, height, ModeInfo{}, structure.Change{})
}

// NewFilePreviewDiff creates a file preview for Diff and Compare modes. The
// S2 version is shown first (working tree, or the S2 ref when the range
// targets a historical snapshot); the "v" key switches to the S1 version
// loaded on demand via "git show". Deleted files fall back to the S1 version
// directly; renamed files resolve their S1 content through Change.OldPath;
// added files have no S1 version and show a hint instead of a git error.
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

	if mode.Changed() && mode.RepoRoot != "" {
		fp.repoRoot = mode.RepoRoot
		fp.s1Ref = mode.S1Ref
		fp.s1Label = mode.S1Label
		fp.s2Ref = mode.S2Ref
		fp.s2Label = mode.S2Label
		fp.kind = change.Kind
		fp.oldPath = change.OldPath
		if rel, err := filepath.Rel(mode.RepoRoot, filePath); err == nil {
			fp.relPath = filepath.ToSlash(rel)
		}
	}

	// Initialize viewport - leave space for borders, title, footer and padding
	viewportWidth := previewWidth - 10  // Leave space for box borders and padding
	viewportHeight := previewHeight - 8 // Leave space for title, footer and padding
	fp.viewport = viewport.New(viewportWidth, viewportHeight)

	// Load file content
	fp.loadFileContent()

	return fp
}

// CanToggleVersion reports whether the preview can switch between the S1 and
// S2 versions of the file (Diff mode with an S1 side and an available S2
// version). The check keys on the S1 label rather than the S1 ref because a
// bare "tokui diff" has the index as S1, which is addressed with an empty
// ref ("git show :path").
func (fp *FilePreview) CanToggleVersion() bool {
	return fp.s1Label != "" && !fp.s2Missing
}

// ToggleVersion switches between the S2 and S1 versions of the file,
// reloading the content on demand.
func (fp *FilePreview) ToggleVersion() {
	if !fp.CanToggleVersion() {
		return
	}
	fp.showingS1 = !fp.showingS1
	fp.viewport.GotoTop()
	fp.loadFileContent()
}

// loadFileContent reads the file content and sets it in the viewport
func (fp *FilePreview) loadFileContent() {
	content, err := fp.loadCurrentVersion()
	if err != nil && fp.s1Label != "" && !fp.showingS1 && fp.kind == gitx.Deleted {
		// A deleted file has no S2 version: fall back to the S1 version
		// directly. Other S2 read failures surface as errors instead of being
		// misread as a deletion.
		fp.s2Missing = true
		fp.showingS1 = true
		content, err = fp.loadCurrentVersion()
	}
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

// loadCurrentVersion loads the version of the file matching the current
// showingS1 state.
func (fp *FilePreview) loadCurrentVersion() (string, error) {
	if fp.showingS1 {
		if fp.kind == gitx.Added {
			// The file did not exist in S1; don't ask git for a path it
			// cannot resolve.
			label := fp.s1Label
			if label == "" {
				label = fp.s1Ref
			}
			return fmt.Sprintf("S1 (%s) 无此文件（新增）", label), nil
		}
		path := fp.relPath
		if fp.kind == gitx.Renamed && fp.oldPath != "" {
			// The file lived under a different path in S1.
			path = fp.oldPath
		}
		return fp.readGitFile(fp.s1Ref, path)
	}
	if fp.s2Ref != "" {
		return fp.readGitFile(fp.s2Ref, fp.relPath)
	}
	return fp.readFileContent(fp.filePath)
}

// readGitFile loads the file content at the given ref via "git show",
// applying the same binary-content and size guards as filesystem reads.
func (fp *FilePreview) readGitFile(ref, path string) (string, error) {
	data, err := gitx.ShowFile(fp.repoRoot, ref, path)
	if err != nil {
		if errors.Is(err, gitx.ErrFileTooLarge) {
			return "File too large to preview (> 10 MB)", nil
		}
		return "", err
	}
	content := string(data)
	if fp.containsBinaryData(content) {
		return fmt.Sprintf("Binary file detected\nFile size: %.2f KB\nUse appropriate tools to view this file.",
			float64(len(data))/1024), nil
	}
	return content, nil
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

	// Create the title, including the S1/S2 version marker in Diff mode
	title := fmt.Sprintf(" File Preview: %s ", fp.fileName)
	if label := fp.versionLabel(); label != "" {
		title = fmt.Sprintf(" File Preview: %s · %s ", fp.fileName, label)
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

// versionLabel returns the S1/S2 marker shown in the preview title bar, or an
// empty string outside Diff mode. Like CanToggleVersion it keys on the S1
// label: a bare "tokui diff" has the index as S1 with an empty S1 ref.
func (fp *FilePreview) versionLabel() string {
	if fp.s1Label == "" {
		return ""
	}
	if fp.showingS1 {
		return fmt.Sprintf("S1: %s", fp.s1Label)
	}
	return fmt.Sprintf("S2: %s", fp.s2Label)
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
	if fp.CanToggleVersion() {
		helpText += ", v: switch S1/S2"
	}
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

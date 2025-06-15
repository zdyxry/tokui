package render

import (
	"bytes"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	headerMessage = `😵 🚨 ⛔ Something went terribly wrong...`
	infoMessage   = `An unexpected error occurred during running the program.
You can help me 🙏 if you want this problem to be fixed.

Please, create a new issue at https://github.com/zdyxry/tokui/issues:

1) 💡 Click on the "New issue" button, and select "Bug report".
2) 📋 Fill up the prepared template.
3) 🚀 Click on the "Create" button.

And that's it! 😎👌🔥`
)

// ReportError formats an error with stack trace and instructions for reporting
func ReportError(err error, stackTrace []byte) string {
	re := lipgloss.NewRenderer(os.Stdout)
	bs := re.NewStyle().Padding(0, 1)

	errorHeaderStyle := bs.Foreground(lipgloss.Color("#f2133c")).Bold(true)
	errorMsgStyle := bs.Foreground(lipgloss.Color("#f6bd60")).Bold(true)

	defaultWidth := 80
	padding := 3

	stackTraceCell := errorMsgStyle.Render(fmtStackTrace(stackTrace))

	data := [][]string{
		{bs.Render(infoMessage)},
		{bs.Render("\nError message:")},
		{errorMsgStyle.Render(err.Error() + "\n")},
		{bs.Render("Stack trace:")},
		{stackTraceCell},
	}

	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(re.NewStyle().Foreground(lipgloss.Color("238"))).
		Headers(errorHeaderStyle.Render(headerMessage)).
		Width(max(defaultWidth, lipgloss.Width(stackTraceCell)) + padding).
		Rows(data...).
		Render()
}

// fmtStackTrace trims stack trace to show only relevant application logic
func fmtStackTrace(stackTrace []byte) string {
	if len(stackTrace) == 0 {
		return "no stack trace data"
	}

	// skip the source of panic
	startIdx := bytes.LastIndex(stackTrace, []byte("panic"))
	if startIdx != -1 {
		stackTrace = stackTrace[startIdx:]
		stackTrace = stackTrace[bytes.IndexByte(stackTrace, '\n'):]
	}

	// skip CLI tool stack trace
	endIdx := bytes.Index(stackTrace, []byte("github.com/spf13/cobra"))
	if endIdx != -1 {
		stackTrace = stackTrace[:endIdx]
	}

	return string(bytes.TrimSpace(stackTrace))
}

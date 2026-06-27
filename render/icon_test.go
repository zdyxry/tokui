package render

import (
	"testing"

	"github.com/zdyxry/tokui/structure"
)

func TestEntryIcon(t *testing.T) {
	tests := []struct {
		name  string
		entry *structure.Entry
		want  string
	}{
		{
			name:  "directory without children",
			entry: structure.NewDirEntry("empty-dir"),
			want:  "📁",
		},
		{
			name: "directory with children",
			entry: func() *structure.Entry {
				d := structure.NewDirEntry("dir")
				d.AddChild(structure.NewFileEntry("dir/a.go", nil))
				return d
			}(),
			want: "📂",
		},
		{name: "go file", entry: structure.NewFileEntry("a.go", nil), want: "💻"},
		{name: "py file", entry: structure.NewFileEntry("a.py", nil), want: "💻"},
		{name: "js file", entry: structure.NewFileEntry("a.js", nil), want: "💻"},
		{name: "json file", entry: structure.NewFileEntry("a.json", nil), want: "🔧"},
		{name: "jpg file", entry: structure.NewFileEntry("a.jpg", nil), want: "🖼"},
		{name: "mp4 file", entry: structure.NewFileEntry("a.mp4", nil), want: "🎞"},
		{name: "zip file", entry: structure.NewFileEntry("a.zip", nil), want: "🗃"},
		{name: "mp3 file", entry: structure.NewFileEntry("a.mp3", nil), want: "🎵"},
		{name: "exe file", entry: structure.NewFileEntry("a.exe", nil), want: "📦"},
		{name: "doc file", entry: structure.NewFileEntry("a.doc", nil), want: "📝"},
		{name: "html file", entry: structure.NewFileEntry("a.html", nil), want: "🌐"},
		{name: "pdf file", entry: structure.NewFileEntry("a.pdf", nil), want: "📕"},
		{name: "md file", entry: structure.NewFileEntry("a.md", nil), want: "📜"},
		{name: "log file", entry: structure.NewFileEntry("a.log", nil), want: "📗"},
		{name: "unknown extension", entry: structure.NewFileEntry("a.unknown", nil), want: "📄"},
		{name: "no extension", entry: structure.NewFileEntry("README", nil), want: "📄"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EntryIcon(tt.entry); got != tt.want {
				t.Errorf("EntryIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

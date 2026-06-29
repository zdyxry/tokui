package structure

import (
	"cmp"
	"testing"
)

func TestSortChildBy(t *testing.T) {
	tests := []struct {
		name string
		sort ChildSortFunc
		want []int64
	}{
		{
			name: "ascending by total",
			sort: func(a, b *Entry) int { return cmp.Compare(a.TotalStats.Total(), b.TotalStats.Total()) },
			want: []int64{10, 20, 30},
		},
		{
			name: "descending by total",
			sort: func(a, b *Entry) int { return cmp.Compare(b.TotalStats.Total(), a.TotalStats.Total()) },
			want: []int64{30, 20, 10},
		},
		{
			name: "ascending by name",
			sort: func(a, b *Entry) int { return cmp.Compare(a.Name(), b.Name()) },
			want: []int64{20, 10, 30}, // a.go, b.go, c.go -> totals 20, 10, 30
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewDirEntry("root")
			root.AddChild(&Entry{Path: "root/c.go", TotalStats: CodeStats{Code: 30}})
			root.AddChild(&Entry{Path: "root/a.go", TotalStats: CodeStats{Code: 20}})
			root.AddChild(&Entry{Path: "root/b.go", TotalStats: CodeStats{Code: 10}})

			root.SortChildBy(tt.sort)

			if len(root.Child) != len(tt.want) {
				t.Fatalf("expected %d children, got %d", len(tt.want), len(root.Child))
			}
			for i, child := range root.Child {
				if child.TotalStats.Total() != tt.want[i] {
					t.Errorf("position %d: expected total %d, got %d", i, tt.want[i], child.TotalStats.Total())
				}
			}
		})
	}
}

func TestSortChild_Default(t *testing.T) {
	root := NewDirEntry("root")
	root.AddChild(&Entry{Path: "root/small.go", TotalStats: CodeStats{Code: 10}})
	root.AddChild(&Entry{Path: "root/large.go", TotalStats: CodeStats{Code: 100}})
	root.AddChild(&Entry{Path: "root/medium.go", TotalStats: CodeStats{Code: 50}})

	root.SortChild()

	want := []int64{100, 50, 10}
	for i, child := range root.Child {
		if child.TotalStats.Total() != want[i] {
			t.Errorf("position %d: expected total %d, got %d", i, want[i], child.TotalStats.Total())
		}
	}
}

func TestNewDirEntry(t *testing.T) {
	e := NewDirEntry("/foo/bar")
	if e.Path != "/foo/bar" {
		t.Errorf("expected path /foo/bar, got %q", e.Path)
	}
	if !e.IsDir {
		t.Error("expected IsDir true")
	}
	if e.Child == nil {
		t.Error("expected Child initialized")
	}
	if e.StatsByLang == nil {
		t.Error("expected StatsByLang initialized")
	}
}

func TestNewFileEntry(t *testing.T) {
	stats := map[string]CodeStats{
		"Go": {Code: 10, Comments: 2, Blanks: 1},
	}
	e := NewFileEntry("/foo/bar.go", stats)
	if e.Path != "/foo/bar.go" {
		t.Errorf("expected path /foo/bar.go, got %q", e.Path)
	}
	if e.IsDir {
		t.Error("expected IsDir false")
	}
	want := CodeStats{Code: 10, Comments: 2, Blanks: 1}
	if e.TotalStats != want {
		t.Errorf("expected total stats %+v, got %+v", want, e.TotalStats)
	}
}

func TestEntryName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/foo/bar.go", "bar.go"},
		{"bar.go", "bar.go"},
		{"/foo/", "foo"},
	}

	for _, tt := range tests {
		e := &Entry{Path: tt.path}
		if got := e.Name(); got != tt.want {
			t.Errorf("Name(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestEntryExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/foo/bar.go", "go"},
		{"bar.GO", "go"},
		{"/foo/bar", ""},
		{"/foo/.gitignore", "gitignore"},
		{"/foo/.bashrc", "bashrc"},
	}

	for _, tt := range tests {
		e := &Entry{Path: tt.path}
		if got := e.Ext(); got != tt.want {
			t.Errorf("Ext(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestEntryGetChild(t *testing.T) {
	root := NewDirEntry("root")
	child := NewFileEntry("root/child.go", nil)
	root.AddChild(child)

	if got := root.GetChild("child.go"); got != child {
		t.Errorf("expected child.go, got %v", got)
	}
	if got := root.GetChild("missing.go"); got != nil {
		t.Errorf("expected nil for missing child, got %v", got)
	}
}

func TestEntryGetChild_Concurrency(t *testing.T) {
	root := NewDirEntry("root")
	for i := 0; i < 100; i++ {
		root.AddChild(NewFileEntry("root/child.go", nil))
	}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				root.GetChild("child.go")
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestEntryAddChild(t *testing.T) {
	root := &Entry{Path: "root"}
	child := NewFileEntry("root/child.go", nil)
	root.AddChild(child)
	if len(root.Child) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Child))
	}
	if root.Child[0] != child {
		t.Error("expected child to be added")
	}
}

func TestEntryHasChild(t *testing.T) {
	root := NewDirEntry("root")
	if root.HasChild() {
		t.Error("expected no children")
	}
	root.AddChild(NewFileEntry("root/child.go", nil))
	if !root.HasChild() {
		t.Error("expected children")
	}
}

func TestEntryGetStats(t *testing.T) {
	stats := map[string]CodeStats{
		"Go":     {Code: 10, Comments: 2, Blanks: 1},
		"Python": {Code: 5, Comments: 1, Blanks: 0},
	}
	e := NewFileEntry("file.go", stats)
	e.TotalStats = CodeStats{Code: 15, Comments: 3, Blanks: 1}

	if got := e.GetStats(""); got != e.TotalStats {
		t.Errorf("empty filter should return total stats, got %+v", got)
	}
	if got := e.GetStats("All"); got != e.TotalStats {
		t.Errorf("All filter should return total stats, got %+v", got)
	}
	wantGo := CodeStats{Code: 10, Comments: 2, Blanks: 1}
	if got := e.GetStats("Go"); got != wantGo {
		t.Errorf("Go filter mismatch, got %+v, want %+v", got, wantGo)
	}
	if got := e.GetStats("Missing"); got != (CodeStats{}) {
		t.Errorf("missing language should return zero stats, got %+v", got)
	}
}

func TestEntryLanguages(t *testing.T) {
	if got := (&Entry{StatsByLang: nil}).Languages(); got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	e := NewFileEntry("file.go", map[string]CodeStats{
		"Go":     {Code: 10},
		"Python": {Code: 5},
	})
	want := []string{"Go", "Python"}
	got := e.Languages()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestEntryAggregateStats(t *testing.T) {
	root := NewDirEntry("root")
	sub := NewDirEntry("root/sub")
	root.AddChild(sub)

	sub.AddChild(NewFileEntry("root/sub/a.go", map[string]CodeStats{"Go": {Code: 10, Comments: 1, Blanks: 1}}))
	sub.AddChild(NewFileEntry("root/sub/b.py", map[string]CodeStats{"Python": {Code: 5, Comments: 1, Blanks: 1}}))
	root.AddChild(NewFileEntry("root/c.go", map[string]CodeStats{"Go": {Code: 20, Comments: 2, Blanks: 2}}))

	root.AggregateStats()

	wantTotal := CodeStats{Code: 35, Comments: 4, Blanks: 4}
	if root.TotalStats != wantTotal {
		t.Errorf("root total stats mismatch, got %+v, want %+v", root.TotalStats, wantTotal)
	}
	wantGo := CodeStats{Code: 30, Comments: 3, Blanks: 3}
	if root.StatsByLang["Go"] != wantGo {
		t.Errorf("root Go stats mismatch, got %+v, want %+v", root.StatsByLang["Go"], wantGo)
	}
	wantPython := CodeStats{Code: 5, Comments: 1, Blanks: 1}
	if root.StatsByLang["Python"] != wantPython {
		t.Errorf("root Python stats mismatch, got %+v, want %+v", root.StatsByLang["Python"], wantPython)
	}

	wantSubTotal := CodeStats{Code: 15, Comments: 2, Blanks: 2}
	if sub.TotalStats != wantSubTotal {
		t.Errorf("sub total stats mismatch, got %+v, want %+v", sub.TotalStats, wantSubTotal)
	}
}

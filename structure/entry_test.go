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

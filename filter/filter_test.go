package filter

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zdyxry/tokui/structure"
)

func TestNewNameFilter(t *testing.T) {
	nf := NewNameFilter("search")
	if nf.ID() != NameFilterID {
		t.Errorf("expected ID %q, got %q", NameFilterID, nf.ID())
	}
	if nf.IsEnabled() {
		t.Error("expected filter to be disabled by default")
	}
	if nf.input.Placeholder != "search" {
		t.Errorf("expected placeholder %q, got %q", "search", nf.input.Placeholder)
	}
}

func TestNameFilterID(t *testing.T) {
	nf := NewNameFilter("search")
	if got := nf.ID(); got != NameFilterID {
		t.Errorf("ID() = %q, want %q", got, NameFilterID)
	}
}

func TestNameFilterIsEnabled_Default(t *testing.T) {
	nf := NewNameFilter("search")
	if nf.IsEnabled() {
		t.Error("expected IsEnabled false by default")
	}
}

func TestNameFilterToggle(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	if !nf.IsEnabled() {
		t.Error("expected enabled after toggle")
	}
	nf.Toggle()
	if nf.IsEnabled() {
		t.Error("expected disabled after second toggle")
	}
}

func TestNameFilterFilter_Disabled(t *testing.T) {
	nf := NewNameFilter("search")
	e := structure.NewFileEntry("foo/bar.go", nil)
	if !nf.Filter(e) {
		t.Error("expected disabled filter to pass everything")
	}
}

func TestNameFilterFilter_CaseInsensitive(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("APP")})

	e := structure.NewFileEntry("foo/app.go", nil)
	if !nf.Filter(e) {
		t.Error("expected case-insensitive name match")
	}
}

func TestNameFilterFilter_PathNotMatched(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})

	e := structure.NewFileEntry("foo/bar.go", nil)
	if nf.Filter(e) {
		t.Error("expected path substring not to match name-only filter")
	}
}

func TestNameFilterClearInput(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	nf.ClearInput()

	if !nf.IsEnabled() {
		t.Error("expected filter to remain enabled after clear input")
	}
	if nf.input.Value() != "" {
		t.Errorf("expected input cleared, got %q", nf.input.Value())
	}
	e := structure.NewFileEntry("foo/bar.go", nil)
	if !nf.Filter(e) {
		t.Error("expected cleared filter to pass everything")
	}
}

func TestNameFilterReset(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	nf.Reset()

	if nf.IsEnabled() {
		t.Error("expected filter disabled after reset")
	}
	if nf.input.Value() != "" {
		t.Errorf("expected input cleared, got %q", nf.input.Value())
	}
}

func TestFiltersListValid_Passes(t *testing.T) {
	nf := NewNameFilter("search")
	fl := NewFiltersList(nf)

	e := structure.NewFileEntry("foo/bar.go", nil)
	if !fl.Valid(e) {
		t.Error("expected valid with passing filter")
	}
}

func TestFiltersListValid_Fails(t *testing.T) {
	nf := NewNameFilter("search")
	fl := NewFiltersList(nf)

	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("baz")})

	e := structure.NewFileEntry("foo/bar.go", nil)
	if fl.Valid(e) {
		t.Error("expected invalid when name filter fails")
	}
}

func TestFiltersListToggleFilter(t *testing.T) {
	nf := NewNameFilter("search")
	fl := NewFiltersList(nf)

	fl.ToggleFilter(NameFilterID)
	if !nf.IsEnabled() {
		t.Error("expected filter toggled")
	}

	fl.ToggleFilter("unknown")
	if !nf.IsEnabled() {
		t.Error("expected unknown ID to be no-op")
	}
}

func TestFiltersListReset(t *testing.T) {
	nf := NewNameFilter("search")
	nf.Toggle()
	nf.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})

	fl := NewFiltersList(nf)
	fl.Reset()

	if nf.IsEnabled() {
		t.Error("expected filter disabled after reset")
	}
	if nf.input.Value() != "" {
		t.Errorf("expected input cleared, got %q", nf.input.Value())
	}
}

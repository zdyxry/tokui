package filter

import (
	"github.com/zdyxry/tokui/structure"

	tea "github.com/charmbracelet/bubbletea"
)

type ID string

type Reset interface {
	Reset()
}

type Toggler interface {
	Toggle()
}

// EntryFilter defines the interface for filtering entries
type EntryFilter interface {
	ID() ID
	Filter(e *structure.Entry) bool
}

// Updater allows filters to update state during rendering
type Updater interface {
	Update(msg tea.Msg)
}

// Viewer allows filters to be rendered
type Viewer interface {
	View() string
}

type FiltersList map[ID]EntryFilter

func NewFiltersList(ef ...EntryFilter) FiltersList {
	fl := make(FiltersList, len(ef))

	for _, e := range ef {
		fl[e.ID()] = e
	}

	return fl
}

// Valid returns true if the entry passes all filters
func (fl *FiltersList) Valid(e *structure.Entry) bool {
	for _, filter := range *fl {
		if !filter.Filter(e) {
			return false
		}
	}

	return true
}

func (fl *FiltersList) ToggleFilter(id ID) {
	if _, ok := (*fl)[id]; !ok {
		return
	}

	if t, ok := (*fl)[id].(Toggler); ok {
		t.Toggle()
	}
}

func (fl *FiltersList) Reset() {
	for _, filter := range *fl {
		if r, ok := filter.(Reset); ok {
			r.Reset()
		}
	}
}

func (fl *FiltersList) Update(msg tea.Msg) {
	for _, f := range *fl {
		if u, ok := f.(Updater); ok {
			u.Update(msg)
		}
	}
}

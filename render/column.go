package render

type SortKey string

const (
	SortByNone       SortKey = ""
	SortByName       SortKey = "name"
	SortByLanguages  SortKey = "languages"
	SortByCode       SortKey = "code"
	SortByComments   SortKey = "comments"
	SortByBlanks     SortKey = "blanks"
	SortByTotal      SortKey = "total"
	SortByPercent    SortKey = "percent"
	SortByComplexity SortKey = "complexity"
)

type Column struct {
	Title   string
	SortKey SortKey
	Width   int
}

// FmtName adds sort indicator (▲/▼) to column title if it's the current sort key
func (c *Column) FmtName(sortState SortState) string {
	var order string

	if len(sortState.Key) > 0 && sortState.Key == c.SortKey {
		order = " ▲"
		if sortState.Desc {
			order = " ▼"
		}
	}

	return c.Title + order
}

type SortState struct {
	Key  SortKey
	Desc bool
}

func (ss SortState) DirectionArrow() string {
	if ss.Desc {
		return "▼"
	}
	return "▲"
}

package render

import (
	"strings"
	"testing"
)

func TestPrepareSectors(t *testing.T) {
	t.Run("sorts by value descending", func(t *testing.T) {
		raw := []RawChartSector{
			{Label: "Small", Value: 10},
			{Label: "Large", Value: 90},
			{Label: "Medium", Value: 50},
		}

		sectors := prepareSectors(150, raw)
		if len(sectors) != 3 {
			t.Fatalf("expected 3 sectors, got %d", len(sectors))
		}
		if sectors[0].label != "Large" || sectors[1].label != "Medium" || sectors[2].label != "Small" {
			t.Errorf("sectors not sorted by value descending: %+v", sectors)
		}
	})

	t.Run("merges excess sectors into Others", func(t *testing.T) {
		raw := make([]RawChartSector, 0, 10)
		for i := 0; i < 10; i++ {
			raw = append(raw, RawChartSector{Label: strings.Repeat("X", i+1), Value: float64(10 - i)})
		}

		total := 55.0
		sectors := prepareSectors(total, raw)

		hasOthers := false
		for _, s := range sectors {
			if s.label == "Others" {
				hasOthers = true
			}
		}
		if !hasOthers {
			t.Errorf("expected 'Others' sector for excess values")
		}
		if len(sectors) > maxSectors+1 {
			t.Errorf("expected at most %d sectors, got %d", maxSectors+1, len(sectors))
		}
	})

	t.Run("handles totalValue=0", func(t *testing.T) {
		raw := []RawChartSector{
			{Label: "A", Value: 10},
			{Label: "B", Value: 20},
		}

		sectors := prepareSectors(0, raw)
		if len(sectors) != 2 {
			t.Fatalf("expected 2 sectors, got %d", len(sectors))
		}
		for _, s := range sectors {
			if s.usage != 0 {
				t.Errorf("expected usage 0 for totalValue=0, got %f", s.usage)
			}
		}
	})
}

func TestIsAngleInSector(t *testing.T) {
	t.Run("returns true for angle inside sector", func(t *testing.T) {
		if !isAngleInSector(1.0, 0.5, 1.5) {
			t.Errorf("expected angle 1.0 to be inside sector (0.5, 1.5)")
		}
	})

	t.Run("handles sector crossing 0 degrees", func(t *testing.T) {
		if !isAngleInSector(6.0, 5.5, 0.5) {
			t.Errorf("expected angle 6.0 to be inside crossing sector")
		}
		if isAngleInSector(2.0, 5.5, 0.5) {
			t.Errorf("expected angle 2.0 to be outside crossing sector")
		}
	})
}

func TestChart(t *testing.T) {
	raw := []RawChartSector{
		{Label: "Go", Value: 60},
		{Label: "Python", Value: 40},
	}

	t.Run("returns non-empty string for valid input", func(t *testing.T) {
		got := Chart(20, 10, 3, 100, raw)
		if got == "" {
			t.Errorf("Chart() returned empty string")
		}
		if !strings.Contains(got, "█") {
			t.Errorf("Chart() output missing expected block characters")
		}
	})

	t.Run("returns empty-ish string for zero width/height", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Chart panicked with zero dimensions: %v", r)
			}
		}()

		got := Chart(0, 0, 1, 100, raw)
		if got == "" {
			t.Errorf("Chart() returned empty string")
		}
	})
}

func TestLegend(t *testing.T) {
	sectors := []chartSector{
		{label: "Go", value: 60, usage: 0.6, color: chartColors[0]},
		{label: "Python", value: 40, usage: 0.4, color: chartColors[1]},
	}

	out := legend(sectors, 40)
	if out == "" {
		t.Fatalf("legend() returned empty string")
	}

	for _, label := range []string{"Go", "Python"} {
		if !strings.Contains(out, label) {
			t.Errorf("legend() missing label %q", label)
		}
	}
	for _, value := range []string{"60", "40"} {
		if !strings.Contains(out, value) {
			t.Errorf("legend() missing value %q", value)
		}
	}
}

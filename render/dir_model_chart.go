package render

func (dm *DirModel) viewChart() string {
	chartSectors := make([]RawChartSector, 0, len(dm.nav.entry.StatsByLang))
	var totalCode float64
	for lang, stats := range dm.nav.entry.StatsByLang {
		if stats.Total() > 0 {
			chartSectors = append(chartSectors, RawChartSector{
				Label: lang,
				Value: float64(stats.Total()),
			})
			totalCode += float64(stats.Total())
		}
	}

	// Ensure the chart has a reasonable radius
	radius := min(dm.width/4, dm.height/4) - 2

	return chartBoxStyle.Render(
		Chart(
			dm.width/2,  // Chart area width
			dm.height/2, // Chart area height
			radius,
			totalCode,
			chartSectors,
		),
	)
}

package render

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	maxSectors      = 8     // Maximum number of chart sectors to display, the rest will be grouped as "Others"
	chartLabelWidth = 50    // Maximum width of legend labels
	aspectFix       = 2.4   // Terminal characters are usually not square, this value is used to fix the aspect ratio to make the pie chart look rounder
	anglePrecision  = 1e-10 // Precision threshold for angle calculations
)

// chartColors defines the color cycle list used for pie chart sectors.
var chartColors = []lipgloss.Color{
	lipgloss.Color("#ffbe0b"),
	lipgloss.Color("#fb5607"),
	lipgloss.Color("#ff006e"),
	lipgloss.Color("#8338ec"),
	lipgloss.Color("#3a86ff"),
	lipgloss.Color("#00f5d4"),
	lipgloss.Color("#fef9ef"),
	lipgloss.Color("#ff85a1"),
	lipgloss.Color("#b5838d"),
}

// RawChartSector is the raw data input for building the chart.
type RawChartSector struct {
	Label string
	Value float64 // Using float64 to accommodate code line counts
}

// chartSector is the internal chart sector structure containing all information needed for rendering.
type chartSector struct {
	color      lipgloss.Color
	label      string
	value      float64
	usage      float64
	startAngle float64
	endAngle   float64
}

// Chart generates an ASCII pie chart and its legend based on the provided data.
func Chart(width, height, radius int, totalValue float64, raw []RawChartSector) string {
	sb := strings.Builder{}
	sectors := prepareSectors(totalValue, raw)

	// Calculate the pie chart center point
	// Divide width by 2 since the pie chart only takes up half the total width
	centerX, centerY := float64(width/2)/2.0, float64(height)/2.0

	// Iterate through each pixel position to draw the pie chart
	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			dx := float64(x) - centerX
			dy := (float64(y) - centerY) * aspectFix // Apply aspect ratio correction

			dist := math.Sqrt(dx*dx + dy*dy)
			radiusFloat := float64(radius)

			// Print a space if the point is outside the pie chart radius
			if dist > radiusFloat {
				sb.WriteByte(' ')
				continue
			}

			// Calculate point angle using high precision
			angle := math.Atan2(dy, dx)
			if angle < 0 {
				angle += 2 * math.Pi // Normalize angle to [0, 2π]
			}

			// Find the sector this point belongs to and render with its color
			rendered := false
			for _, s := range sectors {
				if isAngleInSector(angle, s.startAngle, s.endAngle) {
					sb.WriteString(
						lipgloss.NewStyle().Foreground(s.color).Render("█"), // Use solid block character
					)
					rendered = true
					break
				}
			}
			if !rendered {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}

	// Join the pie chart and legend horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Center, sb.String(), legend(sectors, width/2),
	)
}

// prepareSectors converts raw data to sectors and calculates their angles
func prepareSectors(totalValue float64, rawSectors []RawChartSector) []chartSector {
	// Sort by value in descending order so the largest sectors appear first
	sort.Slice(rawSectors, func(i, j int) bool {
		return rawSectors[i].Value > rawSectors[j].Value
	})

	sectors := make([]chartSector, 0, len(rawSectors))
	others := chartSector{label: "Others"}

	for i, s := range rawSectors {
		// If total value is 0, usage for all sectors is 0
		usage := 0.0
		if totalValue > 0 {
			usage = s.Value / totalValue
		}

		// Merge items exceeding maximum sector count into "Others"
		if i >= maxSectors {
			others.value += s.Value
			continue
		}

		sectors = append(
			sectors,
			chartSector{
				label: s.Label,
				value: s.Value,
				usage: usage,
			},
		)
	}

	// If "Others" has value, calculate its usage and add to sector list
	if others.value > 0 {
		if totalValue > 0 {
			others.usage = others.value / totalValue
		}
		sectors = append(sectors, others)
	}

	// Sort again to ensure "Others" is in the correct position
	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].value > sectors[j].value
	})

	// Calculate start and end angles for each sector using high precision
	start := 0.0
	for i := range sectors {
		sectors[i].color = chartColors[i%len(chartColors)] // Cycle through colors
		sectors[i].startAngle = start
		// 使用更精确的角度计算
		angleSpan := sectors[i].usage * 2 * math.Pi
		sectors[i].endAngle = start + angleSpan
		start = sectors[i].endAngle
	}

	return sectors
}

// isAngleInSector checks if an angle is within a sector's range
func isAngleInSector(angle, startAngle, endAngle float64) bool {
	// Normalize angles to [0, 2π] range
	for angle < 0 {
		angle += 2 * math.Pi
	}
	for angle >= 2*math.Pi {
		angle -= 2 * math.Pi
	}
	for startAngle < 0 {
		startAngle += 2 * math.Pi
	}
	for startAngle >= 2*math.Pi {
		startAngle -= 2 * math.Pi
	}
	for endAngle < 0 {
		endAngle += 2 * math.Pi
	}
	for endAngle >= 2*math.Pi {
		endAngle -= 2 * math.Pi
	}

	// If sector doesn't cross 0-degree boundary
	if endAngle > startAngle {
		return angle >= startAngle-anglePrecision && angle <= endAngle+anglePrecision
	}

	// If sector crosses 0-degree boundary
	return angle >= startAngle-anglePrecision || angle <= endAngle+anglePrecision
}

func legend(sectors []chartSector, width int) string {
	l := make([]string, 0, len(sectors))
	listPadding := 2 // Left and right padding for the legend

	for i, s := range sectors {
		// Truncate long labels
		label := fmtName(s.label, int(float64(width)*0.5))

		// Format values based on size
		var valueStr string
		if s.value >= 1000 {
			// Large values use integer format
			valueStr = strconv.FormatFloat(s.value, 'f', 0, 64) + " lines"
		} else if s.value >= 1 {
			// Medium values keep 1 decimal place
			valueStr = strconv.FormatFloat(s.value, 'f', 1, 64) + " lines"
		} else {
			// Small values keep 2 decimal places
			valueStr = strconv.FormatFloat(s.value, 'f', 2, 64) + " lines"
		}

		// Calculate padding spaces between label and value for right alignment
		padding := strings.Repeat(
			" ",
			max(width-lipgloss.Width(label)-listPadding*2-lipgloss.Width(valueStr), 0),
		)

		// Construct a legend row
		row := lipgloss.NewStyle().
			Width(width).
			Padding(0, listPadding).
			Render(
				lipgloss.NewStyle().Foreground(s.color).Render("█ ") + // Color block
					label +
					padding +
					valueStr,
			)

		l = append(l, row)
		// Don't add blank line after the last row
		if i < len(sectors)-1 {
			l = append(l, "")
		}
	}

	// Join all lines vertically
	return lipgloss.JoinVertical(lipgloss.Left, l...)
}

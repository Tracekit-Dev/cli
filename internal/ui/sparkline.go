package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSparkline renders a braille sparkline chart from float64 data.
// width is in terminal columns, height is in terminal rows.
// Each braille character is 2 dots wide and 4 dots tall,
// so effective resolution is width*2 x height*4.
func RenderSparkline(data []float64, width, height int, color lipgloss.Color) string {
	if len(data) == 0 || width == 0 || height == 0 {
		return ""
	}

	dotH := height * 4 // braille has 4 rows per character
	dotW := width * 2  // braille has 2 cols per character

	// Resample data to fit dotW points
	resampled := resample(data, dotW)

	// Find min/max for scaling
	minVal, maxVal := resampled[0], resampled[0]
	for _, v := range resampled {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	// Create dot grid (row 0 = top)
	grid := make([][]bool, dotH)
	for r := range grid {
		grid[r] = make([]bool, dotW)
	}

	// Plot data points as filled area (from bottom up to value)
	for x := 0; x < dotW && x < len(resampled); x++ {
		normalized := (resampled[x] - minVal) / valRange
		dotRow := int(math.Round(normalized * float64(dotH-1)))
		// Fill from bottom up to the value
		for r := dotH - 1; r >= dotH-1-dotRow; r-- {
			if r >= 0 && r < dotH {
				grid[r][x] = true
			}
		}
	}

	// Render grid to braille characters
	style := lipgloss.NewStyle().Foreground(color)
	var lines []string

	for row := 0; row < dotH; row += 4 {
		var line strings.Builder
		for col := 0; col < dotW; col += 2 {
			ch := brailleChar(grid, row, col, dotH, dotW)
			line.WriteRune(ch)
		}
		lines = append(lines, style.Render(line.String()))
	}

	return strings.Join(lines, "\n")
}

// RenderSparklineLine renders a braille sparkline as a line (not filled area).
func RenderSparklineLine(data []float64, width, height int, color lipgloss.Color) string {
	if len(data) == 0 || width == 0 || height == 0 {
		return ""
	}

	dotH := height * 4
	dotW := width * 2

	resampled := resample(data, dotW)

	minVal, maxVal := resampled[0], resampled[0]
	for _, v := range resampled {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	grid := make([][]bool, dotH)
	for r := range grid {
		grid[r] = make([]bool, dotW)
	}

	// Plot line connecting points
	for x := 0; x < dotW && x < len(resampled); x++ {
		normalized := (resampled[x] - minVal) / valRange
		dotRow := dotH - 1 - int(math.Round(normalized*float64(dotH-1)))
		if dotRow >= 0 && dotRow < dotH {
			grid[dotRow][x] = true
		}
		// Connect to next point
		if x+1 < len(resampled) {
			nextNorm := (resampled[x+1] - minVal) / valRange
			nextRow := dotH - 1 - int(math.Round(nextNorm*float64(dotH-1)))
			// Draw vertical connection
			start, end := dotRow, nextRow
			if start > end {
				start, end = end, start
			}
			for r := start; r <= end; r++ {
				if r >= 0 && r < dotH {
					grid[r][x] = true
				}
			}
		}
	}

	style := lipgloss.NewStyle().Foreground(color)
	var lines []string
	for row := 0; row < dotH; row += 4 {
		var line strings.Builder
		for col := 0; col < dotW; col += 2 {
			ch := brailleChar(grid, row, col, dotH, dotW)
			line.WriteRune(ch)
		}
		lines = append(lines, style.Render(line.String()))
	}

	return strings.Join(lines, "\n")
}

// brailleChar encodes a 2x4 cell from the grid into a braille character.
// Braille dot positions:
//
//	(0,0) (1,0)    dots 1, 4
//	(0,1) (1,1)    dots 2, 5
//	(0,2) (1,2)    dots 3, 6
//	(0,3) (1,3)    dots 7, 8
func brailleChar(grid [][]bool, row, col, maxRow, maxCol int) rune {
	var code rune
	dots := [8][2]int{
		{0, 0}, {0, 1}, {0, 2}, // left column: dots 1,2,3
		{1, 0}, {1, 1}, {1, 2}, // right column: dots 4,5,6
		{0, 3}, {1, 3}, // bottom: dots 7,8
	}
	for i, d := range dots {
		r := row + d[1]
		c := col + d[0]
		if r < maxRow && c < maxCol && grid[r][c] {
			code |= 1 << uint(i)
		}
	}
	return '\u2800' + code
}

// resample resamples data to target number of points using linear interpolation.
func resample(data []float64, target int) []float64 {
	if len(data) == 0 {
		return nil
	}
	if len(data) == target {
		return data
	}

	result := make([]float64, target)
	ratio := float64(len(data)-1) / float64(target-1)

	for i := 0; i < target; i++ {
		pos := float64(i) * ratio
		lower := int(math.Floor(pos))
		upper := int(math.Ceil(pos))
		if upper >= len(data) {
			upper = len(data) - 1
		}
		if lower == upper {
			result[i] = data[lower]
		} else {
			frac := pos - float64(lower)
			result[i] = data[lower]*(1-frac) + data[upper]*frac
		}
	}
	return result
}

package admin

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"sort"
	"time"

	"siaga-api/api/models/responses"

	"github.com/jung-kurt/gofpdf"
	zero "github.com/rs/zerolog/log"
)

// GenerateAttendanceReportPDF builds a concise executive PDF report for the
// attendance dashboard. This implementation intentionally avoids chart images
// and instead uses structured tables that are robust in container environments.
func (s *Service) GenerateAttendanceReportPDF(ctx context.Context, startDate, endDate time.Time) ([]byte, error) {
	data, err := s.buildAttendanceReportData(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}
	statusView := computeOverallStatus(data.Summary)
	insights := buildKeyInsights(data)

	discChartData := buildDisciplineChartData(data)

	trendPNG, err := renderTrendChartPNG(data)
	if err != nil {
		zero.Error().Stack().
			Str("Context", "GenerateAttendanceReportPDF").
			Str("Stage", "render trend chart").
			Err(err).Msg("")
		trendPNG = nil
	}
	breakdownPNG, err := renderBreakdownChartPNG(discChartData)
	if err != nil {
		zero.Error().Stack().
			Str("Context", "GenerateAttendanceReportPDF").
			Str("Stage", "render breakdown chart").
			Err(err).Msg("")
		breakdownPNG = nil
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("SIAGA Attendance Executive Report", false)
	pdf.SetAuthor("SIAGA", false)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Title and metadata
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 10, "SIAGA Attendance Executive Report", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(107, 114, 128) // gray
	// Use the raw start/end dates passed into the service to reflect exactly
	// what the admin selected in the UI, independent of any internal
	// normalization.
	pdf.CellFormat(0, 5, formatDateRangeWithEmDash(startDate, endDate), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated at: %s",
		data.Summary.GeneratedAt.Format("2006-01-02 15:04"),
	), "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	pdf.Ln(6)

	// Overall status badge
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetFillColor(statusView.Color.R, statusView.Color.G, statusView.Color.B)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(40, 6, fmt.Sprintf("Status: %s", statusView.Label), "", 1, "C", true, 0, "")
	pdf.SetTextColor(0, 0, 0)

	// Executive summary section
	addSectionDivider(pdf)
	addSectionHeader(pdf, "Executive Summary")
	renderSummaryCards(pdf, data.Summary)

	// Key insights
	addSectionDivider(pdf)
	addSectionHeader(pdf, "Key Insights")
	pdf.SetFont("Helvetica", "", 10)
	if len(insights) == 0 {
		pdf.CellFormat(0, 5, "Attendance trend data is limited for the selected period.", "", 1, "L", false, 0, "")
	} else {
		for _, line := range insights {
			pdf.MultiCell(0, 5, "- "+line, "", "L", false)
		}
	}

	pdf.Ln(6)

	// Trend section (fallback: table instead of chart)
	addSectionDivider(pdf)
	addSectionHeader(pdf, "Attendance Trend")
	// Render chart image (safe; falls back to table if invalid)
	if imgOpts, err := registerPNGImage(pdf, "trend.png", trendPNG); err == nil && imgOpts != nil {
		pageWidth, _ := pdf.GetPageSize()
		left, _, right, _ := pdf.GetMargins()
		contentWidth := pageWidth - left - right
		width := contentWidth
		height := width * imgOpts.Height() / imgOpts.Width()
		x := left
		y := pdf.GetY() + 4
		pdf.ImageOptions("trend.png", x, y, width, height, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		pdf.SetY(y + height + 4)

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(107, 114, 128)
		pdf.CellFormat(0, 5, "Daily attendance and discipline trend for the selected period.", "", 1, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	} else if err != nil {
		zero.Error().Stack().
			Str("Context", "GenerateAttendanceReportPDF").
			Str("Stage", "embed trend chart").
			Err(err).Msg("")
	}

	// Compact attendance trend table (same concept as Excel "Trend (Daily)" sheet).
	pdf.Ln(4)
	renderTrendTable(pdf, data.Trend)
	// Decide if we need a new page before Discipline Breakdown based on remaining space.
	{
		_, pageHeight := pdf.GetPageSize()
		_, _, _, _ = pdf.GetMargins()
		autoBreak, bottomMargin := pdf.GetAutoPageBreak()
		if !autoBreak {
			// Fallback margin when auto-break is disabled.
			bottomMargin = 10
		}
		bottomLimit := pageHeight - bottomMargin
		// Approximate required height for donut + legend + spacing (in mm).
		required := 90.0
		if pdf.GetY()+required > bottomLimit {
			pdf.AddPage()
		}
	}

	// Discipline breakdown section
	addSectionDivider(pdf)
	addSectionHeader(pdf, "Discipline Breakdown")
	if imgOpts, err := registerPNGImage(pdf, "breakdown.png", breakdownPNG); err == nil && imgOpts != nil {
		pageWidth, _ := pdf.GetPageSize()
		left, _, right, _ := pdf.GetMargins()
		contentWidth := pageWidth - left - right

		// Display donut chart at a fixed size so it doesn't dominate the page.
		width := 70.0
		height := width * imgOpts.Height() / imgOpts.Width()
		x := left + (contentWidth-width)/2
		y := pdf.GetY() + 4
		pdf.ImageOptions("breakdown.png", x, y, width, height, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")

		// Sharp center text ("Total" and number) rendered via GoFPDF.
		if discChartData != nil && discChartData.Total > 0 {
			// cx := x + width/2
			cy := y + height/2

			pdf.SetTextColor(107, 114, 128)
			pdf.SetFont("Helvetica", "", 11)
			pdf.SetXY(x, cy-6)
			pdf.CellFormat(width, 4, "Total", "", 0, "C", false, 0, "")

			pdf.SetTextColor(17, 24, 39)
			pdf.SetFont("Helvetica", "B", 15)
			pdf.SetXY(x, cy+1)
			pdf.CellFormat(width, 6, fmt.Sprintf("%d", discChartData.Total), "", 0, "C", false, 0, "")

			pdf.SetTextColor(0, 0, 0)
		}

		// Legend / summary below chart.
		pdf.SetY(y + height + 6)
		pdf.SetX(left)
		legendItems, boosted := buildDisciplineLegendItems(discChartData)
		renderDisciplineLegend(pdf, legendItems, boosted)
		// Keep some space before the attendance table, but not too much so
		// rows can still use the remaining vertical area on this page.
		pdf.Ln(6)
	} else if err != nil {
		zero.Error().Stack().
			Str("Context", "GenerateAttendanceReportPDF").
			Str("Stage", "embed breakdown chart").
			Err(err).Msg("")
	}

	// Attendance list section (top N rows to keep within 1–2 pages).
	addSectionHeader(pdf, "Attendance List (Top by Risk)")
	const maxUsers = 15
	renderAttendanceListTable(pdf, data.UserRows, maxUsers)

	// Footer on each page
	pdf.SetAutoPageBreak(false, 10)
	pageCount := pdf.PageNo()
	for i := 1; i <= pageCount; i++ {
		pdf.SetPage(i)
		pdf.SetY(287)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(0, 5, "Generated by SIAGA", "", 0, "R", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		zero.Error().Stack().
			Str("Context", "GenerateAttendanceReportPDF").
			Str("Stage", "output pdf").
			Err(err).Msg("")
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

func addSectionHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, 7, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
}

// renderSimpleTable renders a small table with header row and light styling.
func renderSimpleTable(pdf *gofpdf.Fpdf, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	colCount := len(headers)
	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	tableWidth := pageWidth - left - right
	colWidth := tableWidth / float64(colCount)

	// Header
	pdf.SetFillColor(229, 231, 235) // light gray
	pdf.SetDrawColor(209, 213, 219)
	pdf.SetLineWidth(0.2)
	pdf.SetFont("Helvetica", "B", 9)
	for _, h := range headers {
		pdf.CellFormat(colWidth, 6, h, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	// Rows
	pdf.SetFont("Helvetica", "", 9)
	fill := false
	for _, row := range rows {
		if len(row) != colCount {
			continue
		}
		if fill {
			pdf.SetFillColor(249, 250, 251) // even row background
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for _, cell := range row {
			pdf.CellFormat(colWidth, 6, cell, "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}
}

// renderTrendTable renders a small daily attendance trend table underneath
// the trend chart. It mirrors the Excel "Trend (Daily)" sheet structure.
func renderTrendTable(pdf *gofpdf.Fpdf, trend []attendanceReportTrendRow) {
	if len(trend) == 0 {
		return
	}

	headers := []string{"Date", "Present", "Late", "Absent", "Upcoming"}
	rows := make([][]string, 0, len(trend))
	for _, t := range trend {
		rows = append(rows, []string{
			t.Date.Format("2006-01-02"),
			fmt.Sprintf("%d", t.Present),
			fmt.Sprintf("%d", t.Late),
			fmt.Sprintf("%d", t.Absent),
			fmt.Sprintf("%d", t.NotYet),
		})
	}

	renderSimpleTable(pdf, headers, rows)
}

// tableCol describes a column in a PDF table.
type tableCol struct {
	Width float64
	Align string
}

func calcRowHeight(pdf *gofpdf.Fpdf, cols []tableCol, values []string) float64 {
	if len(cols) == 0 || len(cols) != len(values) {
		return 0
	}

	lineHeight := 5.0
	padding := 2.0

	maxLines := 1
	for i, col := range cols {
		text := values[i]
		if text == "" {
			continue
		}
		lines := pdf.SplitLines([]byte(text), col.Width-2*padding)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	return float64(maxLines)*lineHeight + 2*padding
}

// drawTableRow draws a single table row with proper wrapping and row height
// calculation so that cells never overlap or concatenate visually.
func drawTableRow(pdf *gofpdf.Fpdf, cols []tableCol, values []string, fill bool) {
	if len(cols) == 0 || len(cols) != len(values) {
		return
	}

	lineHeight := 5.0
	padding := 2.0

	startX := pdf.GetX()
	startY := pdf.GetY()

	rowHeight := calcRowHeight(pdf, cols, values)
	if rowHeight == 0 {
		return
	}

	// Background for the whole row.
	totalWidth := 0.0
	for _, col := range cols {
		totalWidth += col.Width
	}
	if fill {
		pdf.SetFillColor(249, 250, 251)
	} else {
		pdf.SetFillColor(255, 255, 255)
	}
	pdf.Rect(startX, startY, totalWidth, rowHeight, "F")

	// Cell borders and text.
	x := startX
	for i, col := range cols {
		text := values[i]

		// Border
		pdf.Rect(x, startY, col.Width, rowHeight, "D")

		// Text
		pdf.SetXY(x+padding, startY+padding)
		pdf.MultiCell(col.Width-2*padding, lineHeight, text, "", col.Align, false)

		x += col.Width
		pdf.SetXY(x, startY)
	}

	pdf.SetXY(startX, startY+rowHeight)
}

// renderAttendanceListTable renders the top N risk employees in a clean table.
func renderAttendanceListTable(pdf *gofpdf.Fpdf, rows []attendanceReportUserRow, maxRows int) {
	left, _, _, _ := pdf.GetMargins()
	_, pageHeight := pdf.GetPageSize()
	// Reserve a bit of space above the footer (which is drawn at ~pageHeight-10)
	footerMargin := 10.0
	contentBottom := pageHeight - footerMargin - 2.0

	// Temporarily disable automatic page breaks while we manage pagination.
	autoBreak, autoMargin := pdf.GetAutoPageBreak()
	pdf.SetAutoPageBreak(false, autoMargin)
	defer pdf.SetAutoPageBreak(autoBreak, autoMargin)

	pdf.SetX(left)

	cols := []tableCol{
		{Width: 55, Align: "L"}, // Name
		{Width: 30, Align: "L"}, // Position
		{Width: 15, Align: "C"}, // Present
		{Width: 15, Align: "C"}, // Absent
		{Width: 15, Align: "C"}, // Late
		{Width: 25, Align: "C"}, // Avg late
		{Width: 25, Align: "L"}, // Risk
	}

	headers := []string{"Name", "Position", "Present", "Absent", "Late", "Avg late (min)", "Risk"}

	drawHeader := func() {
		pdf.SetX(left)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetFillColor(229, 231, 235)
		pdf.SetDrawColor(209, 213, 219)
		drawTableRow(pdf, cols, headers, true)
	}

	// Header row on first page.
	drawHeader()

	// Data rows
	pdf.SetFont("Helvetica", "", 10)
	fill := false

	limit := len(rows)
	if maxRows > 0 && limit > maxRows {
		limit = maxRows
	}

	if limit == 0 {
		drawTableRow(pdf, cols, []string{"No data", "", "", "", "", "", ""}, false)
		return
	}

	for i := 0; i < limit; i++ {
		u := rows[i]
		values := []string{
			u.Name,
			u.Position,
			fmt.Sprintf("%d", u.PresentCount),
			fmt.Sprintf("%d", u.AbsentCount),
			fmt.Sprintf("%d", u.LateCount),
			fmt.Sprintf("%.1f", u.AvgLateMinutes),
			fmt.Sprintf("%s (%.0f)", u.RiskLevel, u.RiskScore),
		}

		// Manual pagination: move to next page if row doesn't fit.
		rowHeight := calcRowHeight(pdf, cols, values)
		if rowHeight == 0 {
			continue
		}
		if pdf.GetY()+rowHeight > contentBottom {
			pdf.AddPage()
			// Redraw header on new page.
			drawHeader()
			// Reset font to normal weight for data rows.
			pdf.SetFont("Helvetica", "", 10)
		}

		drawTableRow(pdf, cols, values, fill)
		fill = !fill
	}
}

// renderSummaryCards renders executive summary metrics as 2x2 cards.
func renderSummaryCards(pdf *gofpdf.Fpdf, summary attendanceReportSummary) {
	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	contentWidth := pageWidth - left - right

	gap := 6.0
	cardWidth := (contentWidth - gap) / 2.0
	cardHeight := 18.0

	yStart := pdf.GetY() + 4

	metrics := []struct {
		Label string
		Value string
	}{
		{"Attendance Rate", fmt.Sprintf("%.1f%%", summary.AttendanceRate)},
		{"On-Time Rate", fmt.Sprintf("%.1f%%", summary.OnTimeRate)},
		{"Absent Rate", fmt.Sprintf("%.1f%%", summary.AbsentRate)},
		{"Avg Late Minutes", fmt.Sprintf("%.1f", summary.AvgLateMinutes)},
	}

	for i, m := range metrics {
		row := i / 2
		col := i % 2

		x := left + float64(col)*(cardWidth+gap)
		y := yStart + float64(row)*(cardHeight+4)

		// Card background
		pdf.SetFillColor(249, 250, 251)
		pdf.SetDrawColor(229, 231, 235)
		pdf.Rect(x, y, cardWidth, cardHeight, "DF")

		// Label
		pdf.SetXY(x+3, y+3)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(107, 114, 128)
		pdf.CellFormat(cardWidth-6, 5, m.Label, "", 2, "L", false, 0, "")

		// Value
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetTextColor(17, 24, 39)
		pdf.CellFormat(cardWidth-6, 7, m.Value, "", 0, "L", false, 0, "")
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(yStart + 2*cardHeight + 6)
}

// addSectionDivider draws a subtle divider between sections.
func addSectionDivider(pdf *gofpdf.Fpdf) {
	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()

	pdf.Ln(6)
	y := pdf.GetY()
	pdf.SetDrawColor(229, 231, 235)
	pdf.SetLineWidth(0.2)
	pdf.Line(left, y, pageWidth-right, y)
	pdf.Ln(4)
}

type legendColor struct {
	R, G, B int
}

type disciplineLegendItem struct {
	Label   string
	Count   int
	Percent float64
	Color   legendColor
	Boosted bool
}

// buildDisciplineLegendItems builds legend items from the shared discipline
// chart data, using the same color palette and preserving true percentages.
func buildDisciplineLegendItems(chartData *disciplineChartData) ([]disciplineLegendItem, bool) {
	if chartData == nil || len(chartData.Segments) == 0 {
		return nil, false
	}

	items := make([]disciplineLegendItem, 0, len(chartData.Segments))
	for _, seg := range chartData.Segments {
		items = append(items, disciplineLegendItem{
			Label:   seg.Label,
			Count:   seg.Count,
			Percent: seg.Percent,
			Color: legendColor{
				R: seg.Color.R,
				G: seg.Color.G,
				B: seg.Color.B,
			},
			Boosted: seg.Boosted,
		})
	}

	// Sort by count descending for executive readability.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Label < items[j].Label
		}
		return items[i].Count > items[j].Count
	})

	// Determine if any slice was visually boosted.
	boosted := false
	for _, it := range items {
		if it.Boosted {
			boosted = true
			break
		}
	}

	return items, boosted
}

// renderDisciplineLegend renders a compact, color-coded legend below the donut
// chart. It automatically wraps into two columns if there are many items.
func renderDisciplineLegend(pdf *gofpdf.Fpdf, items []disciplineLegendItem, hasBoosted bool) {
	if len(items) == 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 5, "No discipline incidents in the selected period.", "", 1, "L", false, 0, "")
		return
	}

	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	contentWidth := pageWidth - left - right

	colCount := 1
	if len(items) > 3 {
		colCount = 2
	}
	colWidth := contentWidth / float64(colCount)
	rowHeight := 6.0
	squareSize := 4.0

	startY := pdf.GetY()

	pdf.SetFont("Helvetica", "", 10)
	for i, item := range items {
		row := i / colCount
		col := i % colCount

		x := left + float64(col)*colWidth
		y := startY + float64(row)*rowHeight

		// Color square
		pdf.SetFillColor(item.Color.R, item.Color.G, item.Color.B)
		pdf.Rect(x, y+1, squareSize, squareSize, "F")

		// Text
		pdf.SetXY(x+squareSize+4, y)
		text := fmt.Sprintf("%s: %d (%.1f%%)", item.Label, item.Count, item.Percent)
		pdf.CellFormat(colWidth-6, rowHeight, text, "", 0, "L", false, 0, "")
	}

	rows := (len(items) + colCount - 1) / colCount
	pdf.SetY(startY + float64(rows)*rowHeight + 2)

	if hasBoosted {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(107, 114, 128)
		pdf.CellFormat(0, 4, "* Minor categories may use a minimum slice size for visibility.", "", 5, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
}

// registerPNGImage safely validates and registers a PNG image for use in the PDF.
// If the data is empty or invalid, it returns an error so the caller can fall back
// to a non-chart representation.
func registerPNGImage(pdf *gofpdf.Fpdf, name string, data []byte) (*gofpdf.ImageInfoType, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty png data for %s", name)
	}

	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("invalid png for %s: %w", name, err)
	}

	info := pdf.RegisterImageOptionsReader(name, gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
	if info == nil {
		return nil, fmt.Errorf("failed to register image %s", name)
	}

	return info, nil
}

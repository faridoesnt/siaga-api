package admin

import (
	"bytes"
	// "fmt"
	"time"

	"siaga-api/api/models/responses"

	chart "github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// Discipline chart palette and data model shared between PNG rendering and PDF legend.

type disciplineCategory string

const (
	disciplineLate        disciplineCategory = "late"
	disciplineEarlyLeave  disciplineCategory = "early_leave"
	disciplineNoCheckin   disciplineCategory = "no_checkin"
	disciplineMissedShift disciplineCategory = "missed_shift"
	disciplineUpcoming    disciplineCategory = "upcoming_shift"
)

type disciplineColor struct {
	R, G, B int
}

func (c disciplineColor) toDrawingColor() drawing.Color {
	return drawing.Color{R: uint8(c.R), G: uint8(c.G), B: uint8(c.B), A: 255}
}

var disciplinePalette = map[disciplineCategory]disciplineColor{
	disciplineLate:        {R: 37, G: 99, B: 235},  // #2563EB
	disciplineEarlyLeave:  {R: 34, G: 197, B: 94},  // #22C55E
	disciplineNoCheckin:   {R: 100, G: 116, B: 139}, // #64748B
	disciplineMissedShift: {R: 236, G: 72, B: 153}, // #EC4899
	disciplineUpcoming:    {R: 6, G: 182, B: 212},  // #06B6D4
}

type disciplineChartSegment struct {
	Key          disciplineCategory
	Label        string
	Count        int
	Percent      float64
	DrawFraction float64
	Color        disciplineColor
	Boosted      bool
}

type disciplineChartData struct {
	Segments   []disciplineChartSegment
	Total      int
	HasBoosted bool
}

// buildDisciplineChartData prepares normalized chart segments with a minimum
// visible slice rule for tiny categories (visual only).
func buildDisciplineChartData(data *attendanceReportData) *disciplineChartData {
	if data == nil || data.Breakdown == nil {
		return &disciplineChartData{}
	}

	b := data.Breakdown
	total := b.Late + b.EarlyLeave + b.NoCheckin + b.MissedShift + b.FutureShift
	if total == 0 {
		return &disciplineChartData{}
	}

	// Minimum visible slice rule expressed as an angle in degrees.
	const minAngleDeg = 2.5
	minVisiblePercent := (minAngleDeg / 360.0) * 100.0

	raw := []struct {
		key   disciplineCategory
		label string
		count int
	}{
		{disciplineLate, "Late", b.Late},
		{disciplineEarlyLeave, "Early leave", b.EarlyLeave},
		{disciplineNoCheckin, "No check-in", b.NoCheckin},
		{disciplineMissedShift, "Missed shift", b.MissedShift},
		{disciplineUpcoming, "Upcoming shifts", b.FutureShift},
	}

	out := &disciplineChartData{Total: total}

	// First pass: compute percents and boosted percents.
	type tempSeg struct {
		disciplineChartSegment
		boostedPercent float64
	}
	var temp []tempSeg

	for _, r := range raw {
		if r.count <= 0 {
			continue
		}
		pct := float64(r.count) / float64(total) * 100
		boostedPct := pct
		boosted := false
		if pct > 0 && pct < minVisiblePercent {
			boostedPct = minVisiblePercent
			boosted = true
			out.HasBoosted = true
		}

		color := disciplinePalette[r.key]

		temp = append(temp, tempSeg{
			disciplineChartSegment: disciplineChartSegment{
				Key:     r.key,
				Label:   r.label,
				Count:   r.count,
				Percent: pct,
				Color:   color,
				Boosted: boosted,
			},
			boostedPercent: boostedPct,
		})
	}

	if len(temp) == 0 {
		return &disciplineChartData{}
	}

	// Normalize boosted percents into draw fractions that sum to 1.0.
	var boostedSum float64
	for _, s := range temp {
		boostedSum += s.boostedPercent
	}
	if boostedSum <= 0 {
		return &disciplineChartData{}
	}

	for _, s := range temp {
		s.DrawFraction = s.boostedPercent / boostedSum
		out.Segments = append(out.Segments, s.disciplineChartSegment)
	}

	return out
}

// renderTrendChartPNG renders the attendance trend as a PNG image.
func renderTrendChartPNG(data *attendanceReportData) ([]byte, error) {
	xValues := []time.Time{}
	present := []float64{}
	late := []float64{}
	absent := []float64{}
	upcoming := []float64{}

	for _, t := range data.Trend {
		xValues = append(xValues, t.Date)
		present = append(present, float64(t.Present))
		late = append(late, float64(t.Late))
		absent = append(absent, float64(t.Absent))
		upcoming = append(upcoming, float64(t.NotYet))
	}

	// go-chart requires at least 2 different x-values; otherwise it fails with
	// "zero x-range delta". This happens when the selected date range only
	// contains a single day. To keep the chart rendering stable we:
	//   - if there is no data at all, synthesize a single zero point at "now"
	//   - if there is exactly one point, clone it at +24h so the x-range > 0
	switch len(xValues) {
	case 0:
		now := time.Now()
		xValues = append(xValues, now)
		present = append(present, 0)
		late = append(late, 0)
		absent = append(absent, 0)
		upcoming = append(upcoming, 0)
	case 1:
		cloneTime := xValues[0].Add(24 * time.Hour)
		xValues = append(xValues, cloneTime)
		present = append(present, present[0])
		late = append(late, late[0])
		absent = append(absent, absent[0])
		upcoming = append(upcoming, upcoming[0])
	}

	graph := chart.Chart{
		Width:  900,
		Height: 260,
		Background: chart.Style{
			FillColor: chart.ColorWhite,
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("2006-01-02"),
		},
		YAxis: chart.YAxis{
			Name: "Count",
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Present",
				XValues: xValues,
				YValues: present,
				Style: chart.Style{
					StrokeColor: chart.ColorGreen,
				},
			},
			chart.TimeSeries{
				Name:    "Late",
				XValues: xValues,
				YValues: late,
				Style: chart.Style{
					StrokeColor: chart.ColorOrange,
				},
			},
			chart.TimeSeries{
				Name:    "Absent",
				XValues: xValues,
				YValues: absent,
				Style: chart.Style{
					StrokeColor: chart.ColorRed,
				},
			},
			chart.TimeSeries{
				Name:    "Upcoming",
				XValues: xValues,
				YValues: upcoming,
				Style: chart.Style{
					StrokeColor: chart.ColorBlue,
				},
			},
		},
	}

	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	var buf bytes.Buffer
	if err := graph.Render(chart.PNG, &buf); err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

// renderBreakdownChartPNG renders a donut-style discipline breakdown as PNG.
func renderBreakdownChartPNG(chartData *disciplineChartData) ([]byte, error) {
	values := []chart.Value{}

	if chartData == nil || len(chartData.Segments) == 0 {
		values = append(values, chart.Value{
			Value: 1,
			Label: "",
			Style: chart.Style{
				FillColor:   drawing.Color{R: 229, G: 231, B: 235, A: 255},
				StrokeColor: chart.ColorWhite,
				StrokeWidth: 2.0,
			},
		})
	} else {
		for _, seg := range chartData.Segments {
			values = append(values, chart.Value{
				Value: seg.DrawFraction,
				Label: "",
				Style: chart.Style{
					FillColor:   seg.Color.toDrawingColor(),
					StrokeColor: chart.ColorWhite,
					StrokeWidth: 2.0,
				},
			})
		}
	}

	// Render at high resolution to avoid blur when downscaled in PDF.
	const baseSize = 900
	pie := chart.PieChart{
		Width:  baseSize,
		Height: baseSize,
		Values: values,
		Background: chart.Style{
			FillColor: chart.ColorWhite,
		},
	}

	// Add a donut hole and subtle outer stroke via a custom element.
	// Center text is rendered in the PDF layer (GoFPDF) for sharpness.
	pie.Elements = []chart.Renderable{
		func(r chart.Renderer, cb chart.Box, s chart.Style) {
			cx, cy := cb.Center()
			diameter := chart.MinInt(cb.Width(), cb.Height())
			radius := float64(diameter >> 1)

			innerRadius := radius * 0.65

			// Draw inner white circle to create donut.
			r.SetFillColor(chart.ColorWhite)
			r.SetStrokeColor(chart.ColorWhite)
			r.SetStrokeWidth(0)
			r.MoveTo(cx, cy)
			r.ArcTo(cx, cy, innerRadius, innerRadius, chart.DegreesToRadians(0), chart.DegreesToRadians(359))
			r.LineTo(cx, cy)
			r.Close()
			r.Fill()

			// Subtle outer ring stroke for sharper donut edge.
			r.SetStrokeColor(drawing.Color{R: 229, G: 231, B: 235, A: 255})
			r.SetStrokeWidth(1.0)
			r.MoveTo(cx, cy)
			r.ArcTo(cx, cy, radius*0.98, radius*0.98, chart.DegreesToRadians(0), chart.DegreesToRadians(359))
			r.Stroke()
		},
	}

	var buf bytes.Buffer
	if err := pie.Render(chart.PNG, &buf); err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

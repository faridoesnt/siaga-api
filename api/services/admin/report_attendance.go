package admin

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"siaga-api/api/entities"
	"siaga-api/api/models/responses"

	zero "github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

type attendanceReportSummary struct {
	AttendanceRate  float64
	OnTimeRate      float64
	AbsentRate      float64
	AvgLateMinutes  float64
	DateFrom        time.Time
	DateTo          time.Time
	GeneratedAt     time.Time
}

type attendanceReportTrendRow struct {
	Date      time.Time
	Scheduled int
	Present   int
	Late      int
	Absent    int
	NotYet    int
}

type attendanceReportUserRow struct {
	UserID          int64
	Name            string
	Position        string
	PresentCount    int
	AbsentCount     int
	LateCount       int
	AvgLateMinutes  float64
	RiskScore       float64
	RiskLevel       string
}

type attendanceReportData struct {
	Summary    attendanceReportSummary
	Trend      []attendanceReportTrendRow
	Breakdown  *entities.AdminDashboardDisciplineRow
	UserRows   []attendanceReportUserRow
}

type attendanceUserStats struct {
	UserID          int64
	Name            string
	Position        string
	Present         int
	Absent          int
	LateCount       int
	TotalLateMinute int
	// HasData menandai apakah user ini pernah memiliki jadwal/attendance
	// pada rentang tanggal laporan. Jika tidak, user tidak akan muncul
	// di Attendance List.
	HasData bool
}

// buildAttendanceReportData prepares the data model used by both XLSX and PDF
// generators. It reuses the same queries used by the dashboard and attendance
// monitoring export, without changing underlying business rules.
func (s *Service) buildAttendanceReportData(ctx context.Context, startDate, endDate time.Time) (*attendanceReportData, error) {
	if endDate.Before(startDate) {
		return nil, responses.BadRequest(fmt.Errorf("date_to must be on or after date_from"))
	}

	// Normalize to date-only boundaries in Asia/Jakarta, keeping the same
	// calendar days that the caller provided, karena seluruh reporting
	// dan dashboard dikomunikasikan dalam WIB.
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, loc)
	zero.Info().
		Str("Context", "buildAttendanceReportData").
		Time("param_start", startDate).
		Time("param_end", endDate).
		Time("normalized_start", start).
		Time("normalized_end", end).
		Msg("attendance report date range")

	// Executive summary (reuse dashboard summary logic).
	summaryRow, err := s.repo.GetDashboardSummary(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	var summary attendanceReportSummary
	summary.DateFrom = start
	summary.DateTo = end
	summary.GeneratedAt = time.Now()

	pastScheduled := summaryRow.PastScheduledDays
	pastPresent := summaryRow.PastPresentDays
	pastOnTime := summaryRow.PastOnTimeDays
	if pastScheduled > 0 {
		summary.AttendanceRate = float64(pastPresent) / float64(pastScheduled) * 100
		summary.AbsentRate = float64(pastScheduled-pastPresent) / float64(pastScheduled) * 100
	}
	if pastPresent > 0 {
		summary.OnTimeRate = float64(pastOnTime) / float64(pastPresent) * 100
	}
	if summaryRow.LateRecords > 0 {
		summary.AvgLateMinutes = summaryRow.TotalLateMin / float64(summaryRow.LateRecords)
	}

	// Trend series.
	trendRows, err := s.repo.GetDashboardTrend(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	var trend []attendanceReportTrendRow
	var trendMin, trendMax *time.Time
	for _, r := range trendRows {
		dayDate, err := time.ParseInLocation("2006-01-02", r.Date, loc)
		if err != nil {
			if t2, err2 := time.Parse(time.RFC3339, r.Date); err2 == nil {
				dayDate = t2.In(loc)
			} else {
				// Fallback defensively to the report's start date if parsing fails.
				dayDate = start
			}
		}
		// Normalize any time-of-day component to midnight local time so that
		// comparisons and grouping are purely date-based.
		dayDate = time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), 0, 0, 0, 0, loc)

		// Defensive guard: ensure we never include days outside the requested
		// range even if the underlying query or data has anomalies.
		if dayDate.Before(start) || dayDate.After(end) {
			continue
		}

		if trendMin == nil || dayDate.Before(*trendMin) {
			tmp := dayDate
			trendMin = &tmp
		}
		if trendMax == nil || dayDate.After(*trendMax) {
			tmp := dayDate
			trendMax = &tmp
		}

		row := attendanceReportTrendRow{
			Date:      dayDate,
			Scheduled: r.Scheduled,
			Present:   r.Present,
			Late:      r.Late,
		}
		// Gunakan flag is_future dari DB (berbasis CURDATE() / WIB) agar
		// klasifikasi "Upcoming" konsisten dengan dashboard utama.
		if r.IsFuture {
			row.Absent = 0
			row.NotYet = r.Scheduled
		} else {
			absent := r.Scheduled - r.Present
			if absent < 0 {
				absent = 0
			}
			row.Absent = absent
			row.NotYet = 0
		}
		trend = append(trend, row)
	}

	zero.Info().
		Str("Context", "buildAttendanceReportData").
		Int("trend_count", len(trend)).
		Time("trend_min", func() time.Time {
			if trendMin != nil {
				return *trendMin
			}
			return time.Time{}
		}()).
		Time("trend_max", func() time.Time {
			if trendMax != nil {
				return *trendMax
			}
			return time.Time{}
		}()).
		Msg("attendance report trend bounds")

	// Discipline breakdown.
	discRow, err := s.repo.GetDashboardDiscipline(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	// Risk rows (per-user risk metrics) used for risk levels.
	riskRows, err := s.repo.GetDashboardRiskEmployees(ctx, start, end, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	riskByUser := make(map[int64]*entities.AdminDashboardRiskRow)
	for _, r := range riskRows {
		riskByUser[r.UserID] = r
	}

	// Aggregated attendance stats per user.
	statsByUser, err := s.buildAttendanceUserStats(ctx, start, end)
	if err != nil {
		return nil, err
	}

	// Build final user rows (including risk classification).
	var userRows []attendanceReportUserRow
	for _, st := range statsByUser {
		// Hanya sertakan satpam yang benar-benar punya scheduling/attendance
		// di rentang tanggal yang dipilih.
		if !st.HasData {
			continue
		}

		var score float64
		if r, ok := riskByUser[st.UserID]; ok {
			// Sejajarkan dengan perhitungan risk di dashboard: tanpa faktor
			// no_checkin yang secara praktis tidak digunakan.
			score = float64(r.LateCount*2 + r.AbsentCount*5 + r.MissedShiftCount*3)
		}
		level := "Low"
		if score >= 20 {
			level = "High"
		} else if score >= 10 {
			level = "Medium"
		}
		avgLate := 0.0
		if st.LateCount > 0 && st.TotalLateMinute > 0 {
			avgLate = float64(st.TotalLateMinute) / float64(st.LateCount)
		}
		userRows = append(userRows, attendanceReportUserRow{
			UserID:         st.UserID,
			Name:           st.Name,
			Position:       st.Position,
			PresentCount:   st.Present,
			AbsentCount:    st.Absent,
			LateCount:      st.LateCount,
			AvgLateMinutes: avgLate,
			RiskScore:      score,
			RiskLevel:      level,
		})
	}

	sort.Slice(userRows, func(i, j int) bool {
		if userRows[i].RiskScore == userRows[j].RiskScore {
			return userRows[i].Name < userRows[j].Name
		}
		return userRows[i].RiskScore > userRows[j].RiskScore
	})

	return &attendanceReportData{
		Summary:   summary,
		Trend:     trend,
		Breakdown: discRow,
		UserRows:  userRows,
	}, nil
}

// buildAttendanceUserStats aggregates per-user attendance stats for the given
// date range. It mirrors the logic used by ExportAttendanceMonitoringToExcel.
func (s *Service) buildAttendanceUserStats(ctx context.Context, start, end time.Time) (map[int64]*attendanceUserStats, error) {
	users, err := s.repo.ListSatpam(ctx, nil, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	statsByUser := make(map[int64]*attendanceUserStats, len(users))
	for _, u := range users {
		statsByUser[u.ID] = &attendanceUserStats{
			UserID:   u.ID,
			Name:     u.Name,
			Position: u.Jabatan,
		}
	}

	// Gunakan tanggal hari ini di timezone Asia/Jakarta, konsisten dengan
	// dashboard utama.
	wib, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(wib).Truncate(24 * time.Hour)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		attRows, err := s.repo.ListDailyAttendance(ctx, d)
		if err != nil {
			return nil, responses.InternalServerError(err)
		}
		attByUser := make(map[int64]*entities.AdminAttendanceRow)
		for _, r := range attRows {
			attByUser[r.UserID] = r
		}

		shiftRows, err := s.repo.ListUserShifts(ctx, &d, 0, 0)
		if err != nil {
			return nil, responses.InternalServerError(err)
		}
		type shiftInfo struct {
			Name  string
			Start string
		}
		shiftByUser := make(map[int64]shiftInfo)
		for _, r := range shiftRows {
			shiftByUser[r.UserID] = shiftInfo{
				Name:  r.ShiftName,
				Start: r.StartTime,
			}
		}

		for _, u := range users {
			if !u.WorkStartDate.IsZero() {
				ws := time.Date(u.WorkStartDate.Year(), u.WorkStartDate.Month(), u.WorkStartDate.Day(), 0, 0, 0, 0, u.WorkStartDate.Location())
				if ws.After(d) {
					continue
				}
			}

			stat := statsByUser[u.ID]
			attendance := attByUser[u.ID]
			info, hasShift := shiftByUser[u.ID]

			if !hasShift && attendance == nil {
				// Not scheduled in this date range.
				continue
			}

			// Pada titik ini user punya jadwal atau attendance pada hari ini.
			stat.HasData = true

			// Any scheduled shift (non-Libur filtering is already handled by
			// dashboard queries; here we mirror attendance monitoring export).
			stat.Absent += 0
			stat.Present += 0

			if hasShift {
				// Count as scheduled for this day.
				// (We don't store scheduled separately; absent = scheduled - present
				// is approximated via days where we mark absent below.)
				_ = info.Name
			}

			if attendance != nil {
				stat.Present++
				if attendance.ClockInStatus != nil {
					switch *attendance.ClockInStatus {
					case string(entities.LateStatusLate), string(entities.LateStatusTooLate):
						stat.LateCount++
						if attendance.ClockInTime != nil && attendance.ShiftStart != "" {
							if tStart, err := time.Parse("15:04:05", attendance.ShiftStart); err == nil {
								shiftStartTime := time.Date(
									d.Year(), d.Month(), d.Day(),
									tStart.Hour(), tStart.Minute(), tStart.Second(),
									0, attendance.ClockInTime.Location(),
								)
								threshold := shiftStartTime.Add(time.Duration(attendance.LateTolerance) * time.Minute)
								if delay := attendance.ClockInTime.Sub(threshold); delay > 0 {
									stat.TotalLateMinute += int(delay.Minutes())
								}
							}
						}
					}
				}
			} else if hasShift && !d.After(today) {
				// Scheduled but no attendance and date already passed => absent.
				stat.Absent++
			}
		}
	}

	return statsByUser, nil
}

// GenerateAttendanceReportXLSX builds the multi-sheet HR XLSX file for the
// attendance report.
func (s *Service) GenerateAttendanceReportXLSX(ctx context.Context, startDate, endDate time.Time) ([]byte, error) {
	data, err := s.buildAttendanceReportData(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	summarySheet := f.GetSheetName(f.GetActiveSheetIndex())
	_ = f.SetSheetName(summarySheet, "Summary")
	trendSheet := "Trend (Daily)"
	breakdownSheet := "Breakdown"
	attendanceSheet := "Attendance List"

	if _, err := f.NewSheet(trendSheet); err != nil {
		return nil, responses.InternalServerError(err)
	}
	if _, err := f.NewSheet(breakdownSheet); err != nil {
		return nil, responses.InternalServerError(err)
	}
	if _, err := f.NewSheet(attendanceSheet); err != nil {
		return nil, responses.InternalServerError(err)
	}
	if idx, err := f.GetSheetIndex(summarySheet); err == nil && idx >= 0 {
		f.SetActiveSheet(idx)
	}

	// Summary sheet
	_ = f.SetCellValue(summarySheet, "A1", "SIAGA Attendance Executive Report")
	_ = f.SetCellValue(summarySheet, "A2", fmt.Sprintf("Period: %s - %s",
		data.Summary.DateFrom.Format("2006-01-02"),
		data.Summary.DateTo.Format("2006-01-02"),
	))
	_ = f.SetCellValue(summarySheet, "A3", fmt.Sprintf("Generated at: %s",
		data.Summary.GeneratedAt.Format("2006-01-02 15:04"),
	))

	headers := []string{"Metric", "Value"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 5)
		_ = f.SetCellValue(summarySheet, cell, h)
	}
	_ = setPrettyHeaderStyle(f, summarySheet, len(headers))

	summaryRows := [][]interface{}{
		{"Attendance Rate", fmt.Sprintf("%.1f%%", data.Summary.AttendanceRate)},
		{"On-Time Rate", fmt.Sprintf("%.1f%%", data.Summary.OnTimeRate)},
		{"Absent Rate", fmt.Sprintf("%.1f%%", data.Summary.AbsentRate)},
		{"Avg Late Minutes", fmt.Sprintf("%.1f", data.Summary.AvgLateMinutes)},
	}
	rowIdx := 6
	for _, row := range summaryRows {
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		_ = f.SetSheetRow(summarySheet, cell, &row)
		rowIdx++
	}
	_ = f.SetColWidth(summarySheet, "A", "B", 24)

	// Trend sheet
	trendHeaders := []string{"Date", "Present", "Late", "Absent", "Upcoming"}
	for i, h := range trendHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(trendSheet, cell, h)
	}
	_ = setPrettyHeaderStyle(f, trendSheet, len(trendHeaders))
	rowIdx = 2
	for _, t := range data.Trend {
		row := []interface{}{
			t.Date.Format("2006-01-02"),
			t.Present,
			t.Late,
			t.Absent,
			t.NotYet,
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		_ = f.SetSheetRow(trendSheet, cell, &row)
		rowIdx++
	}
	_ = f.SetColWidth(trendSheet, "A", "E", 18)
	_ = f.SetPanes(trendSheet, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Breakdown sheet
	brHeaders := []string{"Category", "Count", "Percentage"}
	for i, h := range brHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(breakdownSheet, cell, h)
	}
	_ = setPrettyHeaderStyle(f, breakdownSheet, len(brHeaders))
	total := data.Breakdown.Late + data.Breakdown.EarlyLeave + data.Breakdown.NoCheckin + data.Breakdown.MissedShift + data.Breakdown.FutureShift
	rowIdx = 2
	addBreakdownRow := func(label string, count int) {
		if count == 0 && total == 0 {
			return
		}
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		row := []interface{}{label, count, fmt.Sprintf("%.1f%%", percent)}
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		_ = f.SetSheetRow(breakdownSheet, cell, &row)
		rowIdx++
	}
	addBreakdownRow("Late", data.Breakdown.Late)
	addBreakdownRow("Early leave", data.Breakdown.EarlyLeave)
	addBreakdownRow("No check-in", data.Breakdown.NoCheckin)
	addBreakdownRow("Missed shift", data.Breakdown.MissedShift)
	addBreakdownRow("Upcoming shifts", data.Breakdown.FutureShift)
	_ = f.SetColWidth(breakdownSheet, "A", "C", 30)
	_ = f.SetPanes(breakdownSheet, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Attendance list sheet
	attHeaders := []string{
		"Guard name",
		"Position",
		"Spot",
		"Present count",
		"Absent count",
		"Late count",
		"Avg late minutes",
		"Risk level",
	}
	for i, h := range attHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(attendanceSheet, cell, h)
	}
	_ = setPrettyHeaderStyle(f, attendanceSheet, len(attHeaders))
	rowIdx = 2
	for _, u := range data.UserRows {
		row := []interface{}{
			u.Name,
			u.Position,
			"", // Spot (optional, not aggregated here)
			u.PresentCount,
			u.AbsentCount,
			u.LateCount,
			u.AvgLateMinutes,
			fmt.Sprintf("%s (%.0f)", u.RiskLevel, u.RiskScore),
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		_ = f.SetSheetRow(attendanceSheet, cell, &row)
		rowIdx++
	}
	_ = f.SetColWidth(attendanceSheet, "A", "H", 22)
	_ = f.SetPanes(attendanceSheet, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

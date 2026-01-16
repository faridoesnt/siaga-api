package admin

import (
	"fmt"
	"sort"
	"time"
)

type overallStatus string

const (
	statusGood           overallStatus = "GOOD"
	statusNeedsAttention overallStatus = "NEEDS_ATTENTION"
	statusCritical       overallStatus = "CRITICAL"
)

type overallStatusView struct {
	Status overallStatus
	Label  string
	Color  struct {
		R int
		G int
		B int
	}
}

// computeOverallStatus determines Good / Needs Attention / Critical based on
// thresholds driven by absent rate and on-time rate.
func computeOverallStatus(summary attendanceReportSummary) overallStatusView {
	absent := summary.AbsentRate
	onTime := summary.OnTimeRate

	var status overallStatus

	switch {
	case absent < 20 && onTime >= 80:
		status = statusGood
	case absent > 50 || onTime < 50:
		status = statusCritical
	default:
		status = statusNeedsAttention
	}

	view := overallStatusView{Status: status}
	switch status {
	case statusGood:
		view.Label = "Good"
		view.Color = struct{ R, G, B int }{34, 197, 94} // green-500
	case statusNeedsAttention:
		view.Label = "Needs Attention"
		view.Color = struct{ R, G, B int }{245, 158, 11} // amber-500
	case statusCritical:
		view.Label = "Critical"
		view.Color = struct{ R, G, B int }{239, 68, 68} // red-500
	}
	return view
}

// buildKeyInsights generates up to 3 bullet strings summarizing key findings.
func buildKeyInsights(data *attendanceReportData) []string {
	var insights []string

	// 1) Best and worst day by present / absent.
	if len(data.Trend) > 0 {
		best := data.Trend[0]
		worst := data.Trend[0]

		for _, t := range data.Trend[1:] {
			if t.Present > best.Present {
				best = t
			}
			if t.Absent > worst.Absent {
				worst = t
			}
		}

		if best.Present > 0 {
			insights = append(insights,
				fmt.Sprintf("Best attendance day was %s (Present: %d).",
					best.Date.Format("2006-01-02"), best.Present))
		}
		if worst.Absent > 0 {
			insights = append(insights,
				fmt.Sprintf("Highest absence occurred on %s (Absent: %d).",
					worst.Date.Format("2006-01-02"), worst.Absent))
		}
	}

	// 2) Top risk guard.
	if len(data.UserRows) > 0 {
		// already sorted by risk desc, but be safe.
		userCopy := make([]attendanceReportUserRow, len(data.UserRows))
		copy(userCopy, data.UserRows)
		sort.Slice(userCopy, func(i, j int) bool {
			if userCopy[i].RiskScore == userCopy[j].RiskScore {
				return userCopy[i].Name < userCopy[j].Name
			}
			return userCopy[i].RiskScore > userCopy[j].RiskScore
		})
		top := userCopy[0]
		insights = append(insights,
			fmt.Sprintf("Top risk guard: %s (Absent: %d, Late: %d, Avg late: %.1f min).",
				top.Name, top.AbsentCount, top.LateCount, top.AvgLateMinutes))
	}

	// Fallback generic bullets when data is thin.
	if len(insights) == 0 {
		insights = append(insights, "Attendance trend data is limited for the selected period.")
	}

	// Cap to 3 bullets.
	if len(insights) > 3 {
		insights = insights[:3]
	}

	return insights
}

// helper to format a date range with a proper em dash for PDF text.
func formatDateRangeWithEmDash(from, to time.Time) string {
	return fmt.Sprintf("%s - %s", from.Format("2006-01-02"), to.Format("2006-01-02"))
}

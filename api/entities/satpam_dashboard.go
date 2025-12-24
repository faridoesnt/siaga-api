package entities

import "time"

type SatpamDashboardShift struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	StartTime           string `json:"start_time"`
	EndTime             string `json:"end_time"`
	LateToleranceMinute int    `json:"late_tolerance_minute"`
}

type SatpamDashboardAttendance struct {
	Status       AttendanceStatus `json:"status"`
	ClockInTime  *time.Time       `json:"clock_in_time,omitempty"`
	ClockOutTime *time.Time       `json:"clock_out_time,omitempty"`
	LateStatus   *LateStatus      `json:"late_status,omitempty"`
}

type SatpamDashboard struct {
	Date                  string                     `json:"date"`
	Shift                 *SatpamDashboardShift      `json:"shift"`
	Attendance            *SatpamDashboardAttendance `json:"attendance"`
	HasOpenAttendance     bool                       `json:"has_open_attendance"`
	CanClockIn            bool                       `json:"can_clock_in"`
	CanClockOut           bool                       `json:"can_clock_out"`
	OpenAttendanceSummary *SatpamDashboardAttendance `json:"open_attendance_summary,omitempty"`
}

type SatpamAttendanceHistoryItem struct {
	Date       string                     `json:"date"`
	Shift      *SatpamDashboardShift      `json:"shift"`
	Attendance *SatpamDashboardAttendance `json:"attendance"`
}

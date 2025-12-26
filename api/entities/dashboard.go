package entities

type AdminDashboardSummary struct {
	TotalSecurity int     `db:"total_security"`
	ScheduledDays int     `db:"scheduled_days"`
	PresentDays   int     `db:"present_days"`
	OnTimeDays    int     `db:"on_time_days"`
	AbsentDays    int     `db:"absent_days"`
	TotalLateMin  float64 `db:"total_late_minutes"`
	LateRecords   int     `db:"late_records"`

	// Past* fields hanya menghitung shift yang sudah lewat (shift_date <= CURDATE()).
	PastScheduledDays int `db:"past_scheduled_days"`
	PastPresentDays   int `db:"past_present_days"`
	PastOnTimeDays    int `db:"past_on_time_days"`
}

type AdminDashboardTrendRow struct {
	Date      string `db:"shift_date"`
	Scheduled int    `db:"scheduled"`
	Present   int    `db:"present"`
	Late      int    `db:"late"`
}

type AdminDashboardDisciplineRow struct {
	Late        int `db:"late"`
	EarlyLeave  int `db:"early_leave"`
	NoCheckin   int `db:"no_checkin"`
	MissedShift int `db:"missed_shift"`
	FutureShift int `db:"future_scheduled"`
}

type AdminDashboardRiskRow struct {
	UserID           int64  `db:"user_id"`
	UserName         string `db:"user_name"`
	Position         string `db:"position"`
	LateCount        int    `db:"late_count"`
	AbsentCount      int    `db:"absent_count"`
	NoCheckinCount   int    `db:"no_checkin_count"`
	MissedShiftCount int    `db:"missed_shift_count"`
}

type AdminDashboardConsistencyRow struct {
	UserID    int64 `db:"user_id"`
	Scheduled int   `db:"scheduled"`
	Present   int   `db:"present"`
}

type AdminDashboardAuditRow struct {
	ManualOverride  int `db:"manual_override"`
	CompleteRecords int `db:"complete_records"`
	TotalRecords    int `db:"total_records"`
}

type AdminDashboardHeroInsight struct {
	Headline string `json:"headline"`
	Severity string `json:"severity"` // normal | warning | critical
	Context  string `json:"context"`
}

type AdminDashboardKPI struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Delta  float64 `json:"delta"`
	Trend  string  `json:"trend"`  // up | down | flat
	Status string  `json:"status"` // good | warning | bad
}

type AdminDashboardResponse struct {
	Summary struct {
		TotalSecurity  int     `json:"total_security"`
		AttendanceRate float64 `json:"attendance_rate"`
		OnTimeRate     float64 `json:"on_time_rate"`
		AbsentRate     float64 `json:"absent_rate"`
		AvgLateMinutes float64 `json:"avg_late_minutes"`
	} `json:"summary"`

	AttendanceTrend struct {
		Labels     []string `json:"labels"`
		Present    []int    `json:"present"`
		Late       []int    `json:"late"`
		Absent     []int    `json:"absent"`
		BelumAbsen []int    `json:"belum_absen"`
	} `json:"attendance_trend"`

	DisciplineBreakdown struct {
		Late        int `json:"late"`
		EarlyLeave  int `json:"early_leave"`
		NoCheckin   int `json:"no_checkin"`
		MissedShift int `json:"missed_shift"`
		BelumAbsen  int `json:"belum_absen"`
	} `json:"discipline_breakdown"`

	RiskEmployees []struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Position   string  `json:"position"`
		RiskScore  float64 `json:"risk_score"`
		RiskReason string  `json:"risk_reason"`
	} `json:"risk_employees"`

	AttendanceConsistency struct {
		Consistent    int     `json:"consistent"`
		Irregular     int     `json:"irregular"`
		AvgStreakDays float64 `json:"avg_streak_days"`
	} `json:"attendance_consistency"`

	AuditCompliance struct {
		ManualOverride   int     `json:"manual_override"`
		DataCompleteness float64 `json:"data_completeness"`
	} `json:"audit_compliance"`

	// Insight & KPI view models for dashboard
	// HeroInsight AdminDashboardHeroInsight `json:"hero_insight"`
	KPIs        []AdminDashboardKPI       `json:"kpis"`
}

package entities

import "time"

type Shift struct {
	ID                  int64     `db:"id" json:"id"`
	Name                string    `db:"name" json:"name"`
	StartTime           string    `db:"start_time" json:"start_time"`
	EndTime             string    `db:"end_time" json:"end_time"`
	LateToleranceMinute int       `db:"late_tolerance_minute" json:"late_tolerance_minute"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

type UserShift struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	ShiftID   int64     `db:"shift_id" json:"shift_id"`
	ShiftDate time.Time `db:"shift_date" json:"shift_date"`
	IsSwapped bool      `db:"is_swapped" json:"is_swapped"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type UserShiftWithShift struct {
	UserShift
	Shift Shift `json:"shift"`
}

type AdminUserShiftRow struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	UserName  string    `db:"user_name" json:"user_name"`
	ShiftID   int64     `db:"shift_id" json:"shift_id"`
	ShiftName string    `db:"shift_name" json:"shift_name"`
	StartTime string    `db:"shift_start" json:"start_time"`
	EndTime   string    `db:"shift_end" json:"end_time"`
	ShiftDate time.Time `db:"shift_date" json:"shift_date"`
}

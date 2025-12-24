package entities

import "time"

type DailyActivityPhoto struct {
	ID               int64     `db:"id" json:"id"`
	UserID           int64     `db:"user_id" json:"user_id"`
	AttendanceID     int64     `db:"attendance_id" json:"attendance_id"`
	AttendanceSpotID *int64    `db:"attendance_spot_id" json:"attendance_spot_id,omitempty"`
	PhotoURL         string    `db:"photo_url" json:"photo_url"`
	Note             *string   `db:"note" json:"note,omitempty"`
	TakenAt          time.Time `db:"taken_at" json:"taken_at"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
}

type AdminAttendanceActivityRow struct {
	ID                 int64     `db:"id" json:"id"`
	AttendanceID       int64     `db:"attendance_id" json:"attendance_id"`
	PhotoURL           string    `db:"photo_url" json:"photo_url"`
	Note               *string   `db:"note" json:"note,omitempty"`
	TakenAt            time.Time `db:"taken_at" json:"taken_at"`
	AttendanceSpotID   *int64    `db:"attendance_spot_id" json:"attendance_spot_id,omitempty"`
	AttendanceSpotName *string   `db:"attendance_spot_name" json:"attendance_spot_name,omitempty"`
}


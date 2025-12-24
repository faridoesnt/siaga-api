package entities

import "time"

type AttendanceStatus string

const (
	AttendanceStatusNone       AttendanceStatus = "NONE"
	AttendanceStatusClockedIn  AttendanceStatus = "CLOCKED_IN"
	AttendanceStatusClockedOut AttendanceStatus = "CLOCKED_OUT"
)

type LateStatus string

const (
	LateStatusOnTime  LateStatus = "ON_TIME"
	LateStatusLate    LateStatus = "LATE"
	LateStatusTooLate LateStatus = "TOO_LATE"
)

type Attendance struct {
	ID                int64      `db:"id" json:"id"`
	UserID            int64      `db:"user_id" json:"user_id"`
	ShiftID           int64      `db:"shift_id" json:"shift_id"`
	AttendanceSpotID  *int64     `db:"attendance_spot_id" json:"attendance_spot_id,omitempty"`
	ClockInSpotID     *int64     `db:"clock_in_spot_id" json:"clock_in_spot_id,omitempty"`
	ClockOutSpotID    *int64     `db:"clock_out_spot_id" json:"clock_out_spot_id,omitempty"`
	AttendanceDate    time.Time  `db:"attendance_date" json:"attendance_date"`
	ClockInTime       *time.Time `db:"clock_in_time" json:"clock_in_time,omitempty"`
	ClockInLatitude   *float64   `db:"clock_in_latitude" json:"clock_in_latitude,omitempty"`
	ClockInLongitude  *float64   `db:"clock_in_longitude" json:"clock_in_longitude,omitempty"`
	ClockInStatus     *string    `db:"clock_in_status" json:"clock_in_status,omitempty"`
	ClockInPhotoURL   *string    `db:"clock_in_photo_url" json:"clock_in_photo_url,omitempty"`
	FaceVerified      bool       `db:"face_verified" json:"face_verified"`
	FaceMatchScore    *float64   `db:"face_match_score" json:"face_match_score,omitempty"`
	ClockOutTime      *time.Time `db:"clock_out_time" json:"clock_out_time,omitempty"`
	ClockOutLatitude  *float64   `db:"clock_out_latitude" json:"clock_out_latitude,omitempty"`
	ClockOutLongitude *float64   `db:"clock_out_longitude" json:"clock_out_longitude,omitempty"`
	ClockOutPhotoURL  *string    `db:"clock_out_photo_url" json:"clock_out_photo_url,omitempty"`
	OverrideByAdminID *int64     `db:"override_by_admin_id" json:"override_by_admin_id,omitempty"`
	OverrideAt        *time.Time `db:"override_at" json:"override_at,omitempty"`
	OverrideReason    *string    `db:"override_reason" json:"override_reason,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

type AttendanceSpot struct {
	ID           int64     `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Latitude     float64   `db:"latitude" json:"latitude"`
	Longitude    float64   `db:"longitude" json:"longitude"`
	RadiusMeters int       `db:"radius_meters" json:"radius_meters"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type UserAttendanceSpot struct {
	ID               int64      `db:"id" json:"id"`
	UserID           int64      `db:"user_id" json:"user_id"`
	AttendanceSpotID int64      `db:"attendance_spot_id" json:"attendance_spot_id"`
	ActiveFrom       time.Time  `db:"active_from" json:"active_from"`
	ActiveUntil      *time.Time `db:"active_until" json:"active_until,omitempty"`
}

type AdminAttendanceRow struct {
	AttendanceID      int64                          `db:"attendance_id" json:"-"`
	UserID            int64                          `db:"user_id" json:"-"`
	UserName          string                         `db:"user_name" json:"-"`
	ShiftName         string                         `db:"shift_name" json:"-"`
	ShiftStart        string                         `db:"shift_start" json:"-"`
	ShiftEnd          string                         `db:"shift_end" json:"-"`
	LateTolerance     int                            `db:"late_tolerance_minute" json:"-"`
	ClockInTime       *time.Time                     `db:"clock_in_time" json:"clock_in_time"`
	ClockOutTime      *time.Time                     `db:"clock_out_time" json:"clock_out_time"`
	ClockInStatus     *string                        `db:"clock_in_status" json:"status"`
	ClockInPhoto      *string                        `db:"clock_in_photo_url" json:"clock_in_photo_url"`
	ClockOutPhoto     *string                        `db:"clock_out_photo_url" json:"clock_out_photo_url"`
	FaceVerified      bool                           `db:"face_verified" json:"face_verified"`
	FaceMatchScore    *float64                       `db:"face_match_score" json:"face_match_score"`
	ClockInSpotID     *int64                         `db:"clock_in_spot_id" json:"-"`
	ClockInSpotName   *string                        `db:"clock_in_spot_name" json:"-"`
	ClockOutSpotID    *int64                         `db:"clock_out_spot_id" json:"-"`
	ClockOutSpotName  *string                        `db:"clock_out_spot_name" json:"-"`
	Activities        []*AdminAttendanceActivityRow  `db:"-" json:"activities,omitempty"`
}

type AdminUserAttendanceSpotRow struct {
	ID                 int64      `db:"id" json:"id"`
	UserID             int64      `db:"user_id" json:"user_id"`
	UserName           string     `db:"user_name" json:"user_name"`
	AttendanceSpotID   int64      `db:"attendance_spot_id" json:"attendance_spot_id"`
	AttendanceSpotName string     `db:"attendance_spot_name" json:"attendance_spot_name"`
	ActiveFrom         time.Time  `db:"active_from" json:"active_from"`
	ActiveUntil        *time.Time `db:"active_until" json:"active_until,omitempty"`
}

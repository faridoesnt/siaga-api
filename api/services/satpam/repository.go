package satpam

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"siaga-api/api/contracts"
	"siaga-api/api/entities"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetUserByID(ctx context.Context, id int64) (*entities.User, error)
	GetUserShiftForDate(ctx context.Context, userID int64, date time.Time) (*entities.UserShiftWithShift, error)
	GetAttendanceForDate(ctx context.Context, userID int64, date time.Time) (*entities.Attendance, error)
	GetAttendanceForUpdate(ctx context.Context, tx *sqlx.Tx, userID int64, date time.Time) (*entities.Attendance, error)
	GetOpenAttendance(ctx context.Context, userID int64) (*entities.Attendance, error)
	GetOpenAttendanceForUpdate(ctx context.Context, tx *sqlx.Tx, userID int64) (*entities.Attendance, error)
	GetPrimaryAttendanceSpot(ctx context.Context, userID int64) (*entities.AttendanceSpot, error)
	GetActiveAttendanceSpots(ctx context.Context, userID int64) ([]*entities.AttendanceSpot, error)
	GetFaceEmbeddings(ctx context.Context, userID int64) ([]string, error)
	ListMyShiftDates(ctx context.Context, userID int64, from time.Time) ([]time.Time, error)
	InsertAttendance(ctx context.Context, tx *sqlx.Tx, att *entities.Attendance) error
	UpdateAttendanceClockOut(ctx context.Context, tx *sqlx.Tx, attendanceID int64, clockOutTime time.Time, lat, lng float64, photoURL *string, spotID int64) error
	GetAttendanceByIDForUserAndDate(ctx context.Context, userID, attendanceID int64, date time.Time) (*entities.Attendance, error)
	InsertDailyActivityPhoto(ctx context.Context, photo *entities.DailyActivityPhoto) error
	InsertShiftSwapRequest(ctx context.Context, req *entities.ShiftSwapRequest) error
	InsertShiftSwapRequestTx(ctx context.Context, tx *sqlx.Tx, req *entities.ShiftSwapRequest) error
	GetUserShiftForUpdate(ctx context.Context, tx *sqlx.Tx, id int64) (*entities.UserShift, error)
	UpdateUserShift(ctx context.Context, tx *sqlx.Tx, us *entities.UserShift) error
	ListShiftSwapRequests(ctx context.Context, userID int64, status string) ([]*entities.ShiftSwapRequest, error)
	ListShiftSwapPeers(ctx context.Context, userID int64, date time.Time) ([]*entities.User, error)
}

type repository struct {
	app *contracts.App
}

func NewRepository(app *contracts.App) Repository {
	return &repository{app: app}
}

func (r *repository) GetUserByID(ctx context.Context, id int64) (*entities.User, error) {
	var user entities.User
	err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id)
	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *repository) ListMyShiftDates(ctx context.Context, userID int64, from time.Time) ([]time.Time, error) {
	var rows []struct {
		ShiftDate time.Time `db:"shift_date"`
	}
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT DISTINCT shift_date
		FROM user_shifts
		WHERE user_id = ? AND shift_date >= ?
		ORDER BY shift_date ASC
		LIMIT 60
	`, userID, from.Format("2006-01-02")); err != nil {
		if sqlxErrNoRows(err) {
			return []time.Time{}, nil
		}
		return nil, err
	}
	dates := make([]time.Time, 0, len(rows))
	for _, r := range rows {
		dates = append(dates, r.ShiftDate)
	}
	return dates, nil
}

func (r *repository) GetUserShiftForDate(ctx context.Context, userID int64, date time.Time) (*entities.UserShiftWithShift, error) {
	var result struct {
		UserShiftID        int64     `db:"us_id"`
		UserShiftUserID    int64     `db:"us_user_id"`
		UserShiftShiftID   int64     `db:"us_shift_id"`
		UserShiftShiftDate time.Time `db:"us_shift_date"`
		UserShiftCreatedAt time.Time `db:"us_created_at"`
		ShiftID            int64     `db:"s_id"`
		ShiftName          string    `db:"s_name"`
		ShiftStartTime     string    `db:"s_start_time"`
		ShiftEndTime       string    `db:"s_end_time"`
		ShiftLateTolerance int       `db:"s_late_tolerance_minute"`
		ShiftCreatedAt     time.Time `db:"s_created_at"`
		ShiftUpdatedAt     time.Time `db:"s_updated_at"`
	}

	err := r.app.Ds.ReaderDB.GetContext(ctx, &result, `
		SELECT
			us.id AS us_id,
			us.user_id AS us_user_id,
			us.shift_id AS us_shift_id,
			us.shift_date AS us_shift_date,
			us.created_at AS us_created_at,
			s.id AS s_id,
			s.name AS s_name,
			s.start_time AS s_start_time,
			s.end_time AS s_end_time,
			s.late_tolerance_minute AS s_late_tolerance_minute,
			s.created_at AS s_created_at,
			s.updated_at AS s_updated_at
		FROM user_shifts us
		INNER JOIN shifts s ON s.id = us.shift_id
		WHERE us.user_id = ? AND us.shift_date = ?
		LIMIT 1
	`, userID, date.Format("2006-01-02"))

	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}

	us := entities.UserShift{
		ID:        result.UserShiftID,
		UserID:    result.UserShiftUserID,
		ShiftID:   result.UserShiftShiftID,
		ShiftDate: result.UserShiftShiftDate,
		CreatedAt: result.UserShiftCreatedAt,
	}

	shift := entities.Shift{
		ID:                  result.ShiftID,
		Name:                result.ShiftName,
		StartTime:           result.ShiftStartTime,
		EndTime:             result.ShiftEndTime,
		LateToleranceMinute: result.ShiftLateTolerance,
		CreatedAt:           result.ShiftCreatedAt,
		UpdatedAt:           result.ShiftUpdatedAt,
	}

	return &entities.UserShiftWithShift{
		UserShift: us,
		Shift:     shift,
	}, nil
}

func (r *repository) GetAttendanceForDate(ctx context.Context, userID int64, date time.Time) (*entities.Attendance, error) {
	var att entities.Attendance
	err := r.app.Ds.ReaderDB.GetContext(ctx, &att, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			override_by_admin_id, override_at, override_reason,
			created_at, updated_at
		FROM attendance
		WHERE user_id = ? AND attendance_date = ?
		LIMIT 1
	`, userID, date.Format("2006-01-02"))
	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &att, nil
}

func (r *repository) GetAttendanceForUpdate(ctx context.Context, tx *sqlx.Tx, userID int64, date time.Time) (*entities.Attendance, error) {
	var att entities.Attendance
	err := tx.GetContext(ctx, &att, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			override_by_admin_id, override_at, override_reason,
			created_at, updated_at
		FROM attendance
		WHERE user_id = ? AND attendance_date = ?
		FOR UPDATE
	`, userID, date.Format("2006-01-02"))
	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &att, nil
}

func (r *repository) GetOpenAttendance(ctx context.Context, userID int64) (*entities.Attendance, error) {
	var rows []entities.Attendance
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			created_at, updated_at
		FROM attendance
		WHERE user_id = ? AND clock_out_time IS NULL
		ORDER BY attendance_date DESC, id DESC
		LIMIT 2
	`, userID); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("multiple open attendance records found for user %d", userID)
	}
	return &rows[0], nil
}

func (r *repository) GetOpenAttendanceForUpdate(ctx context.Context, tx *sqlx.Tx, userID int64) (*entities.Attendance, error) {
	var rows []entities.Attendance
	if err := tx.SelectContext(ctx, &rows, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			created_at, updated_at
		FROM attendance
		WHERE user_id = ? AND clock_out_time IS NULL
		ORDER BY attendance_date DESC, id DESC
		LIMIT 2
		FOR UPDATE
	`, userID); err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("multiple open attendance records found for user %d", userID)
	}
	return &rows[0], nil
}

func (r *repository) GetPrimaryAttendanceSpot(ctx context.Context, userID int64) (*entities.AttendanceSpot, error) {
	var spot entities.AttendanceSpot
	err := r.app.Ds.ReaderDB.GetContext(ctx, &spot, `
		SELECT s.id, s.name, s.latitude, s.longitude, s.radius_meters, s.created_at, s.updated_at
		FROM user_attendance_spots uas
		INNER JOIN attendance_spots s ON s.id = uas.attendance_spot_id
		WHERE uas.user_id = ?
		  AND uas.active_from <= CURDATE()
		  AND (uas.active_until IS NULL OR uas.active_until >= CURDATE())
		ORDER BY uas.active_from DESC, uas.id DESC
		LIMIT 1
	`, userID)
	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &spot, nil
}

func (r *repository) GetActiveAttendanceSpots(ctx context.Context, userID int64) ([]*entities.AttendanceSpot, error) {
	var spots []*entities.AttendanceSpot
	err := r.app.Ds.ReaderDB.SelectContext(ctx, &spots, `
		SELECT s.id, s.name, s.latitude, s.longitude, s.radius_meters, s.created_at, s.updated_at
		FROM user_attendance_spots uas
		INNER JOIN attendance_spots s ON s.id = uas.attendance_spot_id
		WHERE uas.user_id = ?
		  AND uas.active_from <= CURDATE()
		  AND (uas.active_until IS NULL OR uas.active_until >= CURDATE())
		ORDER BY uas.active_from DESC, uas.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	return spots, nil
}

func (r *repository) GetFaceEmbeddings(ctx context.Context, userID int64) ([]string, error) {
	var rows []string
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT embedding
		FROM face_embeddings
		WHERE user_id = ?
		ORDER BY id ASC
	`, userID); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) InsertAttendance(ctx context.Context, tx *sqlx.Tx, att *entities.Attendance) error {
	res, err := tx.NamedExecContext(ctx, `
		INSERT INTO attendance (
			user_id, shift_id, attendance_spot_id, clock_in_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score
		) VALUES (
			:user_id, :shift_id, :attendance_spot_id, :clock_in_spot_id,
			:attendance_date, :clock_in_time, :clock_in_latitude, :clock_in_longitude,
			:clock_in_status, :clock_in_photo_url, :face_verified, :face_match_score
		)
	`, att)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		att.ID = id
	}
	return nil
}

func (r *repository) UpdateAttendanceClockOut(ctx context.Context, tx *sqlx.Tx, attendanceID int64, clockOutTime time.Time, lat, lng float64, photoURL *string, spotID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE attendance
		SET clock_out_time = ?, clock_out_latitude = ?, clock_out_longitude = ?, clock_out_photo_url = ?, clock_out_spot_id = ?
		WHERE id = ?
	`, clockOutTime, lat, lng, photoURL, spotID, attendanceID)
	return err
}

func (r *repository) GetAttendanceByIDForUserAndDate(ctx context.Context, userID, attendanceID int64, date time.Time) (*entities.Attendance, error) {
	var att entities.Attendance
	err := r.app.Ds.ReaderDB.GetContext(ctx, &att, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			override_by_admin_id, override_at, override_reason,
			created_at, updated_at
		FROM attendance
		WHERE id = ? AND user_id = ? AND attendance_date = ?
		LIMIT 1
	`, attendanceID, userID, date.Format("2006-01-02"))
	if err != nil {
		if sqlxErrNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &att, nil
}

func (r *repository) InsertDailyActivityPhoto(ctx context.Context, photo *entities.DailyActivityPhoto) error {
	res, err := r.app.Ds.WriterDB.NamedExecContext(ctx, `
		INSERT INTO daily_activity_photos (
			user_id, attendance_id, attendance_spot_id, photo_url, note, taken_at
		) VALUES (
			:user_id, :attendance_id, :attendance_spot_id, :photo_url, :note, :taken_at
		)
	`, photo)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		photo.ID = id
	}
	return nil
}

func (r *repository) InsertShiftSwapRequest(ctx context.Context, req *entities.ShiftSwapRequest) error {
	res, err := r.app.Ds.WriterDB.NamedExecContext(ctx, `
		INSERT INTO shift_swap_requests (
			requester_user_id, target_user_id, shift_date,
			requester_user_shift_id, target_user_shift_id,
			status, reason
		) VALUES (
			:requester_user_id, :target_user_id, :shift_date,
			:requester_user_shift_id, :target_user_shift_id,
			:status, :reason
		)
	`, req)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		req.ID = id
	}
	return nil
}

func (r *repository) InsertShiftSwapRequestTx(ctx context.Context, tx *sqlx.Tx, req *entities.ShiftSwapRequest) error {
	res, err := tx.NamedExecContext(ctx, `
		INSERT INTO shift_swap_requests (
			requester_user_id, target_user_id, shift_date,
			requester_user_shift_id, target_user_shift_id,
			status, reason
		) VALUES (
			:requester_user_id, :target_user_id, :shift_date,
			:requester_user_shift_id, :target_user_shift_id,
			:status, :reason
		)
	`, req)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		req.ID = id
	}
	return nil
}

func (r *repository) ListShiftSwapRequests(ctx context.Context, userID int64, status string) ([]*entities.ShiftSwapRequest, error) {
	args := []interface{}{userID}
	query := `
		SELECT
			id, requester_user_id, target_user_id, shift_date,
			requester_user_shift_id, target_user_shift_id,
			status, reason, note, decided_by, decided_at,
			created_at, updated_at
		FROM shift_swap_requests
		WHERE requester_user_id = ?
	`
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	var rows []*entities.ShiftSwapRequest
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListShiftSwapPeers(ctx context.Context, userID int64, date time.Time) ([]*entities.User, error) {
	var users []*entities.User
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &users, `
		SELECT DISTINCT
			u.id, u.name, u.email, u.password_hash, u.role,
			u.work_start_date, u.active, u.created_at, u.updated_at
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id
		WHERE us.shift_date = ?
		  AND u.role = 'SATPAM'
		  AND u.active = 1
		  AND u.id <> ?
		ORDER BY u.name ASC
	`, date.Format("2006-01-02"), userID); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) GetUserShiftForUpdate(ctx context.Context, tx *sqlx.Tx, id int64) (*entities.UserShift, error) {
	var us entities.UserShift
	err := tx.GetContext(ctx, &us, `
		SELECT id, user_id, shift_id, shift_date, is_swapped, created_at
		FROM user_shifts
		WHERE id = ?
		FOR UPDATE
	`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &us, nil
}

func (r *repository) UpdateUserShift(ctx context.Context, tx *sqlx.Tx, us *entities.UserShift) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE user_shifts
		SET user_id = ?, shift_id = ?, shift_date = ?, is_swapped = ?
		WHERE id = ?
	`, us.UserID, us.ShiftID, us.ShiftDate.Format("2006-01-02"), us.IsSwapped, us.ID)
	return err
}

func sqlxErrNoRows(err error) bool {
	return err == sql.ErrNoRows
}

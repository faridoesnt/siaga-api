package admin

import (
	"context"
	"database/sql"
	"time"

	"siaga-api/api/contracts"
	"siaga-api/api/entities"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	// Users / Satpam
	CreateSatpam(ctx context.Context, email, passwordHash, name string, workStartDate *time.Time) (*entities.User, error)
	ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.User, error)
	SetSatpamActive(ctx context.Context, userID int64, active bool) error
	UpdateSatpam(ctx context.Context, userID int64, email, name string, workStartDate *time.Time) (*entities.User, error)
	DeleteSatpam(ctx context.Context, userID int64) error

	// Attendance spots
	CreateAttendanceSpot(ctx context.Context, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error)
	ListAttendanceSpots(ctx context.Context, limit, offset int) ([]*entities.AttendanceSpot, error)
	AssignUserAttendanceSpot(ctx context.Context, tx *sqlx.Tx, userID, attendanceSpotID int64, activeFrom time.Time) error
	UpdateAttendanceSpot(ctx context.Context, id int64, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error)
	DeleteAttendanceSpot(ctx context.Context, id int64) error

	// Shifts
	CreateShift(ctx context.Context, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error)
	ListShifts(ctx context.Context, limit, offset int) ([]*entities.Shift, error)
	AssignUserShift(ctx context.Context, userID, shiftID int64, shiftDate time.Time) (*entities.UserShift, error)
	HasAttendanceForDate(ctx context.Context, userID int64, date time.Time) (bool, error)
	UpdateShift(ctx context.Context, id int64, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error)
	DeleteShift(ctx context.Context, id int64) error

	// Shift swaps
	ListShiftSwapRequests(ctx context.Context, status string, limit, offset int) ([]*entities.ShiftSwapRequest, error)
	GetShiftSwapRequestForUpdate(ctx context.Context, tx *sqlx.Tx, id int64) (*entities.ShiftSwapRequest, error)
	GetUserShiftForUpdate(ctx context.Context, tx *sqlx.Tx, id int64) (*entities.UserShift, error)
	UpdateUserShift(ctx context.Context, tx *sqlx.Tx, us *entities.UserShift) error
	UpdateShiftSwapRequest(ctx context.Context, tx *sqlx.Tx, req *entities.ShiftSwapRequest) error

	// Attendance monitoring
	ListDailyAttendance(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceRow, error)
	ListDailyActivityPhotos(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceActivityRow, error)
	ListOpenAttendance(ctx context.Context) ([]*entities.AdminAttendanceRow, error)
	ListUserShifts(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserShiftRow, error)
	ListUserAttendanceSpots(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserAttendanceSpotRow, error)

	UpdateUserShiftAssignment(ctx context.Context, id, shiftID int64, shiftDate time.Time) (*entities.UserShift, error)
	DeleteUserShiftAssignment(ctx context.Context, id int64) error

	UpdateUserAttendanceSpot(ctx context.Context, id, attendanceSpotID int64, activeFrom time.Time, activeUntil *time.Time) (*entities.UserAttendanceSpot, error)
	DeleteUserAttendanceSpot(ctx context.Context, id int64) error

	// Face embeddings
	GetSatpamByID(ctx context.Context, userID int64) (*entities.User, error)
	ReplaceFaceEmbeddings(ctx context.Context, userID int64, embeddings []string, model string) error
	GetFaceEmbeddingSummary(ctx context.Context, userID int64) (*entities.FaceEmbeddingSummary, error)
}

type repository struct {
	app *contracts.App
}

func NewRepository(app *contracts.App) Repository {
	return &repository{app: app}
}

// Users / Satpam

func (r *repository) CreateSatpam(ctx context.Context, email, passwordHash, name string, workStartDate *time.Time) (*entities.User, error) {
	res, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO users (name, email, password_hash, role, work_start_date, active)
		VALUES (?, ?, ?, 'SATPAM', ?, 1)
	`, name, email, passwordHash, workStartDate)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var user entities.User
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.User, error) {
	args := []interface{}{"SATPAM"}
	query := `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE role = ?
	`
	if active != nil {
		query += " AND active = ?"
		if *active {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	query += " ORDER BY name ASC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	var users []*entities.User
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &users, query, args...); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) SetSatpamActive(ctx context.Context, userID int64, active bool) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE users
		SET active = ?
		WHERE id = ? AND role = 'SATPAM'
	`, active, userID)
	return err
}

func (r *repository) UpdateSatpam(ctx context.Context, userID int64, email, name string, workStartDate *time.Time) (*entities.User, error) {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?, work_start_date = ?, updated_at = NOW()
		WHERE id = ? AND role = 'SATPAM'
	`, name, email, workStartDate, userID)
	if err != nil {
		return nil, err
	}

	var user entities.User
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ?
	`, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *repository) DeleteSatpam(ctx context.Context, userID int64) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		DELETE FROM users
		WHERE id = ? AND role = 'SATPAM'
	`, userID)
	return err
}

func (r *repository) GetSatpamByID(ctx context.Context, userID int64) (*entities.User, error) {
	var user entities.User
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ? AND role = 'SATPAM'
	`, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Attendance spots

func (r *repository) CreateAttendanceSpot(ctx context.Context, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error) {
	res, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO attendance_spots (name, latitude, longitude, radius_meters)
		VALUES (?, ?, ?, ?)
	`, name, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var spot entities.AttendanceSpot
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &spot, `
		SELECT id, name, latitude, longitude, radius_meters, created_at, updated_at
		FROM attendance_spots
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &spot, nil
}

func (r *repository) ListAttendanceSpots(ctx context.Context, limit, offset int) ([]*entities.AttendanceSpot, error) {
	args := []interface{}{}
	var spots []*entities.AttendanceSpot
	query := `
		SELECT id, name, latitude, longitude, radius_meters, created_at, updated_at
		FROM attendance_spots
		ORDER BY name ASC
	`
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &spots, query, args...); err != nil {
		return nil, err
	}
	return spots, nil
}

func (r *repository) AssignUserAttendanceSpot(ctx context.Context, tx *sqlx.Tx, userID, attendanceSpotID int64, activeFrom time.Time) error {
	// Close previous active spots
	_, err := tx.ExecContext(ctx, `
		UPDATE user_attendance_spots
		SET active_until = DATE_SUB(?, INTERVAL 1 DAY)
		WHERE user_id = ?
		  AND (active_until IS NULL OR active_until >= ?)
	`, activeFrom.Format("2006-01-02"), userID, activeFrom.Format("2006-01-02"))
	if err != nil {
		return err
	}

	// Insert new assignment
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_attendance_spots (user_id, attendance_spot_id, active_from)
		VALUES (?, ?, ?)
	`, userID, attendanceSpotID, activeFrom.Format("2006-01-02"))
	return err
}

func (r *repository) UpdateAttendanceSpot(ctx context.Context, id int64, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error) {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE attendance_spots
		SET name = ?, latitude = ?, longitude = ?, radius_meters = ?, updated_at = NOW()
		WHERE id = ?
	`, name, latitude, longitude, radiusMeters, id)
	if err != nil {
		return nil, err
	}

	var spot entities.AttendanceSpot
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &spot, `
		SELECT id, name, latitude, longitude, radius_meters, created_at, updated_at
		FROM attendance_spots
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &spot, nil
}

func (r *repository) DeleteAttendanceSpot(ctx context.Context, id int64) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		DELETE FROM attendance_spots WHERE id = ?
	`, id)
	return err
}

// Shifts

func (r *repository) CreateShift(ctx context.Context, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error) {
	res, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO shifts (name, start_time, end_time, late_tolerance_minute)
		VALUES (?, ?, ?, ?)
	`, name, startTime.Format("15:04:05"), endTime.Format("15:04:05"), lateTolerance)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var shift entities.Shift
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &shift, `
		SELECT id, name, start_time, end_time, late_tolerance_minute, created_at, updated_at
		FROM shifts
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &shift, nil
}

func (r *repository) ListShifts(ctx context.Context, limit, offset int) ([]*entities.Shift, error) {
	args := []interface{}{}
	var shifts []*entities.Shift
	query := `
		SELECT id, name, start_time, end_time, late_tolerance_minute, created_at, updated_at
		FROM shifts
		ORDER BY start_time ASC
	`
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &shifts, query, args...); err != nil {
		return nil, err
	}
	return shifts, nil
}

func (r *repository) AssignUserShift(ctx context.Context, userID, shiftID int64, shiftDate time.Time) (*entities.UserShift, error) {
	res, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO user_shifts (user_id, shift_id, shift_date)
		VALUES (?, ?, ?)
	`, userID, shiftID, shiftDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var us entities.UserShift
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &us, `
		SELECT id, user_id, shift_id, shift_date, is_swapped, created_at
		FROM user_shifts
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &us, nil
}

func (r *repository) UpdateShift(ctx context.Context, id int64, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error) {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE shifts
		SET name = ?, start_time = ?, end_time = ?, late_tolerance_minute = ?, updated_at = NOW()
		WHERE id = ?
	`, name, startTime.Format("15:04:05"), endTime.Format("15:04:05"), lateTolerance, id)
	if err != nil {
		return nil, err
	}

	var shift entities.Shift
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &shift, `
		SELECT id, name, start_time, end_time, late_tolerance_minute, created_at, updated_at
		FROM shifts
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &shift, nil
}

func (r *repository) DeleteShift(ctx context.Context, id int64) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		DELETE FROM shifts WHERE id = ?
	`, id)
	return err
}

func (r *repository) HasAttendanceForDate(ctx context.Context, userID int64, date time.Time) (bool, error) {
	var count int
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &count, `
		SELECT COUNT(1)
		FROM attendance
		WHERE user_id = ? AND attendance_date = ?
	`, userID, date.Format("2006-01-02")); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) UpdateUserShiftAssignment(ctx context.Context, id, shiftID int64, shiftDate time.Time) (*entities.UserShift, error) {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE user_shifts
		SET shift_id = ?, shift_date = ?, updated_at = NOW()
		WHERE id = ?
	`, shiftID, shiftDate.Format("2006-01-02"), id)
	if err != nil {
		return nil, err
	}

	var us entities.UserShift
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &us, `
		SELECT id, user_id, shift_id, shift_date, is_swapped, created_at
		FROM user_shifts
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &us, nil
}

func (r *repository) DeleteUserShiftAssignment(ctx context.Context, id int64) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		DELETE FROM user_shifts WHERE id = ?
	`, id)
	return err
}

// Shift swaps

func (r *repository) ListShiftSwapRequests(ctx context.Context, status string, limit, offset int) ([]*entities.ShiftSwapRequest, error) {
	args := []interface{}{}
	query := `
		SELECT
			ssr.id,
			ssr.requester_user_id,
			ssr.target_user_id,
			ru.name AS requester_name,
			tu.name AS target_name,
			ssr.shift_date,
			ssr.requester_user_shift_id,
			ssr.target_user_shift_id,
			ssr.status,
			ssr.reason,
			ssr.note,
			ssr.decided_by,
			ssr.decided_at,
			ssr.created_at,
			ssr.updated_at
		FROM shift_swap_requests ssr
		INNER JOIN users ru ON ru.id = ssr.requester_user_id
		INNER JOIN users tu ON tu.id = ssr.target_user_id
		WHERE 1=1
	`
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	var rows []*entities.ShiftSwapRequest
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) GetShiftSwapRequestForUpdate(ctx context.Context, tx *sqlx.Tx, id int64) (*entities.ShiftSwapRequest, error) {
	var req entities.ShiftSwapRequest
	err := tx.GetContext(ctx, &req, `
		SELECT
			id, requester_user_id, target_user_id, shift_date,
			requester_user_shift_id, target_user_shift_id,
			status, reason, note, decided_by, decided_at,
			created_at, updated_at
		FROM shift_swap_requests
		WHERE id = ?
		FOR UPDATE
	`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &req, nil
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

func (r *repository) UpdateShiftSwapRequest(ctx context.Context, tx *sqlx.Tx, req *entities.ShiftSwapRequest) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE shift_swap_requests
		SET status = ?, note = ?, decided_by = ?, decided_at = ?, updated_at = NOW()
		WHERE id = ?
	`, req.Status, req.Note, req.DecidedBy, req.DecidedAt, req.ID)
	return err
}

// Attendance monitoring

func (r *repository) ListDailyAttendance(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceRow, error) {
	var rows []*entities.AdminAttendanceRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			a.id AS attendance_id,
			u.id AS user_id,
			u.name AS user_name,
			s.name AS shift_name,
			s.start_time AS shift_start,
			s.end_time AS shift_end,
			s.late_tolerance_minute AS late_tolerance_minute,
			a.clock_in_time,
			a.clock_out_time,
			a.clock_in_status,
			a.clock_in_photo_url,
			a.clock_out_photo_url,
			a.face_verified,
			a.face_match_score,
			cis.id AS clock_in_spot_id,
			cis.name AS clock_in_spot_name,
			cos.id AS clock_out_spot_id,
			cos.name AS clock_out_spot_name
		FROM attendance a
		INNER JOIN users u ON u.id = a.user_id
		INNER JOIN shifts s ON s.id = a.shift_id
		LEFT JOIN attendance_spots cis ON cis.id = a.clock_in_spot_id
		LEFT JOIN attendance_spots cos ON cos.id = a.clock_out_spot_id
		WHERE a.attendance_date = ?
		ORDER BY u.name ASC
	`, date.Format("2006-01-02")); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListOpenAttendance(ctx context.Context) ([]*entities.AdminAttendanceRow, error) {
	var rows []*entities.AdminAttendanceRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			a.id AS attendance_id,
			u.id AS user_id,
			u.name AS user_name,
			s.name AS shift_name,
			s.start_time AS shift_start,
			s.end_time AS shift_end,
			s.late_tolerance_minute AS late_tolerance_minute,
			a.clock_in_time,
			a.clock_out_time,
			a.clock_in_status,
			a.clock_in_photo_url,
			a.clock_out_photo_url,
			a.face_verified,
			a.face_match_score,
			cis.id AS clock_in_spot_id,
			cis.name AS clock_in_spot_name,
			cos.id AS clock_out_spot_id,
			cos.name AS clock_out_spot_name
		FROM attendance a
		INNER JOIN users u ON u.id = a.user_id
		INNER JOIN shifts s ON s.id = a.shift_id
		LEFT JOIN attendance_spots cis ON cis.id = a.clock_in_spot_id
		LEFT JOIN attendance_spots cos ON cos.id = a.clock_out_spot_id
		WHERE a.clock_out_time IS NULL
		ORDER BY a.attendance_date ASC, a.id ASC
	`); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListDailyActivityPhotos(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceActivityRow, error) {
	var rows []*entities.AdminAttendanceActivityRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			dap.id,
			dap.attendance_id,
			dap.photo_url,
			dap.note,
			dap.taken_at,
			dap.attendance_spot_id,
			s.name AS attendance_spot_name
		FROM daily_activity_photos dap
		INNER JOIN attendance a ON a.id = dap.attendance_id
		LEFT JOIN attendance_spots s ON s.id = dap.attendance_spot_id
		WHERE a.attendance_date = ?
		ORDER BY dap.taken_at ASC
	`, date.Format("2006-01-02")); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListUserShifts(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserShiftRow, error) {
	args := []interface{}{}
	query := `
		SELECT
			us.id AS id,
			us.user_id AS user_id,
			u.name AS user_name,
			us.shift_id AS shift_id,
			s.name AS shift_name,
			s.start_time AS shift_start,
			s.end_time AS shift_end,
			us.shift_date AS shift_date
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id
		INNER JOIN shifts s ON s.id = us.shift_id
		WHERE 1=1
	`
	if date != nil {
		query += " AND us.shift_date = ?"
		args = append(args, date.Format("2006-01-02"))
	}
	query += " ORDER BY us.shift_date ASC, u.name ASC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	var rows []*entities.AdminUserShiftRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListUserAttendanceSpots(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserAttendanceSpotRow, error) {
	var rows []*entities.AdminUserAttendanceSpotRow
	args := []interface{}{}
	query := `
		SELECT
			uas.id AS id,
			uas.user_id AS user_id,
			u.name AS user_name,
			uas.attendance_spot_id AS attendance_spot_id,
			s.name AS attendance_spot_name,
			uas.active_from AS active_from,
			uas.active_until AS active_until
		FROM user_attendance_spots uas
		INNER JOIN users u ON u.id = uas.user_id
		INNER JOIN attendance_spots s ON s.id = uas.attendance_spot_id
	`
	if date != nil {
		ds := date.Format("2006-01-02")
		query += `
		WHERE uas.active_from <= ?
		  AND (uas.active_until IS NULL OR uas.active_until >= ?)
		`
		args = append(args, ds, ds)
	}
	query += " ORDER BY u.name ASC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) UpdateUserAttendanceSpot(ctx context.Context, id, attendanceSpotID int64, activeFrom time.Time, activeUntil *time.Time) (*entities.UserAttendanceSpot, error) {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE user_attendance_spots
		SET attendance_spot_id = ?, active_from = ?, active_until = ?
		WHERE id = ?
	`, attendanceSpotID, activeFrom.Format("2006-01-02"), activeUntil, id)
	if err != nil {
		return nil, err
	}

	var uas entities.UserAttendanceSpot
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &uas, `
		SELECT id, user_id, attendance_spot_id, active_from, active_until
		FROM user_attendance_spots
		WHERE id = ?
	`, id); err != nil {
		return nil, err
	}
	return &uas, nil
}

func (r *repository) DeleteUserAttendanceSpot(ctx context.Context, id int64) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		DELETE FROM user_attendance_spots WHERE id = ?
	`, id)
	return err
}

// Face embeddings

func (r *repository) ReplaceFaceEmbeddings(ctx context.Context, userID int64, embeddings []string, model string) error {
	tx, err := r.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM face_embeddings WHERE user_id = ?
	`, userID); err != nil {
		return err
	}

	for _, emb := range embeddings {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO face_embeddings (user_id, embedding, model)
			VALUES (?, ?, ?)
		`, userID, emb, model); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *repository) GetFaceEmbeddingSummary(ctx context.Context, userID int64) (*entities.FaceEmbeddingSummary, error) {
	var row struct {
		Count     int        `db:"cnt"`
		Model     *string    `db:"model"`
		UpdatedAt *time.Time `db:"updated_at"`
	}
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			COUNT(*) AS cnt,
			MAX(model) AS model,
			MAX(updated_at) AS updated_at
		FROM face_embeddings
		WHERE user_id = ?
	`, userID); err != nil {
		return nil, err
	}
	return &entities.FaceEmbeddingSummary{
		UserID:    userID,
		Count:     row.Count,
		Model:     row.Model,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

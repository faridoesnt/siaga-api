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
	CreateSatpam(ctx context.Context, payload *entities.SatpamUpsertPayload, passwordHash string) (*entities.SatpamWithProfile, error)
	ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.SatpamWithProfile, error)
	SetSatpamActive(ctx context.Context, userID int64, active bool) error
	UpdateSatpam(ctx context.Context, userID int64, payload *entities.SatpamUpsertPayload) (*entities.SatpamWithProfile, error)
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
	UpsertUserShift(ctx context.Context, userID, shiftID int64, shiftDate time.Time) (inserted bool, updated bool, err error)
	GetShiftIDsByNames(ctx context.Context, names []string) (map[string]int64, error)
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

	// Dashboard
	GetDashboardSummary(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardSummary, error)
	GetDashboardTrend(ctx context.Context, startDate, endDate time.Time) ([]*entities.AdminDashboardTrendRow, error)
	GetDashboardDiscipline(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardDisciplineRow, error)
	GetDashboardRiskEmployees(ctx context.Context, startDate, endDate time.Time, limit int) ([]*entities.AdminDashboardRiskRow, error)
	GetDashboardConsistency(ctx context.Context, startDate, endDate time.Time) ([]*entities.AdminDashboardConsistencyRow, error)
	GetDashboardAudit(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardAuditRow, error)

	// RBAC
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
	ListPermissions(ctx context.Context) ([]*entities.Permission, error)
	ListAdmins(ctx context.Context, limit, offset int) ([]*entities.User, error)
	GetAdminByID(ctx context.Context, id int64) (*entities.User, error)
}

type repository struct {
	app *contracts.App
}

func NewRepository(app *contracts.App) Repository {
	return &repository{app: app}
}

// Users / Satpam

func (r *repository) CreateSatpam(ctx context.Context, payload *entities.SatpamUpsertPayload, passwordHash string) (*entities.SatpamWithProfile, error) {
	tx, err := r.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (name, email, password_hash, role, work_start_date, active)
		VALUES (?, ?, ?, 'SATPAM', ?, ?)
	`, payload.Name, payload.Email, passwordHash, payload.WorkStartDate, payload.Active)
	if err != nil {
		return nil, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO satpam_profiles (
			user_id, jabatan, jenis_kelamin, tanggal_lahir, tempat_lahir, no_ktp,
			alamat, no_telepon, agama, status_pernikahan, kebangsaan, work_start_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, payload.Jabatan, payload.JenisKelamin, payload.TanggalLahir, payload.TempatLahir, payload.NoKTP,
		payload.Alamat, payload.NoTelepon, payload.Agama, payload.StatusPernikahan, payload.Kebangsaan, payload.WorkStartDate)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var row struct {
		ID              int64      `db:"id"`
		Name            string     `db:"name"`
		Email           string     `db:"email"`
		Role            string     `db:"role"`
		Active          bool       `db:"active"`
		Jabatan         string     `db:"jabatan"`
		JenisKelamin    string     `db:"jenis_kelamin"`
		TanggalLahir    *time.Time `db:"tanggal_lahir"`
		TempatLahir     *string    `db:"tempat_lahir"`
		NoKTP           *string    `db:"no_ktp"`
		Alamat          string     `db:"alamat"`
		NoTelepon       string     `db:"no_telepon"`
		Agama           *string    `db:"agama"`
		StatusPernikahan *string   `db:"status_pernikahan"`
		Kebangsaan      *string    `db:"kebangsaan"`
		WorkStartDate   time.Time  `db:"work_start_date"`
	}

	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			u.id, u.name, u.email, u.role, u.active,
			p.jabatan, p.jenis_kelamin, p.tanggal_lahir, p.tempat_lahir, p.no_ktp,
			p.alamat, p.no_telepon, p.agama, p.status_pernikahan, p.kebangsaan,
			p.work_start_date
		FROM users u
		INNER JOIN satpam_profiles p ON p.user_id = u.id
		WHERE u.id = ?
	`, userID); err != nil {
		return nil, err
	}

	return &entities.SatpamWithProfile{
		ID:               row.ID,
		Name:             row.Name,
		Email:            row.Email,
		Role:             row.Role,
		Active:           row.Active,
		Jabatan:          row.Jabatan,
		JenisKelamin:     row.JenisKelamin,
		TanggalLahir:     row.TanggalLahir,
		TempatLahir:      row.TempatLahir,
		NoKTP:            row.NoKTP,
		Alamat:           row.Alamat,
		NoTelepon:        row.NoTelepon,
		Agama:            row.Agama,
		StatusPernikahan: row.StatusPernikahan,
		Kebangsaan:       row.Kebangsaan,
		WorkStartDate:    row.WorkStartDate,
	}, nil
}

func (r *repository) ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.SatpamWithProfile, error) {
	args := []interface{}{"SATPAM"}
	query := `
		SELECT
			u.id, u.name, u.email, u.role, u.active,
			p.jabatan, p.jenis_kelamin, p.tanggal_lahir, p.tempat_lahir, p.no_ktp,
			p.alamat, p.no_telepon, p.agama, p.status_pernikahan, p.kebangsaan,
			p.work_start_date
		FROM users u
		INNER JOIN satpam_profiles p ON p.user_id = u.id
		WHERE u.role = ?
	`
	if active != nil {
		query += " AND u.active = ?"
		if *active {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
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

	var rows []struct {
		ID              int64      `db:"id"`
		Name            string     `db:"name"`
		Email           string     `db:"email"`
		Role            string     `db:"role"`
		Active          bool       `db:"active"`
		Jabatan         string     `db:"jabatan"`
		JenisKelamin    string     `db:"jenis_kelamin"`
		TanggalLahir    *time.Time `db:"tanggal_lahir"`
		TempatLahir     *string    `db:"tempat_lahir"`
		NoKTP           *string    `db:"no_ktp"`
		Alamat          string     `db:"alamat"`
		NoTelepon       string     `db:"no_telepon"`
		Agama           *string    `db:"agama"`
		StatusPernikahan *string   `db:"status_pernikahan"`
		Kebangsaan      *string    `db:"kebangsaan"`
		WorkStartDate   time.Time  `db:"work_start_date"`
	}
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	result := make([]*entities.SatpamWithProfile, 0, len(rows))
	for _, rrow := range rows {
		result = append(result, &entities.SatpamWithProfile{
			ID:               rrow.ID,
			Name:             rrow.Name,
			Email:            rrow.Email,
			Role:             rrow.Role,
			Active:           rrow.Active,
			Jabatan:          rrow.Jabatan,
			JenisKelamin:     rrow.JenisKelamin,
			TanggalLahir:     rrow.TanggalLahir,
			TempatLahir:      rrow.TempatLahir,
			NoKTP:            rrow.NoKTP,
			Alamat:           rrow.Alamat,
			NoTelepon:        rrow.NoTelepon,
			Agama:            rrow.Agama,
			StatusPernikahan: rrow.StatusPernikahan,
			Kebangsaan:       rrow.Kebangsaan,
			WorkStartDate:    rrow.WorkStartDate,
		})
	}
	return result, nil
}

func (r *repository) ListAdmins(ctx context.Context, limit, offset int) ([]*entities.User, error) {
	args := []interface{}{"ADMIN"}
	query := `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE role = ?
		ORDER BY name ASC
	`
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	var users []*entities.User
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &users, query, args...); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) GetAdminByID(ctx context.Context, id int64) (*entities.User, error) {
	var user entities.User
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ? AND role = 'ADMIN'
	`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *repository) SetSatpamActive(ctx context.Context, userID int64, active bool) error {
	_, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE users
		SET active = ?
		WHERE id = ? AND role = 'SATPAM'
	`, active, userID)
	return err
}

func (r *repository) UpdateSatpam(ctx context.Context, userID int64, payload *entities.SatpamUpsertPayload) (*entities.SatpamWithProfile, error) {
	tx, err := r.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?, work_start_date = ?, active = ?, updated_at = NOW()
		WHERE id = ? AND role = 'SATPAM'
	`, payload.Name, payload.Email, payload.WorkStartDate, payload.Active, userID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE satpam_profiles
		SET jabatan = ?, jenis_kelamin = ?, tanggal_lahir = ?, tempat_lahir = ?, no_ktp = ?,
		    alamat = ?, no_telepon = ?, agama = ?, status_pernikahan = ?, kebangsaan = ?, work_start_date = ?, updated_at = NOW()
		WHERE user_id = ?
	`, payload.Jabatan, payload.JenisKelamin, payload.TanggalLahir, payload.TempatLahir, payload.NoKTP,
		payload.Alamat, payload.NoTelepon, payload.Agama, payload.StatusPernikahan, payload.Kebangsaan, payload.WorkStartDate, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var row struct {
		ID              int64      `db:"id"`
		Name            string     `db:"name"`
		Email           string     `db:"email"`
		Role            string     `db:"role"`
		Active          bool       `db:"active"`
		Jabatan         string     `db:"jabatan"`
		JenisKelamin    string     `db:"jenis_kelamin"`
		TanggalLahir    *time.Time `db:"tanggal_lahir"`
		TempatLahir     *string    `db:"tempat_lahir"`
		NoKTP           *string    `db:"no_ktp"`
		Alamat          string     `db:"alamat"`
		NoTelepon       string     `db:"no_telepon"`
		Agama           *string    `db:"agama"`
		StatusPernikahan *string   `db:"status_pernikahan"`
		Kebangsaan      *string    `db:"kebangsaan"`
		WorkStartDate   time.Time  `db:"work_start_date"`
	}

	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			u.id, u.name, u.email, u.role, u.active,
			p.jabatan, p.jenis_kelamin, p.tanggal_lahir, p.tempat_lahir, p.no_ktp,
			p.alamat, p.no_telepon, p.agama, p.status_pernikahan, p.kebangsaan,
			p.work_start_date
		FROM users u
		INNER JOIN satpam_profiles p ON p.user_id = u.id
		WHERE u.id = ? AND u.role = 'SATPAM'
	`, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &entities.SatpamWithProfile{
		ID:               row.ID,
		Name:             row.Name,
		Email:            row.Email,
		Role:             row.Role,
		Active:           row.Active,
		Jabatan:          row.Jabatan,
		JenisKelamin:     row.JenisKelamin,
		TanggalLahir:     row.TanggalLahir,
		TempatLahir:      row.TempatLahir,
		NoKTP:            row.NoKTP,
		Alamat:           row.Alamat,
		NoTelepon:        row.NoTelepon,
		Agama:            row.Agama,
		StatusPernikahan: row.StatusPernikahan,
		Kebangsaan:       row.Kebangsaan,
		WorkStartDate:    row.WorkStartDate,
	}, nil
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

func (r *repository) GetShiftIDsByNames(ctx context.Context, names []string) (map[string]int64, error) {
	if len(names) == 0 {
		return map[string]int64{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT id, name
		FROM shifts
		WHERE name IN (?)
	`, names)
	if err != nil {
		return nil, err
	}
	query = r.app.Ds.ReaderDB.Rebind(query)

	type row struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	var rows []row
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, rrow := range rows {
		result[rrow.Name] = rrow.ID
	}
	return result, nil
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

func (r *repository) UpsertUserShift(ctx context.Context, userID, shiftID int64, shiftDate time.Time) (bool, bool, error) {
	res, err := r.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO user_shifts (user_id, shift_id, shift_date, is_swapped)
		VALUES (?, ?, ?, 0)
		ON DUPLICATE KEY UPDATE
			shift_id = VALUES(shift_id),
			is_swapped = VALUES(is_swapped),
			updated_at = CURRENT_TIMESTAMP
	`, userID, shiftID, shiftDate.Format("2006-01-02"))
	if err != nil {
		return false, false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, false, err
	}
	inserted := affected == 1
	updated := affected == 2
	return inserted, updated, nil
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

// Dashboard queries

func (r *repository) GetDashboardSummary(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardSummary, error) {
	var row entities.AdminDashboardSummary
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE role = 'SATPAM' AND active = 1) AS total_security,
			COUNT(us.id) AS scheduled_days,
			COUNT(a.id) AS present_days,
			COALESCE(SUM(CASE WHEN a.clock_in_status = 'ON_TIME' THEN 1 ELSE 0 END), 0) AS on_time_days,
			COUNT(us.id) - COUNT(a.id) AS absent_days,
			COALESCE(SUM(
				CASE WHEN a.clock_in_status IN ('LATE','TOO_LATE') AND a.clock_in_time IS NOT NULL THEN
					GREATEST(
						TIMESTAMPDIFF(
							MINUTE,
							DATE_ADD(TIMESTAMP(a.attendance_date, s.start_time), INTERVAL s.late_tolerance_minute MINUTE),
							a.clock_in_time
						),
						0
					)
				ELSE 0 END
			), 0) AS total_late_minutes,
			COALESCE(SUM(CASE WHEN a.clock_in_status IN ('LATE','TOO_LATE') THEN 1 ELSE 0 END), 0) AS late_records,
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() THEN 1 ELSE 0 END), 0) AS past_scheduled_days,
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() AND a.id IS NOT NULL THEN 1 ELSE 0 END), 0) AS past_present_days,
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() AND a.clock_in_status = 'ON_TIME' THEN 1 ELSE 0 END), 0) AS past_on_time_days
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id AND u.role = 'SATPAM'
		INNER JOIN shifts s ON s.id = us.shift_id AND s.name <> 'Libur'
		LEFT JOIN attendance a
			ON a.user_id = us.user_id AND a.attendance_date = us.shift_date AND a.shift_id = us.shift_id
		WHERE us.shift_date BETWEEN ? AND ?
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); err != nil {
		if err == sql.ErrNoRows {
			return &entities.AdminDashboardSummary{}, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) GetDashboardTrend(ctx context.Context, startDate, endDate time.Time) ([]*entities.AdminDashboardTrendRow, error) {
	var rows []*entities.AdminDashboardTrendRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			us.shift_date AS shift_date,
			COUNT(us.id) AS scheduled,
			COUNT(a.id) AS present,
			SUM(CASE WHEN a.clock_in_status IN ('LATE','TOO_LATE') THEN 1 ELSE 0 END) AS late
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id AND u.role = 'SATPAM'
		INNER JOIN shifts s ON s.id = us.shift_id AND s.name <> 'Libur'
		LEFT JOIN attendance a
			ON a.user_id = us.user_id AND a.attendance_date = us.shift_date AND a.shift_id = us.shift_id
		WHERE us.shift_date BETWEEN ? AND ?
		GROUP BY us.shift_date
		ORDER BY us.shift_date ASC
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) GetDashboardDiscipline(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardDisciplineRow, error) {
	var row entities.AdminDashboardDisciplineRow
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() AND a.clock_in_status IN ('LATE','TOO_LATE') THEN 1 ELSE 0 END), 0) AS late,
			COALESCE(SUM(
				CASE WHEN us.shift_date <= CURDATE()
					AND a.clock_out_time IS NOT NULL AND a.clock_in_time IS NOT NULL
					AND (
						CASE
							-- Shift lintas hari (mis. 20:00-08:00): jam selesai di hari berikutnya.
							WHEN s.end_time < s.start_time THEN TIMESTAMP(DATE_ADD(us.shift_date, INTERVAL 1 DAY), s.end_time)
							-- Shift normal (di hari yang sama).
							ELSE TIMESTAMP(us.shift_date, s.end_time)
						END
					) > a.clock_out_time
				THEN 1 ELSE 0 END
			), 0) AS early_leave,
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() AND a.id IS NOT NULL AND a.clock_in_time IS NULL THEN 1 ELSE 0 END), 0) AS no_checkin,
			COALESCE(SUM(CASE WHEN us.shift_date <= CURDATE() AND a.id IS NULL THEN 1 ELSE 0 END), 0) AS missed_shift,
			COALESCE(SUM(CASE WHEN us.shift_date > CURDATE() THEN 1 ELSE 0 END), 0) AS future_scheduled
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id AND u.role = 'SATPAM'
		INNER JOIN shifts s ON s.id = us.shift_id AND s.name <> 'Libur'
		LEFT JOIN attendance a
			ON a.user_id = us.user_id AND a.attendance_date = us.shift_date AND a.shift_id = us.shift_id
		WHERE us.shift_date BETWEEN ? AND ?
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); err != nil {
		if err == sql.ErrNoRows {
			return &entities.AdminDashboardDisciplineRow{}, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) GetDashboardRiskEmployees(ctx context.Context, startDate, endDate time.Time, limit int) ([]*entities.AdminDashboardRiskRow, error) {
	var rows []*entities.AdminDashboardRiskRow
	args := []interface{}{startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}
	query := `
		SELECT
			u.id AS user_id,
			u.name AS user_name,
			COALESCE(p.jabatan, 'Satpam') AS position,
			SUM(CASE WHEN a.clock_in_status IN ('LATE','TOO_LATE') THEN 1 ELSE 0 END) AS late_count,
			SUM(CASE WHEN a.id IS NULL THEN 1 ELSE 0 END) AS absent_count,
			SUM(CASE WHEN a.id IS NOT NULL AND a.clock_in_time IS NULL THEN 1 ELSE 0 END) AS no_checkin_count,
			SUM(CASE WHEN a.id IS NULL THEN 1 ELSE 0 END) AS missed_shift_count
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id AND u.role = 'SATPAM'
		LEFT JOIN satpam_profiles p ON p.user_id = u.id
		INNER JOIN shifts s ON s.id = us.shift_id AND s.name <> 'Libur'
		LEFT JOIN attendance a
			ON a.user_id = us.user_id AND a.attendance_date = us.shift_date AND a.shift_id = us.shift_id
		WHERE us.shift_date BETWEEN ? AND ? AND us.shift_date <= CURDATE()
		GROUP BY u.id, u.name
		HAVING late_count > 0 OR absent_count > 0 OR no_checkin_count > 0 OR missed_shift_count > 0
		ORDER BY (late_count * 2 + absent_count * 5 + no_checkin_count * 4 + missed_shift_count * 3) DESC
	`
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) GetDashboardConsistency(ctx context.Context, startDate, endDate time.Time) ([]*entities.AdminDashboardConsistencyRow, error) {
	var rows []*entities.AdminDashboardConsistencyRow
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT
			us.user_id AS user_id,
			COUNT(us.id) AS scheduled,
			COUNT(a.id) AS present
		FROM user_shifts us
		INNER JOIN users u ON u.id = us.user_id AND u.role = 'SATPAM'
		INNER JOIN shifts s ON s.id = us.shift_id AND s.name <> 'Libur'
		LEFT JOIN attendance a
			ON a.user_id = us.user_id AND a.attendance_date = us.shift_date AND a.shift_id = us.shift_id
		WHERE us.shift_date BETWEEN ? AND ? AND us.shift_date <= CURDATE()
		GROUP BY us.user_id
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) GetDashboardAudit(ctx context.Context, startDate, endDate time.Time) (*entities.AdminDashboardAuditRow, error) {
	var row entities.AdminDashboardAuditRow
	if err := r.app.Ds.ReaderDB.GetContext(ctx, &row, `
		SELECT
			COALESCE(SUM(CASE WHEN override_by_admin_id IS NOT NULL THEN 1 ELSE 0 END), 0) AS manual_override,
			COALESCE(SUM(
				CASE WHEN clock_in_time IS NOT NULL AND clock_out_time IS NOT NULL
					AND clock_in_photo_url IS NOT NULL AND clock_out_photo_url IS NOT NULL
				THEN 1 ELSE 0 END
			), 0) AS complete_records,
			COALESCE(COUNT(*), 0) AS total_records
		FROM attendance
		WHERE attendance_date BETWEEN ? AND ? AND attendance_date <= CURDATE()
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); err != nil {
		if err == sql.ErrNoRows {
			return &entities.AdminDashboardAuditRow{}, nil
		}
		return nil, err
	}
	return &row, nil
}

// RBAC

func (r *repository) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	var codes []string
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &codes, `
		SELECT permission_code
		FROM user_permissions
		WHERE user_id = ?
		ORDER BY permission_code ASC
	`, userID); err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *repository) ListPermissions(ctx context.Context) ([]*entities.Permission, error) {
	var rows []*entities.Permission
	if err := r.app.Ds.ReaderDB.SelectContext(ctx, &rows, `
		SELECT code, COALESCE(label, code) AS label
		FROM permissions
		ORDER BY code ASC
	`); err != nil {
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

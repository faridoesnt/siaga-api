package admin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/contracts"
	"siaga-api/api/entities"
	"siaga-api/api/models/responses"
	"siaga-api/internal/pkg/face"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	app        *contracts.App
	repo       Repository
	faceClient face.Client
}

func Init(app *contracts.App) contracts.AdminService {
	repo := NewRepository(app)

	bypass := app.Config[constants.FaceVerifyBypass] == "true"
	baseURL := app.Config[constants.FaceServiceURL]
	client := face.New(baseURL, bypass)

	return &Service{
		app:        app,
		repo:       repo,
		faceClient: client,
	}
}

func monthRange(month time.Time) (time.Time, time.Time) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, -1)
	return start, end
}

// Satpam management

func (s *Service) CreateSatpam(ctx context.Context, adminID int64, payload *entities.SatpamUpsertPayload, password string) (*entities.SatpamWithProfile, error) {
	if payload == nil {
		return nil, responses.BadRequest(errors.New("payload is required"))
	}

	email := strings.TrimSpace(payload.Email)
	name := strings.TrimSpace(payload.Name)
	password = strings.TrimSpace(password)
	jabatan := strings.TrimSpace(payload.Jabatan)
	alamat := strings.TrimSpace(payload.Alamat)
	noTelepon := strings.TrimSpace(payload.NoTelepon)

	if email == "" || password == "" || name == "" || jabatan == "" || alamat == "" || noTelepon == "" {
		return nil, responses.BadRequest(errors.New("name, email, password, jabatan, alamat and no_telepon are required"))
	}
	if len(password) < 8 {
		return nil, responses.BadRequest(errors.New("password minimum length is 8 characters"))
	}
	if payload.JenisKelamin != "L" && payload.JenisKelamin != "P" {
		return nil, responses.BadRequest(errors.New("jenis_kelamin must be 'L' or 'P'"))
	}

	payload.Email = email
	payload.Name = name
	payload.Jabatan = jabatan
	payload.Alamat = alamat
	payload.NoTelepon = noTelepon

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	user, err := s.repo.CreateSatpam(ctx, payload, string(hash))
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("email already in use"))
		}
		return nil, responses.InternalServerError(err)
	}

	_ = adminID // reserved for future auditing if needed

	return user, nil
}

func (s *Service) ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.SatpamWithProfile, error) {
	users, err := s.repo.ListSatpam(ctx, active, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return users, nil
}

func (s *Service) UpdateSatpamProfilePhoto(ctx context.Context, adminID, userID int64, photoURL string) (*entities.SatpamWithProfile, error) {
	_ = adminID // reserved for future auditing
	if strings.TrimSpace(photoURL) == "" {
		return nil, responses.BadRequest(errors.New("photo_url is required"))
	}
	user, err := s.repo.UpdateSatpamPhotoFields(ctx, userID, &photoURL, nil)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.NotFound(errors.New("satpam not found"))
	}
	return user, nil
}

func (s *Service) UpdateSatpamKTPPhoto(ctx context.Context, adminID, userID int64, photoURL string) (*entities.SatpamWithProfile, error) {
	_ = adminID // reserved for future auditing
	if strings.TrimSpace(photoURL) == "" {
		return nil, responses.BadRequest(errors.New("photo_url is required"))
	}
	user, err := s.repo.UpdateSatpamPhotoFields(ctx, userID, nil, &photoURL)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.NotFound(errors.New("satpam not found"))
	}
	return user, nil
}

// RBAC / Permissions

func (s *Service) ListPermissions(ctx context.Context) ([]*entities.Permission, error) {
	rows, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return rows, nil
}

func (s *Service) ListAdmins(ctx context.Context, limit, offset int) ([]*entities.User, error) {
	users, err := s.repo.ListAdmins(ctx, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return users, nil
}

func (s *Service) GetAdminWithPermissions(ctx context.Context, id int64) (*entities.User, []string, error) {
	user, err := s.repo.GetAdminByID(ctx, id)
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, nil, responses.NotFound(errors.New("admin not found"))
	}
	perms, err := s.repo.GetUserPermissions(ctx, id)
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}
	return user, perms, nil
}

func (s *Service) validatePermissionCodes(ctx context.Context, codes []string) ([]string, error) {
	if len(codes) == 0 {
		return []string{}, nil
	}
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	allowed := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		allowed[p.Code] = struct{}{}
	}
	unique := make(map[string]struct{})
	var cleaned []string
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := allowed[code]; !ok {
			return nil, responses.BadRequest(fmt.Errorf("unknown permission code: %s", code))
		}
		if _, seen := unique[code]; !seen {
			unique[code] = struct{}{}
			cleaned = append(cleaned, code)
		}
	}
	return cleaned, nil
}

func (s *Service) CreateAdminUser(ctx context.Context, actorID int64, email, password, name string, perms []string) (*entities.User, []string, error) {
	_ = actorID

	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	name = strings.TrimSpace(name)

	if email == "" || password == "" || name == "" {
		return nil, nil, responses.BadRequest(errors.New("name, email and password are required"))
	}
	if len(password) < 8 {
		return nil, nil, responses.BadRequest(errors.New("password minimum length is 8 characters"))
	}

	perms, err := s.validatePermissionCodes(ctx, perms)
	if err != nil {
		return nil, nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (name, email, password_hash, role, active)
		VALUES (?, ?, ?, 'ADMIN', 1)
	`, name, email, string(hash))
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, nil, responses.Conflict(errors.New("email already in use"))
		}
		return nil, nil, responses.InternalServerError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	for _, code := range perms {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_permissions (user_id, permission_code)
			VALUES (?, ?)
		`, id, code); err != nil {
			return nil, nil, responses.InternalServerError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	var user entities.User
	if err := s.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id); err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	return &user, perms, nil
}

func (s *Service) UpdateAdminUser(ctx context.Context, actorID, id int64, email, name string, perms []string) (*entities.User, []string, error) {
	_ = actorID

	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if email == "" || name == "" {
		return nil, nil, responses.BadRequest(errors.New("name and email are required"))
	}

	perms, err := s.validatePermissionCodes(ctx, perms)
	if err != nil {
		return nil, nil, err
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, responses.InternalServerError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?
		WHERE id = ? AND role = 'ADMIN'
	`, name, email, id); err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, nil, responses.Conflict(errors.New("email already in use"))
		}
		return nil, nil, responses.InternalServerError(err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM user_permissions WHERE user_id = ?
	`, id); err != nil {
		return nil, nil, responses.InternalServerError(err)
	}
	for _, code := range perms {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_permissions (user_id, permission_code)
			VALUES (?, ?)
		`, id, code); err != nil {
			return nil, nil, responses.InternalServerError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	var user entities.User
	if err := s.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE id = ? AND role = 'ADMIN'
	`, id); err != nil {
		return nil, nil, responses.InternalServerError(err)
	}

	return &user, perms, nil
}

func (s *Service) DeleteAdminUser(ctx context.Context, actorID, id int64) error {
	_ = actorID

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return responses.InternalServerError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM user_permissions WHERE user_id = ?
	`, id); err != nil {
		return responses.InternalServerError(err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM users WHERE id = ? AND role = 'ADMIN'
	`, id); err != nil {
		return responses.InternalServerError(err)
	}

	if err := tx.Commit(); err != nil {
		return responses.InternalServerError(err)
	}

	return nil
}

func (s *Service) ResetAdminPassword(ctx context.Context, actorID, id int64, newPassword string) error {
	_ = actorID

	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 {
		return responses.BadRequest(errors.New("password minimum length is 8 characters"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return responses.InternalServerError(err)
	}

	if _, err := s.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?
		WHERE id = ? AND role = 'ADMIN'
	`, string(hash), id); err != nil {
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) SetSatpamActive(ctx context.Context, adminID, userID int64, active bool) error {
	if adminID == userID && !active {
		return responses.Forbidden(errors.New("cannot disable self"))
	}
	if err := s.repo.SetSatpamActive(ctx, userID, active); err != nil {
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) UpdateSatpam(ctx context.Context, adminID, userID int64, payload *entities.SatpamUpsertPayload) (*entities.SatpamWithProfile, error) {
	if payload == nil {
		return nil, responses.BadRequest(errors.New("payload is required"))
	}

	email := strings.TrimSpace(payload.Email)
	name := strings.TrimSpace(payload.Name)
	jabatan := strings.TrimSpace(payload.Jabatan)
	alamat := strings.TrimSpace(payload.Alamat)
	noTelepon := strings.TrimSpace(payload.NoTelepon)

	if email == "" || name == "" || jabatan == "" || alamat == "" || noTelepon == "" {
		return nil, responses.BadRequest(errors.New("name, email, jabatan, alamat and no_telepon are required"))
	}
	if payload.JenisKelamin != "L" && payload.JenisKelamin != "P" {
		return nil, responses.BadRequest(errors.New("jenis_kelamin must be 'L' or 'P'"))
	}

	payload.Email = email
	payload.Name = name
	payload.Jabatan = jabatan
	payload.Alamat = alamat
	payload.NoTelepon = noTelepon

	user, err := s.repo.UpdateSatpam(ctx, userID, payload)
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("email already in use"))
		}
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.NotFound(errors.New("satpam not found"))
	}

	_ = adminID

	return user, nil
}

func (s *Service) DeleteSatpam(ctx context.Context, adminID, userID int64) error {
	if adminID == userID {
		return responses.Forbidden(errors.New("cannot delete self"))
	}
	if err := s.repo.DeleteSatpam(ctx, userID); err != nil {
		if responses.IsForeignKeyErr(err) {
			return responses.Conflict(errors.New("satpam is in use"))
		}
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) ResetSatpamPassword(ctx context.Context, adminID, userID int64, newPassword string) error {
	if adminID == userID {
		return responses.Forbidden(errors.New("cannot reset own password via this endpoint"))
	}

	user, err := s.repo.GetSatpamByID(ctx, userID)
	if err != nil {
		return responses.InternalServerError(err)
	}
	if user == nil {
		return responses.NotFound(errors.New("satpam not found"))
	}

	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" {
		return responses.BadRequest(errors.New("new password is required"))
	}
	if len(newPassword) < 8 {
		return responses.BadRequest(errors.New("password minimum length is 8 characters"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return responses.InternalServerError(err)
	}

	if _, err := s.app.Ds.WriterDB.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?
		WHERE id = ? AND role = 'SATPAM'
	`, string(hash), userID); err != nil {
		return responses.InternalServerError(err)
	}

	return nil
}

// Attendance spots

func (s *Service) CreateAttendanceSpot(ctx context.Context, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error) {
	if name == "" {
		return nil, responses.BadRequest(errors.New("name is required"))
	}
	if radiusMeters <= 0 {
		return nil, responses.BadRequest(errors.New("radius_meter must be > 0"))
	}
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, responses.BadRequest(errors.New("invalid latitude/longitude"))
	}

	spot, err := s.repo.CreateAttendanceSpot(ctx, name, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return spot, nil
}

func (s *Service) ListAttendanceSpots(ctx context.Context, limit, offset int) ([]*entities.AttendanceSpot, error) {
	spots, err := s.repo.ListAttendanceSpots(ctx, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return spots, nil
}

func (s *Service) AssignUserAttendanceSpot(ctx context.Context, adminID, userID, attendanceSpotID int64, activeFrom time.Time) error {
	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := s.repo.AssignUserAttendanceSpot(ctx, tx, userID, attendanceSpotID, activeFrom); err != nil {
		return responses.InternalServerError(err)
	}

	if err := tx.Commit(); err != nil {
		return responses.InternalServerError(err)
	}

	_ = adminID // reserved for auditing if needed

	return nil
}

// Shifts

func (s *Service) CreateShift(ctx context.Context, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error) {
	if name == "" {
		return nil, responses.BadRequest(errors.New("name is required"))
	}
	if lateTolerance < 0 {
		return nil, responses.BadRequest(errors.New("late_tolerance_minute must be >= 0"))
	}
	shift, err := s.repo.CreateShift(ctx, name, startTime, endTime, lateTolerance)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return shift, nil
}

func (s *Service) ListShifts(ctx context.Context, limit, offset int) ([]*entities.Shift, error) {
	shifts, err := s.repo.ListShifts(ctx, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return shifts, nil
}

func (s *Service) AssignUserShift(ctx context.Context, adminID, userID, shiftID int64, shiftDate time.Time) (*entities.UserShift, error) {
	hasAttendance, err := s.repo.HasAttendanceForDate(ctx, userID, shiftDate)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if hasAttendance {
		return nil, responses.BadRequest(errors.New("attendance already exists for that date"))
	}

	us, err := s.repo.AssignUserShift(ctx, userID, shiftID, shiftDate)
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("shift already assigned for that date"))
		}
		return nil, responses.InternalServerError(err)
	}

	_ = adminID // reserved for auditing if needed

	return us, nil
}

// Shift swaps

func (s *Service) ListShiftSwapRequests(ctx context.Context, status string, limit, offset int) ([]*entities.ShiftSwapRequest, error) {
	reqs, err := s.repo.ListShiftSwapRequests(ctx, status, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return reqs, nil
}

func (s *Service) ApproveShiftSwapRequest(ctx context.Context, adminID, requestID int64) (*entities.ShiftSwapRequest, error) {
	return nil, responses.BadRequest(errors.New("manual shift swap approval is disabled"))
}

func (s *Service) RejectShiftSwapRequest(ctx context.Context, adminID, requestID int64, note string) (*entities.ShiftSwapRequest, error) {
	return nil, responses.BadRequest(errors.New("manual shift swap rejection is disabled"))
}

// Attendance monitoring

func (s *Service) ListDailyAttendance(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceRow, error) {
	rows, err := s.repo.ListDailyAttendance(ctx, date)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	activities, err := s.repo.ListDailyActivityPhotos(ctx, date)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	activityByAttendance := make(map[int64][]*entities.AdminAttendanceActivityRow)
	for _, a := range activities {
		activityByAttendance[a.AttendanceID] = append(activityByAttendance[a.AttendanceID], a)
	}

	for _, r := range rows {
		if acts, ok := activityByAttendance[r.AttendanceID]; ok {
			r.Activities = acts
		}
	}

	return rows, nil
}

func (s *Service) ListOpenAttendance(ctx context.Context) ([]*entities.AdminAttendanceRow, error) {
	rows, err := s.repo.ListOpenAttendance(ctx)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return rows, nil
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (s *Service) ListUserShifts(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserShiftRow, error) {
	rows, err := s.repo.ListUserShifts(ctx, date, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return rows, nil
}

func (s *Service) ListUserAttendanceSpots(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserAttendanceSpotRow, error) {
	rows, err := s.repo.ListUserAttendanceSpots(ctx, date, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return rows, nil
}

func (s *Service) UpdateAttendanceSpot(ctx context.Context, id int64, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error) {
	if name == "" {
		return nil, responses.BadRequest(errors.New("name is required"))
	}
	if radiusMeters <= 0 {
		return nil, responses.BadRequest(errors.New("radius_meter must be > 0"))
	}
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, responses.BadRequest(errors.New("invalid latitude/longitude"))
	}
	spot, err := s.repo.UpdateAttendanceSpot(ctx, id, name, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return spot, nil
}

func (s *Service) DeleteAttendanceSpot(ctx context.Context, id int64) error {
	if err := s.repo.DeleteAttendanceSpot(ctx, id); err != nil {
		if responses.IsForeignKeyErr(err) {
			return responses.Conflict(errors.New("attendance spot is in use"))
		}
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) UpdateShift(ctx context.Context, id int64, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error) {
	if name == "" {
		return nil, responses.BadRequest(errors.New("name is required"))
	}
	if lateTolerance < 0 {
		return nil, responses.BadRequest(errors.New("late_tolerance_minute must be >= 0"))
	}
	shift, err := s.repo.UpdateShift(ctx, id, name, startTime, endTime, lateTolerance)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return shift, nil
}

func (s *Service) DeleteShift(ctx context.Context, id int64) error {
	if err := s.repo.DeleteShift(ctx, id); err != nil {
		if responses.IsForeignKeyErr(err) {
			return responses.Conflict(errors.New("shift is in use"))
		}
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) UpdateUserShiftAssignment(ctx context.Context, adminID, id, shiftID int64, shiftDate time.Time) (*entities.UserShift, error) {
	us, err := s.repo.UpdateUserShiftAssignment(ctx, id, shiftID, shiftDate)
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("shift already assigned for that date"))
		}
		return nil, responses.InternalServerError(err)
	}

	_ = adminID

	return us, nil
}

func (s *Service) DeleteUserShiftAssignment(ctx context.Context, adminID, id int64) error {
	if err := s.repo.DeleteUserShiftAssignment(ctx, id); err != nil {
		if responses.IsForeignKeyErr(err) {
			return responses.Conflict(errors.New("user shift assignment is in use"))
		}
		return responses.InternalServerError(err)
	}
	_ = adminID
	return nil
}

func (s *Service) UpdateUserAttendanceSpot(ctx context.Context, adminID, id, attendanceSpotID int64, activeFrom time.Time, activeUntil *time.Time) (*entities.UserAttendanceSpot, error) {
	uas, err := s.repo.UpdateUserAttendanceSpot(ctx, id, attendanceSpotID, activeFrom, activeUntil)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	_ = adminID
	return uas, nil
}

func (s *Service) DeleteUserAttendanceSpot(ctx context.Context, adminID, id int64) error {
	if err := s.repo.DeleteUserAttendanceSpot(ctx, id); err != nil {
		if responses.IsForeignKeyErr(err) {
			return responses.Conflict(errors.New("user attendance spot assignment is in use"))
		}
		return responses.InternalServerError(err)
	}
	_ = adminID
	return nil
}

// Face enrollment

func (s *Service) EnrollFace(ctx context.Context, adminID, userID int64, images []string) error {
	if len(images) == 0 {
		return responses.BadRequest(errors.New("images is required"))
	}
	_ = adminID

	user, err := s.repo.GetSatpamByID(ctx, userID)
	if err != nil {
		return responses.InternalServerError(err)
	}
	if user == nil {
		return responses.BadRequest(errors.New("user is not a SATPAM or does not exist"))
	}

	embs, err := s.faceClient.Enroll(ctx, userID, images)
	if err != nil {
		return responses.InternalServerError(err)
	}
	if len(embs) == 0 {
		return responses.BadRequest(errors.New("no face embeddings returned"))
	}

	vectors := make([]string, 0, len(embs))
	model := embs[0].Model
	for _, e := range embs {
		if e.Vector != "" {
			vectors = append(vectors, e.Vector)
		}
	}
	if len(vectors) == 0 {
		return responses.BadRequest(errors.New("no valid embeddings"))
	}

	if err := s.repo.ReplaceFaceEmbeddings(ctx, userID, vectors, model); err != nil {
		return responses.InternalServerError(err)
	}

	return nil
}

func (s *Service) GetFaceEnrollStatus(ctx context.Context, adminID, userID int64) (*entities.FaceEmbeddingSummary, error) {
	_ = adminID

	user, err := s.repo.GetSatpamByID(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.BadRequest(errors.New("user is not a SATPAM or does not exist"))
	}

	sum, err := s.repo.GetFaceEmbeddingSummary(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return sum, nil
}

func (s *Service) DeleteFaceEnroll(ctx context.Context, adminID, userID int64) error {
	_ = adminID

	user, err := s.repo.GetSatpamByID(ctx, userID)
	if err != nil {
		return responses.InternalServerError(err)
	}
	if user == nil {
		return responses.BadRequest(errors.New("user is not a SATPAM or does not exist"))
	}

	if err := s.repo.ReplaceFaceEmbeddings(ctx, userID, []string{}, ""); err != nil {
		return responses.InternalServerError(err)
	}
	return nil
}

func (s *Service) ForceClockOutAttendance(ctx context.Context, adminID, attendanceID int64, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return responses.BadRequest(errors.New("reason is required"))
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var att entities.Attendance
	err = tx.GetContext(ctx, &att, `
		SELECT
			id, user_id, shift_id, attendance_spot_id,
			clock_in_spot_id, clock_out_spot_id,
			attendance_date, clock_in_time, clock_in_latitude, clock_in_longitude,
			clock_in_status, clock_in_photo_url, face_verified, face_match_score,
			clock_out_time, clock_out_latitude, clock_out_longitude, clock_out_photo_url,
			override_by_admin_id, override_at, override_reason,
			created_at, updated_at
		FROM attendance
		WHERE id = ?
		FOR UPDATE
	`, attendanceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return responses.NotFound(errors.New("attendance not found"))
		}
		return responses.InternalServerError(err)
	}

	if att.ClockOutTime != nil {
		return responses.Conflict(errors.New("attendance already clocked out"))
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		UPDATE attendance
		SET clock_out_time = ?, override_by_admin_id = ?, override_at = ?, override_reason = ?, updated_at = NOW()
		WHERE id = ?
	`, now, adminID, now, reason, attendanceID)
	if err != nil {
		return responses.InternalServerError(err)
	}

	if err := tx.Commit(); err != nil {
		return responses.InternalServerError(err)
	}

	log.Printf("attendance force clock-out: attendance_id=%d admin_id=%d reason=%s", attendanceID, adminID, reason)

	return nil
}

func (s *Service) GetDashboard(ctx context.Context, month time.Time) (*entities.AdminDashboardResponse, error) {
	start, end := monthRange(month)

	// Current month aggregates
	summaryRow, err := s.repo.GetDashboardSummary(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	trendRows, err := s.repo.GetDashboardTrend(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	discRow, err := s.repo.GetDashboardDiscipline(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	riskRows, err := s.repo.GetDashboardRiskEmployees(ctx, start, end, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	consRows, err := s.repo.GetDashboardConsistency(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	auditRow, err := s.repo.GetDashboardAudit(ctx, start, end)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	// Previous month summary for KPI deltas / insight
	prevMonth := month.AddDate(0, -1, 0)
	prevStart, prevEnd := monthRange(prevMonth)
	prevSummaryRow, err := s.repo.GetDashboardSummary(ctx, prevStart, prevEnd)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	resp := &entities.AdminDashboardResponse{}

	// Summary calculations
	resp.Summary.TotalSecurity = summaryRow.TotalSecurity
	// Gunakan hanya shift yang sudah lewat untuk menghitung rate.
	pastScheduled := summaryRow.PastScheduledDays
	pastPresent := summaryRow.PastPresentDays
	pastOnTime := summaryRow.PastOnTimeDays
	if pastScheduled > 0 {
		resp.Summary.AttendanceRate = float64(pastPresent) / float64(pastScheduled) * 100
		resp.Summary.AbsentRate = float64(pastScheduled-pastPresent) / float64(pastScheduled) * 100
	}
	if pastPresent > 0 {
		resp.Summary.OnTimeRate = float64(pastOnTime) / float64(pastPresent) * 100
	}
	if summaryRow.LateRecords > 0 {
		resp.Summary.AvgLateMinutes = summaryRow.TotalLateMin / float64(summaryRow.LateRecords)
	}

	for _, r := range trendRows {
		resp.AttendanceTrend.Labels = append(resp.AttendanceTrend.Labels, r.Date)
		resp.AttendanceTrend.Present = append(resp.AttendanceTrend.Present, r.Present)
		resp.AttendanceTrend.Late = append(resp.AttendanceTrend.Late, r.Late)

		// Delegasikan penentuan "future" ke database via kolom is_future
		// yang berbasis CURDATE() di timezone DB (WIB).
		if r.IsFuture {
			resp.AttendanceTrend.Absent = append(resp.AttendanceTrend.Absent, 0)
			resp.AttendanceTrend.BelumAbsen = append(resp.AttendanceTrend.BelumAbsen, r.Scheduled)
		} else {
			absent := r.Scheduled - r.Present
			if absent < 0 {
				absent = 0
			}
			resp.AttendanceTrend.Absent = append(resp.AttendanceTrend.Absent, absent)
			resp.AttendanceTrend.BelumAbsen = append(resp.AttendanceTrend.BelumAbsen, 0)
		}
	}

	// Discipline breakdown
	resp.DisciplineBreakdown.Late = discRow.Late
	resp.DisciplineBreakdown.EarlyLeave = discRow.EarlyLeave
	resp.DisciplineBreakdown.NoCheckin = discRow.NoCheckin
	resp.DisciplineBreakdown.MissedShift = discRow.MissedShift
	resp.DisciplineBreakdown.BelumAbsen = discRow.FutureShift

	// Index risk rows by user for later guard summary.
	riskByUser := make(map[int64]*entities.AdminDashboardRiskRow, len(riskRows))
	for _, r := range riskRows {
		riskByUser[r.UserID] = r
	}

	// Risk employees (top N by score)
	const topRiskLimit = 10
	for idx, r := range riskRows {
		if idx >= topRiskLimit {
			break
		}
		absent := r.AbsentCount
		// Simplified risk score: fokus ke late, absent, dan missed shift.
		riskScore := float64(r.LateCount*2 + absent*5 + r.MissedShiftCount*3)
		// Determine dominant factor (use codes; FE handles human-readable text).
		maxVal := r.LateCount
		reason := "LATE"
		if absent > maxVal {
			maxVal = absent
			reason = "ABSENT"
		}
		if r.MissedShiftCount > maxVal {
			maxVal = r.MissedShiftCount
			reason = "MISSED_SHIFT"
		}

		resp.RiskEmployees = append(resp.RiskEmployees, struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			Position   string  `json:"position"`
			RiskScore  float64 `json:"risk_score"`
			RiskReason string  `json:"risk_reason"`
		}{
			ID:         fmt.Sprintf("%d", r.UserID),
			Name:       r.UserName,
			Position:   r.Position,
			RiskScore:  riskScore,
			RiskReason: reason,
		})
	}

	// Guard-level summary for all satpam yang memiliki scheduling
	for _, c := range consRows {
		if c.Scheduled == 0 {
			continue
		}
		absent := c.Scheduled - c.Present
		if absent < 0 {
			absent = 0
		}
		late := 0
		missed := 0
		if r, ok := riskByUser[c.UserID]; ok {
			late = r.LateCount
			missed = r.MissedShiftCount
		}
		riskScore := float64(late*2 + absent*5 + missed*3)

		resp.GuardSummary = append(resp.GuardSummary, entities.AdminDashboardGuardSummaryRow{
			ID:        fmt.Sprintf("%d", c.UserID),
			Name:      c.UserName,
			Position:  c.Position,
			Scheduled: c.Scheduled,
			Present:   c.Present,
			Absent:    absent,
			Late:      late,
			RiskScore: riskScore,
		})
	}

	// Sort guard summary by risk score desc, then name asc.
	if len(resp.GuardSummary) > 0 {
		sort.Slice(resp.GuardSummary, func(i, j int) bool {
			if resp.GuardSummary[i].RiskScore == resp.GuardSummary[j].RiskScore {
				return resp.GuardSummary[i].Name < resp.GuardSummary[j].Name
			}
			return resp.GuardSummary[i].RiskScore > resp.GuardSummary[j].RiskScore
		})
	}

	// Attendance consistency (aggregate counts)
	totalUsers := 0
	consistent := 0
	var totalStreak float64
	for _, r := range consRows {
		if r.Scheduled == 0 {
			continue
		}
		totalUsers++
		presentRate := float64(r.Present) / float64(r.Scheduled)
		if presentRate >= 0.9 {
			consistent++
		}
		totalStreak += float64(r.Present)
	}
	resp.AttendanceConsistency.Consistent = consistent
	if totalUsers > 0 {
		resp.AttendanceConsistency.Irregular = totalUsers - consistent
		resp.AttendanceConsistency.AvgStreakDays = totalStreak / float64(totalUsers)
	}

	// Audit compliance
	resp.AuditCompliance.ManualOverride = auditRow.ManualOverride
	if auditRow.TotalRecords > 0 {
		resp.AuditCompliance.DataCompleteness = float64(auditRow.CompleteRecords) / float64(auditRow.TotalRecords) * 100
	}

	// KPI cards with deltas and status
	currAttendanceRate := resp.Summary.AttendanceRate
	currOnTimeRate := resp.Summary.OnTimeRate
	currAbsentRate := resp.Summary.AbsentRate
	currAvgLate := resp.Summary.AvgLateMinutes

	var prevAttendanceRate, prevOnTimeRate, prevAbsentRate, prevAvgLate float64
	if prevSummaryRow.ScheduledDays > 0 {
		prevAttendanceRate = float64(prevSummaryRow.PresentDays) / float64(prevSummaryRow.ScheduledDays) * 100
		prevAbsentRate = float64(prevSummaryRow.AbsentDays) / float64(prevSummaryRow.ScheduledDays) * 100
	}
	if prevSummaryRow.PresentDays > 0 {
		prevOnTimeRate = float64(prevSummaryRow.OnTimeDays) / float64(prevSummaryRow.PresentDays) * 100
	}
	if prevSummaryRow.LateRecords > 0 {
		prevAvgLate = prevSummaryRow.TotalLateMin / float64(prevSummaryRow.LateRecords)
	}

	resp.KPIs = append(resp.KPIs,
		buildKPI("Attendance Rate", currAttendanceRate, prevAttendanceRate),
		buildKPI("On-Time Rate", currOnTimeRate, prevOnTimeRate),
		buildKPI("Absent Rate", currAbsentRate, prevAbsentRate),
		buildKPI("Avg Late Minutes", currAvgLate, prevAvgLate),
	)

	// Hero insight derived from summary and risk employees
	// resp.HeroInsight = buildHeroInsight(month, currAttendanceRate, currOnTimeRate, resp.KPIs, len(resp.RiskEmployees))

	return resp, nil
}

func buildKPI(label string, value, prevValue float64) entities.AdminDashboardKPI {
	delta := value - prevValue

	trend := "flat"
	const epsilon = 0.1
	if delta > epsilon {
		trend = "up"
	} else if delta < -epsilon {
		trend = "down"
	}

	status := "good"
	switch label {
	case "Attendance Rate":
		// Good if >= 95, warning if >= 90, else bad
		if value < 90 {
			status = "bad"
		} else if value < 95 {
			status = "warning"
		}
	case "On-Time Rate":
		// Good if >= 90, warning if >= 80, else bad
		if value < 80 {
			status = "bad"
		} else if value < 90 {
			status = "warning"
		}
	case "Absent Rate":
		// Lower is better: good <= 3, warning <= 7, else bad
		if value > 7 {
			status = "bad"
		} else if value > 3 {
			status = "warning"
		}
	case "Avg Late Minutes":
		// Lower is better: good <= 5, warning <= 10, else bad
		if value > 10 {
			status = "bad"
		} else if value > 5 {
			status = "warning"
		}
	}

	return entities.AdminDashboardKPI{
		Label:  label,
		Value:  value,
		Delta:  delta,
		Trend:  trend,
		Status: status,
	}
}

// func buildHeroInsight(month time.Time, attendanceRate, onTimeRate float64, kpis []entities.AdminDashboardKPI, riskCount int) entities.AdminDashboardHeroInsight {
// 	severity := "normal"
// 	if attendanceRate < 80 || riskCount >= 3 {
// 		severity = "critical"
// 	} else if attendanceRate < 90 || riskCount >= 1 {
// 		severity = "warning"
// 	}

// 	var headline string
// 	var context string

// 	monthLabel := month.Format("January 2006")

// 	if severity == "critical" {
// 		headline = fmt.Sprintf("Attendance turun dan perlu tindakan segera di %s.", monthLabel)
// 		context = fmt.Sprintf("Rate kehadiran hanya %.1f%% dengan %d satpam berisiko. Fokuskan review pada jadwal dan disiplin shift yang sering terlambat.", attendanceRate, riskCount)
// 	} else if severity == "warning" {
// 		headline = fmt.Sprintf("Attendance di %s memerlukan perhatian manajemen.", monthLabel)
// 		context = fmt.Sprintf("Rate kehadiran %.1f%% dan on-time %.1f%%. Lakukan coaching pada satpam dengan pola keterlambatan atau ketidakhadiran berulang.", attendanceRate, onTimeRate)
// 	} else {
// 		headline = fmt.Sprintf("Attendance di %s berada pada level stabil.", monthLabel)
// 		context = fmt.Sprintf("Rate kehadiran %.1f%% dan on-time %.1f%%. Pertahankan pola jadwal dan monitoring berkala.", attendanceRate, onTimeRate)
// 	}

// 	// Enrich context with the most concerning KPI if available
// 	if len(kpis) > 0 {
// 		var worst entities.AdminDashboardKPI
// 		for i, k := range kpis {
// 			if i == 0 || k.Status == "bad" || (k.Status == "warning" && worst.Status == "good") {
// 				worst = k
// 			}
// 		}
// 		if worst.Status != "" {
// 			context = fmt.Sprintf("%s Fokus khusus pada indikator %s (%.1f, Δ%.1f).", context, worst.Label, worst.Value, worst.Delta)
// 		}
// 	}

// 	return entities.AdminDashboardHeroInsight{
// 		Headline: headline,
// 		Severity: severity,
// 		Context:  context,
// 	}
// }

// Import / Export helpers

func setPrettyHeaderStyle(f *excelize.File, sheet string, lastCol int) error {
	colName, err := excelize.ColumnNumberToName(lastCol)
	if err != nil {
		return err
	}
	styleID, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#111827", // dark gray text
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E5E7EB"}, // light gray background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "bottom", Color: "DDDDDD", Style: 1},
		},
	})
	if err != nil {
		return err
	}
	return f.SetCellStyle(sheet, "A1", colName+"1", styleID)
}

func parseFlexibleDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty date")
	}

	// --------------------------------------------------
	// 1. Excel serial date (1900-based & 1904-based)
	// --------------------------------------------------
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		if n > 0 {
			// try 1900 system
			if t, err := excelize.ExcelDateToTime(n, false); err == nil {
				return t.In(time.Local), nil
			}
			// try 1904 system (rare but exists)
			if t, err := excelize.ExcelDateToTime(n, true); err == nil {
				return t.In(time.Local), nil
			}
		}
	}

	// --------------------------------------------------
	// 2. Remove time part safely
	// --------------------------------------------------
	// Examples:
	// 2025-01-01 00:00:00
	// 01/02/2025 12:30
	reTime := regexp.MustCompile(`\s+\d{1,2}:\d{2}(:\d{2})?`)
	value = reTime.ReplaceAllString(value, "")

	// Normalize separators
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.ReplaceAll(value, "-", "/")
	value = strings.ReplaceAll(value, ".", "/")
	value = strings.TrimSpace(value)

	// --------------------------------------------------
	// 3. Handle slash date explicitly (dd/mm/yy, dd/mm/yyyy)
	// --------------------------------------------------
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		if len(parts) == 3 {
			a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			c, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))

			if err1 == nil && err2 == nil && err3 == nil {
				day, month, year := 0, 0, c

				// Fix 2-digit year
				if year < 100 {
					if year >= 70 {
						year += 1900
					} else {
						year += 2000
					}
				}

				// 🔥 STRONG RULES (NO AMBIGUITY)
				switch {
				case a > 12:
					// 25/02/00 → dd/mm/yy
					day, month = a, b

				case b > 12:
					// 02/25/00 → mm/dd/yy
					day, month = b, a

				default:
					// Ambiguous → force dd/mm (Indonesia default)
					day, month = a, b
				}

				if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
					return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), nil
				}
			}
		}
	}

	// --------------------------------------------------
	// 4. Textual & ISO layouts
	// --------------------------------------------------
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"02 Jan 2006",
		"2 Jan 2006",
		"02-Jan-2006",
		"2-Jan-2006",
		"02 Jan 06",
		"2 Jan 06",
		"02-Jan-06",
		"2-Jan-06",
		"Jan 2 2006",
		"Jan 2, 2006",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid excel date format: %q", value)
}

func (s *Service) GenerateSatpamImportTemplate(ctx context.Context) ([]byte, error) {
	_ = ctx

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	// --------------------------------------------------
	// Headers
	// --------------------------------------------------
	headers := []string{
		"Nama",
		"Email",
		"Password",
		"Aktif (TRUE/FALSE)",
		"Jabatan",
		"Jenis Kelamin (L/P)",
		"Alamat",
		"No Telepon",
		"Tanggal Mulai Kerja (YYYY-MM-DD)",
		"Tanggal Lahir (YYYY-MM-DD)",
		"Tempat Lahir",
		"No KTP",
		"Agama",
		"Status Pernikahan",
		"Kebangsaan",
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	// --------------------------------------------------
	// Styles
	// --------------------------------------------------
	// Header style (existing helper)
	_ = setPrettyHeaderStyle(f, sheet, len(headers))

	// Date style: yyyy-mm-dd (Excel native)
	dateStyle, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"yyyy-mm-dd"}[0],
	})
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	// Apply DATE style to date columns
	_ = f.SetCellStyle(sheet, "I2", "I500", dateStyle)
	_ = f.SetCellStyle(sheet, "J2", "J500", dateStyle)

	// Layout
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "N", 24)

	// --------------------------------------------------
	// Data Validation (Prevent wrong input)
	// --------------------------------------------------
	// Date validation (I & J columns)
	_ = f.AddDataValidation(sheet, &excelize.DataValidation{
		Type:     "date",
		Sqref:    "I2:I1048576",
		Operator: "between",
		Formula1: "DATE(1900,1,1)",
		Formula2: "DATE(2100,12,31)",
	})

	_ = f.AddDataValidation(sheet, &excelize.DataValidation{
		Type:     "date",
		Sqref:    "J2:J1048576",
		Operator: "between",
		Formula1: "DATE(1900,1,1)",
		Formula2: "DATE(2100,12,31)",
	})

	// --------------------------------------------------
	// Export
	// --------------------------------------------------
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	return buf.Bytes(), nil
}

func (s *Service) GenerateShiftImportTemplate(ctx context.Context) ([]byte, error) {
	_ = ctx

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Nama Shift",
		"Jam Mulai (HH:mm)",
		"Jam Selesai (HH:mm)",
		"Toleransi Keterlambatan (menit)",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	_ = setPrettyHeaderStyle(f, sheet, len(headers))
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "D", 26)

	_ = f.SetCellValue(sheet, "A2", "#Pagi (hapus baris ini)")
	_ = f.SetCellValue(sheet, "B2", "07:00")
	_ = f.SetCellValue(sheet, "C2", "16:00")
	_ = f.SetCellValue(sheet, "D2", 10)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

func (s *Service) ImportSatpamFromExcel(ctx context.Context, adminID int64, fileData []byte) (int, error) {
	_ = adminID

	f, err := excelize.OpenReader(bytes.NewReader(fileData))
	if err != nil {
		return 0, responses.BadRequest(errors.New("invalid Excel file"))
	}
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, responses.BadRequest(errors.New("failed to read Excel rows"))
	}
	if len(rows) < 2 {
		return 0, responses.BadRequest(errors.New("template must include header and at least one data row"))
	}

	expectedHeaders := []string{
		"Nama",
		"Email",
		"Password",
		"Aktif (TRUE/FALSE)",
		"Jabatan",
		"Jenis Kelamin (L/P)",
		"Alamat",
		"No Telepon",
		"Tanggal Mulai Kerja (YYYY-MM-DD)",
		"Tanggal Lahir (YYYY-MM-DD)",
		"Tempat Lahir",
		"No KTP",
		"Agama",
		"Status Pernikahan",
		"Kebangsaan",
	}
	header := rows[0]
	if len(header) != len(expectedHeaders) {
		return 0, responses.BadRequest(fmt.Errorf("invalid header, expected columns: %s", strings.Join(expectedHeaders, ",")))
	}
	for i, h := range expectedHeaders {
		if header[i] != h {
			return 0, responses.BadRequest(fmt.Errorf("invalid header for column %d: expected %s", i+1, h))
		}
	}

	type rowData struct {
		Payload  *entities.SatpamUpsertPayload
		Password string
	}
	var entries []rowData
	emailRow := make(map[string]int)

	for idx, row := range rows[1:] {
		rowNum := idx + 2

		values := make([]string, len(expectedHeaders))
		for i := 0; i < len(expectedHeaders); i++ {
			if i < len(row) {
				values[i] = strings.TrimSpace(row[i])
			}
		}

		// skip empty or commented rows
		allEmpty := true
		for _, v := range values {
			if v != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty || strings.HasPrefix(strings.TrimSpace(values[0]), "#") {
			continue
		}

		if len(row) > len(expectedHeaders) {
			return 0, responses.BadRequest(fmt.Errorf("row %d has extra columns", rowNum))
		}

		name := values[0]
		email := values[1]
		password := values[2]
		activeStr := strings.ToUpper(values[3])
		jabatan := values[4]
		jenisKelamin := strings.ToUpper(values[5])
		alamat := values[6]
		noTelepon := values[7]
		workStartStr := values[8]
		tanggalLahirStr := values[9]
		tempatLahirStr := values[10]
		noKTPStr := values[11]
		agamaStr := values[12]
		statusNikahStr := values[13]
		kebangsaanStr := values[14]

		if name == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Nama: value is required", rowNum))
		}
		if email == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: value is required", rowNum))
		}
		if !strings.Contains(email, "@") {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: invalid email format", rowNum))
		}
		if password == "" || len(password) < 8 {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Password: minimum length is 8 characters", rowNum))
		}
		if jabatan == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jabatan: value is required", rowNum))
		}
		if jenisKelamin != "L" && jenisKelamin != "P" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jenis Kelamin (L/P): value must be L or P", rowNum))
		}
		if alamat == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Alamat: value is required", rowNum))
		}
		if noTelepon == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column No Telepon: value is required", rowNum))
		}
		if workStartStr == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Tanggal Mulai Kerja (YYYY-MM-DD): value is required", rowNum))
		}
		workStart, err := parseFlexibleDate(workStartStr)
		if err != nil {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Tanggal Mulai Kerja (YYYY-MM-DD): %v", rowNum, err))
		}

		var tanggalLahirPtr *time.Time
		if tanggalLahirStr != "" {
			d, err := parseFlexibleDate(tanggalLahirStr)
			if err != nil {
				return 0, responses.BadRequest(fmt.Errorf("row %d, column Tanggal Lahir (YYYY-MM-DD): %v", rowNum, err))
			}
			tanggalLahirPtr = &d
		}

		var tempatLahirPtr *string
		if tempatLahirStr != "" {
			v := tempatLahirStr
			tempatLahirPtr = &v
		}
		var noKTPPtr *string
		if noKTPStr != "" {
			v := noKTPStr
			noKTPPtr = &v
		}
		var agamaPtr *string
		if agamaStr != "" {
			v := agamaStr
			agamaPtr = &v
		}
		var statusNikahPtr *string
		if statusNikahStr != "" {
			v := statusNikahStr
			statusNikahPtr = &v
		}
		var kebangsaanPtr *string
		if kebangsaanStr != "" {
			v := kebangsaanStr
			kebangsaanPtr = &v
		}

		isActive := true
		if activeStr != "" {
			switch activeStr {
			case "FALSE", "0", "N", "NO":
				isActive = false
			default:
				isActive = true
			}
		}

		key := strings.ToLower(email)
		if prevRow, ok := emailRow[key]; ok {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: duplicate email (also in row %d)", rowNum, prevRow))
		}
		emailRow[key] = rowNum

		var count int
		if err := s.app.Ds.ReaderDB.GetContext(ctx, &count, `
			SELECT COUNT(1) FROM users WHERE email = ?
		`, email); err != nil {
			return 0, responses.InternalServerError(err)
		}
		if count > 0 {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: email already exists", rowNum))
		}

		payload := &entities.SatpamUpsertPayload{
			Name:             name,
			Email:            email,
			Active:           isActive,
			Jabatan:          jabatan,
			JenisKelamin:     jenisKelamin,
			TanggalLahir:     tanggalLahirPtr,
			TempatLahir:      tempatLahirPtr,
			NoKTP:            noKTPPtr,
			Alamat:           alamat,
			NoTelepon:        noTelepon,
			Agama:            agamaPtr,
			StatusPernikahan: statusNikahPtr,
			Kebangsaan:       kebangsaanPtr,
			WorkStartDate:    workStart,
		}

		entries = append(entries, rowData{
			Payload:  payload,
			Password: password,
		})
	}

	if len(entries) == 0 {
		return 0, responses.BadRequest(errors.New("no data rows found"))
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return 0, responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	inserted := 0
	for _, e := range entries {
		hash, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)
		if err != nil {
			return 0, responses.InternalServerError(err)
		}

		res, err := tx.ExecContext(ctx, `
			INSERT INTO users (name, email, password_hash, role, work_start_date, active)
			VALUES (?, ?, ?, 'SATPAM', ?, ?)
		`, e.Payload.Name, e.Payload.Email, string(hash), e.Payload.WorkStartDate.Format("2006-01-02"), e.Payload.Active)
		if err != nil {
			if responses.IsDuplicateErr(err) {
				return 0, responses.BadRequest(fmt.Errorf("duplicate email detected during insert"))
			}
			return 0, responses.InternalServerError(err)
		}
		userID, err := res.LastInsertId()
		if err != nil {
			return 0, responses.InternalServerError(err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO satpam_profiles (
				user_id, jabatan, jenis_kelamin, tanggal_lahir, tempat_lahir, no_ktp,
				alamat, no_telepon, agama, status_pernikahan, kebangsaan, work_start_date
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, e.Payload.Jabatan, e.Payload.JenisKelamin, e.Payload.TanggalLahir, e.Payload.TempatLahir,
			e.Payload.NoKTP, e.Payload.Alamat, e.Payload.NoTelepon, e.Payload.Agama, e.Payload.StatusPernikahan,
			e.Payload.Kebangsaan, e.Payload.WorkStartDate); err != nil {
			return 0, responses.InternalServerError(err)
		}

		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, responses.InternalServerError(err)
	}

	return inserted, nil
}

func (s *Service) GenerateSchedulingTemplate(ctx context.Context, month, year int) ([]byte, error) {
	active := true
	users, err := s.repo.ListSatpam(ctx, &active, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	// Load shifts to build legend & allowed codes.
	shifts, err := s.repo.ListShifts(ctx, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	shiftMetaByCode, allowedCodes := buildSchedulingShiftCodes(shifts)

	loc := time.Now().Location()
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	lastDay := firstDay.AddDate(0, 1, -1).Day()

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	// Title
	title := "SECURITY SIAGA"
	_ = f.SetCellValue(sheet, "A1", title)
	lastColIdx := 3 + lastDay
	lastColCell, _ := excelize.CoordinatesToCellName(lastColIdx, 1)
	_ = f.MergeCell(sheet, "A1", lastColCell)

	// Subtitle, e.g. Jan-26
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	monthLabel := "???"
	if month >= 1 && month <= 12 {
		monthLabel = monthNames[month-1]
	}
	subtitle := fmt.Sprintf("%s-%02d", monthLabel, year%100)
	_ = f.SetCellValue(sheet, "A2", subtitle)
	subtitleCell, _ := excelize.CoordinatesToCellName(lastColIdx, 2)
	_ = f.MergeCell(sheet, "A2", subtitleCell)

	// Table header rows
	headerRow1 := 4
	headerRow2 := 5

	// Header row 1: NO | NAMA | JABATAN | 1..N
	_ = f.SetCellValue(sheet, "A4", "NO")
	_ = f.SetCellValue(sheet, "B4", "NAMA")
	_ = f.SetCellValue(sheet, "C4", "JABATAN")
	// Merge NO/NAMA/JABATAN header cells vertically with weekday row.
	_ = f.MergeCell(sheet, "A4", "A5")
	_ = f.MergeCell(sheet, "B4", "B5")
	_ = f.MergeCell(sheet, "C4", "C5")

	// Column widths: narrow for day columns, wider for labels.
	_ = f.SetColWidth(sheet, "A", "A", 4)   // NO
	_ = f.SetColWidth(sheet, "B", "B", 18)  // NAMA
	_ = f.SetColWidth(sheet, "C", "C", 12)  // JABATAN
	for d := 1; d <= lastDay; d++ {
		colIdx := 3 + d
		cell, _ := excelize.CoordinatesToCellName(colIdx, headerRow1)
		_ = f.SetCellInt(sheet, cell, d)
	}

	// Set narrow width for all day columns (approx 23px).
	startDayColName, _ := excelize.ColumnNumberToName(4)
	endDayColName, _ := excelize.ColumnNumberToName(lastColIdx)
	_ = f.SetColWidth(sheet, startDayColName, endDayColName, 3.0)

	// Header row 2: weekday initials
	for d := 1; d <= lastDay; d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, loc)
		var initial string
		switch date.Weekday() {
		case time.Monday:
			initial = "S" // Senin
		case time.Tuesday:
			initial = "S" // Selasa
		case time.Wednesday:
			initial = "R"
		case time.Thursday:
			initial = "K"
		case time.Friday:
			initial = "J"
		case time.Saturday:
			initial = "S"
		case time.Sunday:
			initial = "M"
		default:
			initial = ""
		}
		colIdx := 3 + d
		cell, _ := excelize.CoordinatesToCellName(colIdx, headerRow2)
		_ = f.SetCellValue(sheet, cell, initial)

		// Minggu: red background
		if date.Weekday() == time.Sunday {
			style, _ := f.NewStyle(&excelize.Style{
				Fill: excelize.Fill{
					Type:    "pattern",
					Color:   []string{"#FB5E5A"},
					Pattern: 1,
				},
			})
			_ = f.SetCellStyle(sheet, cell, cell, style)
		}
	}

	// Common borders for table cells.
	commonBorders := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}

	// Header styles
	// Left labels (NO/NAMA/JABATAN) with shaded background.
	labelHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FDCD01"}, // yellow
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: commonBorders,
	})
	_ = f.SetCellStyle(sheet, "A4", "C5", labelHeaderStyle)

	// Day & weekday headers (columns D..)
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: commonBorders,
	})
	headerStartCell1, _ := excelize.CoordinatesToCellName(4, headerRow1)
	headerEndCell1, _ := excelize.CoordinatesToCellName(lastColIdx, headerRow1)
	headerStartCell2, _ := excelize.CoordinatesToCellName(4, headerRow2)
	headerEndCell2, _ := excelize.CoordinatesToCellName(lastColIdx, headerRow2)
	_ = f.SetCellStyle(sheet, headerStartCell1, headerEndCell1, headerStyle)
	_ = f.SetCellStyle(sheet, headerStartCell2, headerEndCell2, headerStyle)

	// Sunday date numbers in row 4 colored red (font + background).
	sundayDateStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFF0000",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FB5E5A"},
			Pattern: 1,
		},
		Border: commonBorders,
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	for d := 1; d <= lastDay; d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, loc)
		if date.Weekday() == time.Sunday {
			colIdx := 3 + d
			cell, _ := excelize.CoordinatesToCellName(colIdx, headerRow1)
			_ = f.SetCellStyle(sheet, cell, cell, sundayDateStyle)
		}
	}

	// Sunday weekday initials (row 5) with red font on red background.
	sundayWeekdayStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFF0000",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FB5E5A"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: commonBorders,
	})
	for d := 1; d <= lastDay; d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, loc)
		if date.Weekday() == time.Sunday {
			colIdx := 3 + d
			cell, _ := excelize.CoordinatesToCellName(colIdx, headerRow2)
			_ = f.SetCellStyle(sheet, cell, cell, sundayWeekdayStyle)
		}
	}

	// Data rows
	startRow := 6
	row := startRow
	for i, u := range users {
		noCell, _ := excelize.CoordinatesToCellName(1, row)
		nameCell, _ := excelize.CoordinatesToCellName(2, row)
		roleCell, _ := excelize.CoordinatesToCellName(3, row)
		_ = f.SetCellInt(sheet, noCell, i+1)
		_ = f.SetCellValue(sheet, nameCell, u.Name)
		_ = f.SetCellValue(sheet, roleCell, u.Jabatan)
		row++
	}
	lastDataRow := row - 1

	// Borders & alignment for data rows only
	dataStartCell, _ := excelize.CoordinatesToCellName(1, startRow)
	dataEndCell, _ := excelize.CoordinatesToCellName(lastColIdx, lastDataRow)
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: commonBorders,
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	_ = f.SetCellStyle(sheet, dataStartCell, dataEndCell, dataStyle)

	// Conditional formatting: X => red background.
	dayStartColCell, _ := excelize.CoordinatesToCellName(4, startRow)
	dayEndColCell, _ := excelize.CoordinatesToCellName(lastColIdx, lastDataRow)
	condStyleID, _ := f.NewConditionalStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FB5E5A"},
			Pattern: 1,
		},
	})
	rangeRef := fmt.Sprintf("%s:%s", dayStartColCell, dayEndColCell)
	_ = f.SetConditionalFormat(sheet, rangeRef, []excelize.ConditionalFormatOptions{
		{
			Type:     "cell",
			Criteria: "==",
			Format:   &condStyleID,
			Value:    "\"X\"",
		},
	})

	// Data validation: only allow codes defined in legend (empty allowed).
	if len(allowedCodes) > 0 {
		promptTitle := "Kode shift"
		prompt := fmt.Sprintf("Gunakan salah satu dari: %s", strings.Join(allowedCodes, ", "))
		errorStyle := "stop"
		errorTitle := "Kode shift tidak valid"
		errorMsg := fmt.Sprintf("Hanya boleh: %s", strings.Join(allowedCodes, ", "))

		listFormula := fmt.Sprintf("\"%s\"", strings.Join(allowedCodes, ","))
		_ = f.AddDataValidation(sheet, &excelize.DataValidation{
			Type:             "list",
			Sqref:            rangeRef,
			AllowBlank:       true,
			ShowInputMessage: true,
			PromptTitle:      &promptTitle,
			Prompt:           &prompt,
			ShowErrorMessage: true,
			ErrorStyle:       &errorStyle,
			ErrorTitle:       &errorTitle,
			Error:            &errorMsg,
			Formula1:         listFormula,
		})
	}

	// Legend from shifts (KET / JAM KERJA)
	legendStartRow := lastDataRow + 2
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", legendStartRow), "KET")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", legendStartRow), "JAM KERJA")

	legendRow := legendStartRow + 1
	for _, code := range allowedCodes {
		meta, ok := shiftMetaByCode[code]
		if !ok {
			continue
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", legendRow), code)

		label := "LIBUR"
		if !strings.EqualFold(meta.Name, "Libur") {
			formatTime := func(t string) string {
				if len(t) >= 5 {
					s := t[:5]
					return strings.ReplaceAll(s, ":", ".")
				}
				return t
			}
			label = fmt.Sprintf("%s - %s", formatTime(meta.Start), formatTime(meta.End))
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", legendRow), label)
		legendRow++
	}

	_ = f.SetRowHeight(sheet, 1, 22)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

func (s *Service) ImportSchedulingFromExcel(ctx context.Context, adminID int64, fileData []byte) (*entities.SchedulingImportResult, error) {
	_ = adminID

	f, err := excelize.OpenReader(bytes.NewReader(fileData))
	if err != nil {
		return nil, responses.BadRequest(errors.New("invalid Excel file"))
	}
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	// Subtitle in A2: Jan-26
	subtitle, err := f.GetCellValue(sheet, "A2")
	if err != nil || strings.TrimSpace(subtitle) == "" {
		return nil, responses.BadRequest(errors.New("subtitle (month-year) is missing at A2"))
	}
	t, err := time.Parse("Jan-06", strings.TrimSpace(subtitle))
	if err != nil {
		return nil, responses.BadRequest(errors.New("invalid subtitle format, expected Mon-YY (e.g. Jan-26)"))
	}
	year := t.Year()
	month := int(t.Month())
	loc := time.Now().Location()
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	lastDay := firstDay.AddDate(0, 1, -1).Day()

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, responses.BadRequest(errors.New("failed to read Excel rows"))
	}
	if len(rows) < 6 {
		return nil, responses.BadRequest(errors.New("template must include header and data rows"))
	}

	headerRowIdx := 3 // row 4 (0-based index)
	headerRow := rows[headerRowIdx]
	if len(headerRow) < 3+lastDay {
		return nil, responses.BadRequest(errors.New("invalid header row in template"))
	}

	// Map day column index -> day number.
	type dayCol struct {
		ColIdx int
		Day    int
	}
	var dayCols []dayCol
	for col := 3; col < 3+lastDay; col++ {
		if col >= len(headerRow) {
			break
		}
		val := strings.TrimSpace(headerRow[col])
		if val == "" {
			continue
		}
		d, err := strconv.Atoi(val)
		if err != nil {
			return nil, responses.BadRequest(fmt.Errorf("invalid day value in header at column %d", col+1))
		}
		dayCols = append(dayCols, dayCol{ColIdx: col + 1, Day: d})
	}

	// Build satpam name -> userID map.
	allSatpam, err := s.repo.ListSatpam(ctx, nil, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	nameToID := make(map[string]int64)
	for _, u := range allSatpam {
		key := strings.ToUpper(strings.TrimSpace(u.Name))
		if key == "" {
			continue
		}
		if _, exists := nameToID[key]; !exists {
			nameToID[key] = u.ID
		}
	}

	// Shift mapping (dynamic, based on current shifts / legend).
	shifts, err := s.repo.ListShifts(ctx, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	shiftMetaByCode, allowedCodes := buildSchedulingShiftCodes(shifts)
	allowedCodesStr := strings.Join(allowedCodes, ",")

	result := &entities.SchedulingImportResult{
		Errors: make([]entities.SchedulingImportError, 0),
	}

	// Data starts from row 6 (index 5).
	for rowIdx := 5; rowIdx < len(rows); rowIdx++ {
		excelRow := rowIdx + 1
		row := rows[rowIdx]

		// Jika menjumpai baris kosong pertama di bawah data, anggap sebagai akhir data.
		if len(row) == 0 {
			break
		}

		// Column A berisi nomor urut (1,2,3,...) untuk baris satpam.
		// Jika sudah tidak numeric (mis. "KET", "X", dll.) atau kosong, anggap akhir data dan stop loop.
		var noStr string
		if len(row) > 0 {
			noStr = strings.TrimSpace(row[0])
		}
		if noStr == "" {
			break
		}
		if _, err := strconv.Atoi(noStr); err != nil {
			break
		}

		// NAMA at column B (index 1)
		var name string
		if len(row) > 1 {
			name = strings.TrimSpace(row[1])
		}
		if name == "" {
			// assume end of data
			continue
		}

		result.ProcessedRows++

		userID, ok := nameToID[strings.ToUpper(name)]
		if !ok {
			result.Errors = append(result.Errors, entities.SchedulingImportError{
				SatpamName: name,
				Date:       "",
				Value:      "",
				Reason:     "satpam name not found",
			})
			result.Skipped++
			continue
		}

		for _, dc := range dayCols {
			// Read value via GetCellValue to include blanks.
			cellRef, _ := excelize.CoordinatesToCellName(dc.ColIdx, excelRow)
			val, _ := f.GetCellValue(sheet, cellRef)
			code := strings.ToUpper(strings.TrimSpace(val))
			if code == "" {
				continue
			}

			result.ProcessedCells++

			meta, ok := shiftMetaByCode[code]
			if !ok {
				result.Errors = append(result.Errors, entities.SchedulingImportError{
					SatpamName: name,
					Date:       time.Date(year, time.Month(month), dc.Day, 0, 0, 0, 0, loc).Format("2006-01-02"),
					Value:      code,
					Reason:     fmt.Sprintf("invalid shift code (allowed: %s)", allowedCodesStr),
				})
				result.Skipped++
				continue
			}

			shiftDate := time.Date(year, time.Month(month), dc.Day, 0, 0, 0, 0, loc)
			inserted, updated, err := s.repo.UpsertUserShift(ctx, userID, meta.ID, shiftDate)
			if err != nil {
				result.Errors = append(result.Errors, entities.SchedulingImportError{
					SatpamName: name,
					Date:       shiftDate.Format("2006-01-02"),
					Value:      code,
					Reason:     err.Error(),
				})
				result.Skipped++
				continue
			}
			if inserted {
				result.Inserted++
			} else if updated {
				result.Updated++
			} else {
				result.Skipped++
			}
		}
	}

	return result, nil
}

// schedulingShiftMeta describes a shift with its generated code for scheduling template/import.
type schedulingShiftMeta struct {
	ID    int64
	Name  string
	Start string
	End   string
}

// buildSchedulingShiftCodes builds a stable mapping from shift names to short codes:
// - "Pagi"  -> "P"
// - "Siang" -> "S"
// - "Malam" -> "M"
// - "Libur" -> "X"
// - other shift names get generated codes A-Z that are not used yet.
// Returns:
// - map[code]*meta
// - slice of codes (in stable order) for validation/legend.
func buildSchedulingShiftCodes(shifts []*entities.Shift) (map[string]*schedulingShiftMeta, []string) {
	reserved := map[string]string{
		"Pagi":  "P",
		"Siang": "S",
		"Malam": "M",
		"Libur": "X",
	}

	used := map[string]bool{}

	// Sort shifts by name for deterministic codes.
	sort.Slice(shifts, func(i, j int) bool {
		return strings.ToLower(strings.TrimSpace(shifts[i].Name)) < strings.ToLower(strings.TrimSpace(shifts[j].Name))
	})

	metaByCode := make(map[string]*schedulingShiftMeta)
	codes := make([]string, 0, len(shifts))

	nextCode := func() string {
		for ch := 'A'; ch <= 'Z'; ch++ {
			c := string(ch)
			if !used[c] {
				return c
			}
		}
		return "?" // fallback
	}

	for _, sft := range shifts {
		name := strings.TrimSpace(sft.Name)
		if name == "" {
			continue
		}

		code, ok := reserved[name]
		if !ok {
			code = nextCode()
		}

		// Mark code as used and register meta.
		used[code] = true
		if _, exists := metaByCode[code]; exists {
			continue
		}

		meta := &schedulingShiftMeta{
			ID:    sft.ID,
			Name:  name,
			Start: sft.StartTime,
			End:   sft.EndTime,
		}
		metaByCode[code] = meta
		codes = append(codes, code)
	}

	return metaByCode, codes
}

func (s *Service) ImportShiftsFromExcel(ctx context.Context, adminID int64, fileData []byte) (int, error) {
	_ = adminID

	f, err := excelize.OpenReader(bytes.NewReader(fileData))
	if err != nil {
		return 0, responses.BadRequest(errors.New("invalid Excel file"))
	}
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, responses.BadRequest(errors.New("failed to read Excel rows"))
	}
	if len(rows) < 2 {
		return 0, responses.BadRequest(errors.New("template must include header and at least one data row"))
	}

	expectedHeaders := []string{
		"Nama Shift",
		"Jam Mulai (HH:mm)",
		"Jam Selesai (HH:mm)",
		"Toleransi Keterlambatan (menit)",
	}
	header := rows[0]
	if len(header) != len(expectedHeaders) {
		return 0, responses.BadRequest(fmt.Errorf("invalid header, expected columns: %s", strings.Join(expectedHeaders, ",")))
	}
	for i, h := range expectedHeaders {
		if header[i] != h {
			return 0, responses.BadRequest(fmt.Errorf("invalid header for column %d: expected %s", i+1, h))
		}
	}

	type rowData struct {
		Name          string
		Start         string
		End           string
		LateTolerance int
	}
	var entries []rowData

	for idx, row := range rows[1:] {
		rowNum := idx + 2

		values := make([]string, len(expectedHeaders))
		for i := 0; i < len(expectedHeaders); i++ {
			if i < len(row) {
				values[i] = strings.TrimSpace(row[i])
			}
		}

		allEmpty := true
		for _, v := range values {
			if v != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty || strings.HasPrefix(strings.TrimSpace(values[0]), "#") {
			continue
		}

		if len(row) > len(expectedHeaders) {
			return 0, responses.BadRequest(fmt.Errorf("row %d has extra columns", rowNum))
		}

		name := values[0]
		startStr := values[1]
		endStr := values[2]
		lateTolStr := values[3]

		if name == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Nama Shift: value is required", rowNum))
		}
		if startStr == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jam Mulai (HH:mm): value is required", rowNum))
		}
		if endStr == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jam Selesai (HH:mm): value is required", rowNum))
		}
		if lateTolStr == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Toleransi Keterlambatan (menit): value is required", rowNum))
		}

		if _, err := time.Parse("15:04", startStr); err != nil {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jam Mulai (HH:mm): invalid time, expected HH:MM", rowNum))
		}
		if _, err := time.Parse("15:04", endStr); err != nil {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Jam Selesai (HH:mm): invalid time, expected HH:MM", rowNum))
		}

		lateTol, err := strconv.Atoi(lateTolStr)
		if err != nil || lateTol < 0 {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Toleransi Keterlambatan (menit): must be a non-negative integer", rowNum))
		}

		entries = append(entries, rowData{
			Name:          name,
			Start:         startStr,
			End:           endStr,
			LateTolerance: lateTol,
		})
	}

	if len(entries) == 0 {
		return 0, responses.BadRequest(errors.New("no data rows found"))
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return 0, responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	inserted := 0
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO shifts (name, start_time, end_time, late_tolerance_minute)
			VALUES (?, ?, ?, ?)
		`, e.Name, e.Start, e.End, e.LateTolerance); err != nil {
			if responses.IsDuplicateErr(err) {
				return 0, responses.BadRequest(fmt.Errorf("duplicate shift detected during insert"))
			}
			return 0, responses.InternalServerError(err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, responses.InternalServerError(err)
	}

	return inserted, nil
}

func (s *Service) ExportSatpamToExcel(ctx context.Context) ([]byte, error) {
	users, err := s.repo.ListSatpam(ctx, nil, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	now := time.Now()

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Nama",
		"Jabatan",
		"Jenis Kelamin",
		"Tanggal Lahir",
		"Tempat Lahir",
		"No. KTP",
		"Alamat",
		"No. Telepon",
		"Agama",
		"Status Pernikahan",
		"Kebangsaan",
		"Tanggal Mulai Kerja",
		"Lama Bekerja",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	_ = setPrettyHeaderStyle(f, sheet, len(headers))
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "M", 28)

	rowIdx := 2
	for _, u := range users {
		// Gender label
		genderLabel := ""
		switch strings.ToUpper(u.JenisKelamin) {
		case "L":
			genderLabel = "Laki-laki"
		case "P":
			genderLabel = "Perempuan"
		default:
			genderLabel = u.JenisKelamin
		}

		var tglLahirStr string
		if u.TanggalLahir != nil {
			tglLahirStr = u.TanggalLahir.Format("2006-01-02")
		}

		var tempatLahir string
		if u.TempatLahir != nil {
			tempatLahir = *u.TempatLahir
		}

		var noKTP string
		if u.NoKTP != nil {
			noKTP = *u.NoKTP
		}
		var agama string
		if u.Agama != nil {
			agama = *u.Agama
		}
		var statusNikah string
		if u.StatusPernikahan != nil {
			statusNikah = *u.StatusPernikahan
		}
		var kebangsaan string
		if u.Kebangsaan != nil {
			kebangsaan = *u.Kebangsaan
		}

		workStartStr := u.WorkStartDate.Format("2006-01-02")

		// Hitung lama bekerja dalam tahun & bulan (dibulatkan ke bawah).
		years := now.Year() - u.WorkStartDate.Year()
		months := int(now.Month()) - int(u.WorkStartDate.Month())
		if now.Day() < u.WorkStartDate.Day() {
			months--
		}
		if months < 0 {
			years--
			months += 12
		}
		if years < 0 {
			years = 0
		}
		if months < 0 {
			months = 0
		}

		var lamaBekerja string
		switch {
		case years > 0 && months > 0:
			lamaBekerja = fmt.Sprintf("%d tahun %d bulan", years, months)
		case years > 0:
			lamaBekerja = fmt.Sprintf("%d tahun", years)
		default:
			lamaBekerja = fmt.Sprintf("%d bulan", months)
		}

		row := []interface{}{
			u.Name,
			u.Jabatan,
			genderLabel,
			tglLahirStr,
			tempatLahir,
			noKTP,
			u.Alamat,
			u.NoTelepon,
			agama,
			statusNikah,
			kebangsaan,
			workStartStr,
			lamaBekerja,
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, responses.InternalServerError(err)
		}
		rowIdx++
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

func (s *Service) ExportAttendanceMonitoringToExcel(ctx context.Context, startDate, endDate time.Time) ([]byte, error) {
	if endDate.Before(startDate) {
		return nil, responses.BadRequest(errors.New("end_date must be >= start_date"))
	}

	wib, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, wib)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, wib)
	today := time.Now().In(wib).Truncate(24 * time.Hour)

	users, err := s.repo.ListSatpam(ctx, nil, 0, 0)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Tanggal",
		"Nama Satpam",
		"Shift",
		"Jam Masuk Shift",
		"Jam Keluar Shift",
		"Jam Masuk",
		"Jam Keluar",
		"Status Kehadiran",
		"Status Telat",
		"Durasi Telat",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	_ = setPrettyHeaderStyle(f, sheet, len(headers))
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "J", 20)

	rowIdx := 2
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
			End   string
		}
		shiftByUser := make(map[int64]shiftInfo)
		for _, r := range shiftRows {
			shiftByUser[r.UserID] = shiftInfo{
				Name:  r.ShiftName,
				Start: r.StartTime,
				End:   r.EndTime,
			}
		}

		for _, u := range users {
			if !u.WorkStartDate.IsZero() {
				ws := time.Date(u.WorkStartDate.Year(), u.WorkStartDate.Month(), u.WorkStartDate.Day(), 0, 0, 0, 0, u.WorkStartDate.Location())
				if ws.After(d) {
					continue
				}
			}

			attendance := attByUser[u.ID]

			dateStr := d.Format("2006-01-02")
			shiftName := ""
			shiftStartStr := ""
			shiftEndStr := ""
			jamMasuk := ""
			jamKeluar := ""
			statusKehadiran := ""
			statusTelat := ""
			durasiTelat := ""

			if attendance != nil {
				shiftName = attendance.ShiftName
				if attendance.ShiftStart != "" {
					if len(attendance.ShiftStart) >= 5 {
						shiftStartStr = attendance.ShiftStart[:5]
					} else {
						shiftStartStr = attendance.ShiftStart
					}
				}
				if attendance.ShiftEnd != "" {
					if len(attendance.ShiftEnd) >= 5 {
						shiftEndStr = attendance.ShiftEnd[:5]
					} else {
						shiftEndStr = attendance.ShiftEnd
					}
				}
				statusKehadiran = "HADIR"
				if attendance.ClockInTime != nil {
					jamMasuk = attendance.ClockInTime.Format("15:04")
				}
				if attendance.ClockOutTime != nil {
					jamKeluar = attendance.ClockOutTime.Format("15:04")
				}
				if attendance.ClockInStatus != nil {
					switch *attendance.ClockInStatus {
					case string(entities.LateStatusOnTime):
						statusTelat = "TEPAT_WAKTU"
					case string(entities.LateStatusLate), string(entities.LateStatusTooLate):
						statusTelat = "TELAT"
						// compute duration late based on shift start + tolerance
						if attendance.ClockInTime != nil && attendance.ShiftStart != "" {
							if tStart, err := time.Parse("15:04:05", attendance.ShiftStart); err == nil {
								shiftStartTime := time.Date(
									d.Year(), d.Month(), d.Day(),
									tStart.Hour(), tStart.Minute(), tStart.Second(),
									0, attendance.ClockInTime.Location(),
								)
								threshold := shiftStartTime.Add(time.Duration(attendance.LateTolerance) * time.Minute)
								if delay := attendance.ClockInTime.Sub(threshold); delay > 0 {
									totalMinutes := int(delay.Minutes())
									hours := totalMinutes / 60
									minutes := totalMinutes % 60
									parts := []string{}
									if hours > 0 {
										parts = append(parts, fmt.Sprintf("%d jam", hours))
									}
									if minutes > 0 {
										parts = append(parts, fmt.Sprintf("%d menit", minutes))
									}
									if len(parts) == 0 {
										durasiTelat = "0 menit"
									} else {
										durasiTelat = strings.Join(parts, " ")
									}
								}
							}
						}
					}
				}
			} else {
				if info, ok := shiftByUser[u.ID]; ok {
					shiftName = info.Name
					if info.Start != "" {
						if len(info.Start) >= 5 {
							shiftStartStr = info.Start[:5]
						} else {
							shiftStartStr = info.Start
						}
					}
					if info.End != "" {
						if len(info.End) >= 5 {
							shiftEndStr = info.End[:5]
						} else {
							shiftEndStr = info.End
						}
					}
					// Jika tanggal shift sudah lewat (<= hari ini) dan tidak ada absensi -> TIDAK_ABSEN.
					// Jika tanggal di depan hari ini -> BELUM_ABSEN (belum terjadi).
					if !d.After(today) {
						statusKehadiran = "TIDAK_ABSEN"
					} else {
						statusKehadiran = "BELUM_ABSEN"
					}
				}
			}

			row := []interface{}{
				dateStr,
				u.Name,
				shiftName,
				shiftStartStr,
				shiftEndStr,
				jamMasuk,
				jamKeluar,
				statusKehadiran,
				statusTelat,
				durasiTelat,
			}
			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			if err := f.SetSheetRow(sheet, cell, &row); err != nil {
				return nil, responses.InternalServerError(err)
			}
			rowIdx++
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return buf.Bytes(), nil
}

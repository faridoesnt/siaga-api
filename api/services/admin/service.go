package admin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

// Satpam management

func (s *Service) CreateSatpam(ctx context.Context, adminID int64, email, password, name string, workStartDate *time.Time) (*entities.User, error) {
	if email == "" || password == "" || name == "" {
		return nil, responses.BadRequest(errors.New("email, password and name are required"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	user, err := s.repo.CreateSatpam(ctx, email, string(hash), name, workStartDate)
	if err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("email already in use"))
		}
		return nil, responses.InternalServerError(err)
	}

	_ = adminID // reserved for future auditing if needed

	return user, nil
}

func (s *Service) ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.User, error) {
	users, err := s.repo.ListSatpam(ctx, active, limit, offset)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return users, nil
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

func (s *Service) UpdateSatpam(ctx context.Context, adminID, userID int64, email, name string, workStartDate *time.Time) (*entities.User, error) {
	if email == "" || name == "" {
		return nil, responses.BadRequest(errors.New("email and name are required"))
	}

	user, err := s.repo.UpdateSatpam(ctx, userID, email, name, workStartDate)
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

	// Excel serial date (e.g. 45205)
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		if t, err2 := excelize.ExcelDateToTime(n, false); err2 == nil {
			return t, nil
		}
	}

	// Strip time part if present (e.g. "2025-01-01 00:00:00")
	if i := strings.IndexAny(value, " T"); i > 0 {
		value = value[:i]
	}

	candidates := []string{value}
	// Also try with unified separators
	repl := strings.NewReplacer("\\", "/", "-", "/", ".", "/")
	if v2 := repl.Replace(value); v2 != value {
		candidates = append(candidates, v2)
	}

	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"02/01/2006", // dd/mm/yyyy
		"2/1/2006",
		"02/01/06", // dd/mm/yy
		"2/1/06",
	}

	for _, cand := range candidates {
		for _, layout := range layouts {
			if t, err := time.Parse(layout, cand); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("invalid date, expected YYYY-MM-DD or dd/mm/yyyy")
}

func (s *Service) GenerateSatpamImportTemplate(ctx context.Context) ([]byte, error) {
	_ = ctx

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Nama",
		"Email",
		"Password",
		"Tanggal Mulai Kerja (YYYY-MM-DD)",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	_ = setPrettyHeaderStyle(f, sheet, len(headers))
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "D", 28)

	// Example row (commented)
	_ = f.SetCellValue(sheet, "A2", "#contoh (hapus baris ini)")
	_ = f.SetCellValue(sheet, "B2", "satpam1@siaga.local")
	_ = f.SetCellValue(sheet, "C2", "Password123")
	_ = f.SetCellValue(sheet, "D2", "2025-01-01")

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
		"Tanggal Mulai Kerja (YYYY-MM-DD)",
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
		Name         string
		Email        string
		Password     string
		WorkStart    time.Time
		WorkStartStr string
	}
	var entries []rowData
	emailRow := make(map[string]int)

	for idx, row := range rows[1:] {
		rowNum := idx + 2

		// normalize row to expected length
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
		workStartStr := values[3]

		if name == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Nama: value is required", rowNum))
		}
		if email == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: value is required", rowNum))
		}
		if !strings.Contains(email, "@") {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: invalid email format", rowNum))
		}
		if password == "" || len(password) < 6 {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Password: minimum length is 6 characters", rowNum))
		}
		if workStartStr == "" {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Tanggal Mulai Kerja (YYYY-MM-DD): value is required", rowNum))
		}
		workStart, err := parseFlexibleDate(workStartStr)
		if err != nil {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Tanggal Mulai Kerja (YYYY-MM-DD): %v", rowNum, err))
		}
		// normalize to YYYY-MM-DD for DB insert
		workStartStr = workStart.Format("2006-01-02")

		key := strings.ToLower(email)
		if prevRow, ok := emailRow[key]; ok {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: duplicate email (also in row %d)", rowNum, prevRow))
		}
		emailRow[key] = rowNum

		// check existing email in DB
		var count int
		if err := s.app.Ds.ReaderDB.GetContext(ctx, &count, `
			SELECT COUNT(1) FROM users WHERE email = ?
		`, email); err != nil {
			return 0, responses.InternalServerError(err)
		}
		if count > 0 {
			return 0, responses.BadRequest(fmt.Errorf("row %d, column Email: email already exists", rowNum))
		}

		entries = append(entries, rowData{
			Name:         name,
			Email:        email,
			Password:     password,
			WorkStart:    workStart,
			WorkStartStr: workStartStr,
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

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (name, email, password_hash, role, work_start_date, active)
			VALUES (?, ?, ?, 'SATPAM', ?, 1)
		`, e.Name, e.Email, string(hash), e.WorkStartStr); err != nil {
			if responses.IsDuplicateErr(err) {
				return 0, responses.BadRequest(fmt.Errorf("duplicate email detected during insert"))
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

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Nama",
		"Email",
		"Tanggal Mulai Kerja",
		"Lama Bekerja",
		"Aktif",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, responses.InternalServerError(err)
		}
	}

	_ = setPrettyHeaderStyle(f, sheet, len(headers))
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetColWidth(sheet, "A", "E", 24)

	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	rowIdx := 2
	for _, u := range users {
		var workStartStr string
		var lamaBekerja string

		if u.WorkStartDate != nil {
			wd := time.Date(u.WorkStartDate.Year(), u.WorkStartDate.Month(), u.WorkStartDate.Day(), 0, 0, 0, 0, u.WorkStartDate.Location())
			workStartStr = wd.Format("2006-01-02")

			if !wd.After(today) {
				totalDays := int(today.Sub(wd).Hours() / 24)
				years := totalDays / 365
				months := (totalDays % 365) / 30
				parts := []string{}
				if years > 0 {
					parts = append(parts, fmt.Sprintf("%d tahun", years))
				}
				if months > 0 {
					parts = append(parts, fmt.Sprintf("%d bulan", months))
				}
				if len(parts) == 0 {
					lamaBekerja = "0 bulan"
				} else {
					lamaBekerja = strings.Join(parts, " ")
				}
			}
		}

		row := []interface{}{
			u.Name,
			u.Email,
			workStartStr,
			lamaBekerja,
			u.Active,
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

	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())

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
			if u.WorkStartDate != nil {
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
					statusKehadiran = "TIDAK_ABSEN"
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

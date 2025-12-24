package satpam

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/contracts"
	"siaga-api/api/entities"
	"siaga-api/api/models/responses"
	"siaga-api/internal/pkg/email"
	"siaga-api/internal/pkg/face"
	"siaga-api/internal/pkg/geo"
)

type Service struct {
	app        *contracts.App
	repo       Repository
	faceClient face.Client
	faceBypass bool
}

func Init(app *contracts.App) contracts.SatpamService {
	repo := NewRepository(app)

	bypass := app.Config[constants.FaceVerifyBypass] == "true"
	baseURL := app.Config[constants.FaceServiceURL]
	client := face.New(baseURL, bypass)

	return &Service{
		app:        app,
		repo:       repo,
		faceClient: client,
		faceBypass: bypass,
	}
}

func (s *Service) GetMe(ctx context.Context, userID int64) (*entities.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.NotFound(errors.New("user not found"))
	}
	return user, nil
}

func (s *Service) GetDashboard(ctx context.Context, userID int64, date time.Time) (*entities.SatpamDashboard, error) {
	dateOnly := date.Truncate(24 * time.Hour)

	userShift, err := s.repo.GetUserShiftForDate(ctx, userID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	var shiftDto *entities.SatpamDashboardShift
	if userShift != nil {
		startStr := userShift.Shift.StartTime
		if len(startStr) >= 5 {
			startStr = startStr[:5]
		}
		endStr := userShift.Shift.EndTime
		if len(endStr) >= 5 {
			endStr = endStr[:5]
		}
		shiftDto = &entities.SatpamDashboardShift{
			ID:                  userShift.Shift.ID,
			Name:                userShift.Shift.Name,
			StartTime:           startStr,
			EndTime:             endStr,
			LateToleranceMinute: userShift.Shift.LateToleranceMinute,
		}
	}

	attendance, err := s.repo.GetAttendanceForDate(ctx, userID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	attendanceDto := &entities.SatpamDashboardAttendance{
		Status: entities.AttendanceStatusNone,
	}

	if attendance != nil {
		if attendance.ClockInTime == nil {
			attendanceDto.Status = entities.AttendanceStatusNone
		} else if attendance.ClockOutTime == nil {
			attendanceDto.Status = entities.AttendanceStatusClockedIn
		} else {
			attendanceDto.Status = entities.AttendanceStatusClockedOut
		}

		attendanceDto.ClockInTime = attendance.ClockInTime
		attendanceDto.ClockOutTime = attendance.ClockOutTime
		if attendance.ClockInStatus != nil {
			ls := entities.LateStatus(*attendance.ClockInStatus)
			attendanceDto.LateStatus = &ls
		}
	}

	// open attendance (regardless of date)
	openAtt, err := s.repo.GetOpenAttendance(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	hasOpen := openAtt != nil

	canClockIn := false
	canClockOut := false

	if hasOpen {
		canClockIn = false
		canClockOut = true
	} else {
		// Tidak ada attendance yang masih terbuka.
		// Jika hari ini sudah lengkap clock-in & clock-out untuk shift hari ini,
		// maka tidak perlu mengizinkan clock-in lagi.
		if attendance != nil && attendance.ClockInTime != nil && attendance.ClockOutTime != nil {
			canClockIn = false
			canClockOut = false
		} else {
			// Masih belum absen hari ini; jika ada shift hari ini, izinkan clock-in.
			if userShift != nil {
				canClockIn = true
			}
			canClockOut = false
		}
	}

	var openSummary *entities.SatpamDashboardAttendance
	if openAtt != nil {
		openSummary = &entities.SatpamDashboardAttendance{
			Status:       entities.AttendanceStatusClockedIn,
			ClockInTime:  openAtt.ClockInTime,
			ClockOutTime: openAtt.ClockOutTime,
		}
		if openAtt.ClockInStatus != nil {
			ls := entities.LateStatus(*openAtt.ClockInStatus)
			openSummary.LateStatus = &ls
		}
	}

	return &entities.SatpamDashboard{
		Date:                  dateOnly.Format("2006-01-02"),
		Shift:                 shiftDto,
		Attendance:            attendanceDto,
		HasOpenAttendance:     hasOpen,
		CanClockIn:            canClockIn,
		CanClockOut:           canClockOut,
		OpenAttendanceSummary: openSummary,
	}, nil
}

func (s *Service) ListMyShiftDates(ctx context.Context, userID int64, from time.Time) ([]time.Time, error) {
	fromDate := from.Truncate(24 * time.Hour)
	dates, err := s.repo.ListMyShiftDates(ctx, userID, fromDate)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return dates, nil
}

func (s *Service) ListAttendanceHistory(ctx context.Context, userID int64, startDate, endDate time.Time) ([]*entities.SatpamAttendanceHistoryItem, error) {
	fromDate := startDate.Truncate(24 * time.Hour)
	toDate := endDate.Truncate(24 * time.Hour)
	if toDate.Before(fromDate) {
		fromDate, toDate = toDate, fromDate
	}

	items := make([]*entities.SatpamAttendanceHistoryItem, 0)
	for d := fromDate; !d.After(toDate); d = d.AddDate(0, 0, 1) {
		dOnly := d.Truncate(24 * time.Hour)

		userShift, err := s.repo.GetUserShiftForDate(ctx, userID, dOnly)
		if err != nil {
			return nil, responses.InternalServerError(err)
		}

		attendance, err := s.repo.GetAttendanceForDate(ctx, userID, dOnly)
		if err != nil {
			return nil, responses.InternalServerError(err)
		}

		if userShift == nil && attendance == nil {
			continue
		}

		var shiftDto *entities.SatpamDashboardShift
		if userShift != nil {
			startStr := userShift.Shift.StartTime
			if len(startStr) >= 5 {
				startStr = startStr[:5]
			}
			endStr := userShift.Shift.EndTime
			if len(endStr) >= 5 {
				endStr = endStr[:5]
			}
			shiftDto = &entities.SatpamDashboardShift{
				ID:                  userShift.Shift.ID,
				Name:                userShift.Shift.Name,
				StartTime:           startStr,
				EndTime:             endStr,
				LateToleranceMinute: userShift.Shift.LateToleranceMinute,
			}
		}

		attDto := &entities.SatpamDashboardAttendance{
			Status: entities.AttendanceStatusNone,
		}
		if attendance != nil {
			if attendance.ClockInTime == nil {
				attDto.Status = entities.AttendanceStatusNone
			} else if attendance.ClockOutTime == nil {
				attDto.Status = entities.AttendanceStatusClockedIn
			} else {
				attDto.Status = entities.AttendanceStatusClockedOut
			}

			attDto.ClockInTime = attendance.ClockInTime
			attDto.ClockOutTime = attendance.ClockOutTime
			if attendance.ClockInStatus != nil {
				ls := entities.LateStatus(*attendance.ClockInStatus)
				attDto.LateStatus = &ls
			}
		}

		items = append(items, &entities.SatpamAttendanceHistoryItem{
			Date:       dOnly.Format("2006-01-02"),
			Shift:      shiftDto,
			Attendance: attDto,
		})
	}

	return items, nil
}

func (s *Service) ClockIn(ctx context.Context, userID int64, lat, lng float64, imageBase64 string) (*entities.Attendance, error) {
	now := time.Now()
	dateOnly := now.Truncate(24 * time.Hour)

	// RULE 1: block if there is any open attendance
	openAtt, err := s.repo.GetOpenAttendance(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if openAtt != nil {
		return nil, responses.Conflict(errors.New("Masih ada absensi yang belum clock-out. Silakan clock-out terlebih dahulu."))
	}

	// ensure user has shift
	userShift, err := s.repo.GetUserShiftForDate(ctx, userID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if userShift == nil {
		return nil, responses.BadRequest(errors.New("no shift for today"))
	}

	// get attendance spots (may be more than one)
	spots, err := s.repo.GetActiveAttendanceSpots(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if len(spots) == 0 {
		return nil, responses.BadRequest(errors.New("no attendance spot assigned"))
	}

	var (
		chosenSpot *entities.AttendanceSpot
		minDist    float64
	)
	for i, sp := range spots {
		d := geo.DistanceMeters(lat, lng, sp.Latitude, sp.Longitude)
		if i == 0 || d < minDist {
			minDist = d
			chosenSpot = sp
		}
	}

	if chosenSpot == nil {
		return nil, responses.InternalServerError(errors.New("failed to determine attendance spot"))
	}
	if minDist > float64(chosenSpot.RadiusMeters) {
		return nil, responses.Forbidden(fmt.Errorf("outside attendance radius (%.2fm)", minDist))
	}

	// store clock-in photo
	var clockInPhotoURL *string
	if imageBase64 != "" {
		basePath := s.app.Config[constants.StoragePath]
		if basePath == "" {
			basePath = "./storage"
		}
		dateStr := dateOnly.Format("2006-01-02")
		dir := filepath.Join(basePath, "attendance", dateStr, fmt.Sprintf("%d", userID))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, responses.InternalServerError(err)
		}
		filename := fmt.Sprintf("clock-in-%d.jpg", now.UnixNano())
		fullPath := filepath.Join(dir, filename)

		data, err := base64.StdEncoding.DecodeString(imageBase64)
		if err != nil {
			return nil, responses.BadRequest(errors.New("invalid image_base64"))
		}
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			return nil, responses.InternalServerError(err)
		}

		url := fmt.Sprintf("/static/attendance/%s/%d/%s", dateStr, userID, filename)
		clockInPhotoURL = &url
	}

	// face verification
	embeddings, err := s.repo.GetFaceEmbeddings(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if len(embeddings) == 0 && !s.faceBypass {
		return nil, responses.BadRequest(errors.New("face not enrolled"))
	}

	matched, score, err := s.faceClient.VerifyFace(ctx, imageBase64, userID, embeddings)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if !matched {
		return nil, responses.Forbidden(errors.New("face verification failed"))
	}

	lateStatus := determineLateStatus(now, userShift.Shift)
	lateStatusStr := string(lateStatus)

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	att := &entities.Attendance{
		UserID:           userID,
		ShiftID:          userShift.Shift.ID,
		AttendanceSpotID: &chosenSpot.ID,
		ClockInSpotID:    &chosenSpot.ID,
		AttendanceDate:   dateOnly,
		ClockInTime:      &now,
		ClockInLatitude:  &lat,
		ClockInLongitude: &lng,
		ClockInStatus:    &lateStatusStr,
		ClockInPhotoURL:  clockInPhotoURL,
		FaceVerified:     matched,
		FaceMatchScore:   &score,
	}

	if err := s.repo.InsertAttendance(ctx, tx, att); err != nil {
		if responses.IsDuplicateErr(err) {
			return nil, responses.Conflict(errors.New("attendance already exists for today"))
		}
		return nil, responses.InternalServerError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, responses.InternalServerError(err)
	}
	return att, nil
}

func (s *Service) ClockOut(ctx context.Context, userID int64, lat, lng float64) (*entities.Attendance, error) {
	return s.ClockOutWithPhoto(ctx, userID, lat, lng, nil, "")
}

func (s *Service) ClockOutWithPhoto(ctx context.Context, userID int64, lat, lng float64, photoURL *string, imageBase64 string) (*entities.Attendance, error) {
	now := time.Now()
	if strings.TrimSpace(imageBase64) == "" {
		return nil, responses.BadRequest(errors.New("image is required"))
	}

	// resolve active attendance spot at current location/time
	spots, err := s.repo.GetActiveAttendanceSpots(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if len(spots) == 0 {
		return nil, responses.BadRequest(errors.New("no attendance spot assigned"))
	}

	var (
		chosenSpot *entities.AttendanceSpot
		minDist    float64
	)
	for i, sp := range spots {
		d := geo.DistanceMeters(lat, lng, sp.Latitude, sp.Longitude)
		if i == 0 || d < minDist {
			minDist = d
			chosenSpot = sp
		}
	}
	if chosenSpot == nil {
		return nil, responses.InternalServerError(errors.New("failed to determine attendance spot"))
	}
	if minDist > float64(chosenSpot.RadiusMeters) {
		return nil, responses.Forbidden(fmt.Errorf("outside attendance radius (%.2fm)", minDist))
	}

	// face verification
	embeddings, err := s.repo.GetFaceEmbeddings(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if len(embeddings) == 0 && !s.faceBypass {
		return nil, responses.BadRequest(errors.New("face not enrolled"))
	}
	if imageBase64 != "" {
		matched, _, err := s.faceClient.VerifyFace(ctx, imageBase64, userID, embeddings)
		if err != nil {
			return nil, responses.InternalServerError(err)
		}
		if !matched {
			return nil, responses.Forbidden(errors.New("face verification failed"))
		}
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	att, err := s.repo.GetOpenAttendanceForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if att == nil {
		return nil, responses.Conflict(errors.New("Tidak ada absensi yang perlu clock-out."))
	}
	if att.ClockInTime == nil {
		return nil, responses.BadRequest(errors.New("cannot clock out without clock in"))
	}

	if err := s.repo.UpdateAttendanceClockOut(ctx, tx, att.ID, now, lat, lng, photoURL, chosenSpot.ID); err != nil {
		return nil, responses.InternalServerError(err)
	}

	att.ClockOutTime = &now
	att.ClockOutLatitude = &lat
	att.ClockOutLongitude = &lng
	att.ClockOutPhotoURL = photoURL
	att.ClockOutSpotID = &chosenSpot.ID

	if err := tx.Commit(); err != nil {
		return nil, responses.InternalServerError(err)
	}

	return att, nil
}

func (s *Service) CreateActivityPhoto(ctx context.Context, userID, attendanceID int64, note string, photoURL string, takenAt time.Time, lat, lng float64) (*entities.DailyActivityPhoto, error) {
	// validate attendance belongs to user and is today
	now := time.Now()
	dateOnly := now.Truncate(24 * time.Hour)

	att, err := s.repo.GetAttendanceByIDForUserAndDate(ctx, userID, attendanceID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if att == nil {
		return nil, responses.BadRequest(errors.New("invalid attendance for today"))
	}

	// resolve active attendance spot at current location/time
	spots, err := s.repo.GetActiveAttendanceSpots(ctx, userID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if len(spots) == 0 {
		return nil, responses.BadRequest(errors.New("no attendance spot assigned"))
	}

	var (
		chosenSpot *entities.AttendanceSpot
		minDist    float64
	)
	for i, sp := range spots {
		d := geo.DistanceMeters(lat, lng, sp.Latitude, sp.Longitude)
		if i == 0 || d < minDist {
			minDist = d
			chosenSpot = sp
		}
	}
	if chosenSpot == nil {
		return nil, responses.InternalServerError(errors.New("failed to determine attendance spot"))
	}
	if minDist > float64(chosenSpot.RadiusMeters) {
		return nil, responses.Forbidden(fmt.Errorf("outside attendance radius (%.2fm)", minDist))
	}

	photo := &entities.DailyActivityPhoto{
		UserID:           userID,
		AttendanceID:     attendanceID,
		AttendanceSpotID: &chosenSpot.ID,
		PhotoURL:         photoURL,
	}
	if note != "" {
		photo.Note = &note
	}
	photo.TakenAt = takenAt

	if err := s.repo.InsertDailyActivityPhoto(ctx, photo); err != nil {
		return nil, responses.InternalServerError(err)
	}

	return photo, nil
}

func (s *Service) CreateShiftSwapRequest(ctx context.Context, userID, targetUserID int64, shiftDate time.Time, reason string) (*entities.ShiftSwapRequest, error) {
	dateOnly := shiftDate.Truncate(24 * time.Hour)
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	if dateOnly.Before(today) {
		return nil, responses.BadRequest(errors.New("shift_date must be today or later"))
	}

	requesterShift, err := s.repo.GetUserShiftForDate(ctx, userID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if requesterShift == nil {
		return nil, responses.BadRequest(errors.New("requester has no shift on that date"))
	}

	targetShift, err := s.repo.GetUserShiftForDate(ctx, targetUserID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if targetShift == nil {
		return nil, responses.BadRequest(errors.New("target user has no shift on that date"))
	}

	tx, err := s.app.Ds.WriterDB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// lock user_shifts to avoid race
	reqUs, err := s.repo.GetUserShiftForUpdate(ctx, tx, requesterShift.UserShift.ID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	tgtUs, err := s.repo.GetUserShiftForUpdate(ctx, tx, targetShift.UserShift.ID)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if reqUs == nil || tgtUs == nil {
		return nil, responses.Conflict(errors.New("user shifts not found"))
	}
	if !sameDate(reqUs.ShiftDate, dateOnly) || !sameDate(tgtUs.ShiftDate, dateOnly) {
		return nil, responses.Conflict(errors.New("shift date mismatch"))
	}
	if reqUs.UserID != userID || tgtUs.UserID != targetUserID {
		return nil, responses.Conflict(errors.New("user shift ownership mismatch"))
	}

	// swap shift IDs
	reqUs.ShiftID, tgtUs.ShiftID = tgtUs.ShiftID, reqUs.ShiftID
	reqUs.IsSwapped = true
	tgtUs.IsSwapped = true

	if err := s.repo.UpdateUserShift(ctx, tx, reqUs); err != nil {
		return nil, responses.InternalServerError(err)
	}
	if err := s.repo.UpdateUserShift(ctx, tx, tgtUs); err != nil {
		return nil, responses.InternalServerError(err)
	}

	decidedBy := int64(0) // system
	decidedAt := now

	req := &entities.ShiftSwapRequest{
		RequesterUserID:      userID,
		TargetUserID:         targetUserID,
		ShiftDate:            dateOnly,
		RequesterUserShiftID: requesterShift.UserShift.ID,
		TargetUserShiftID:    targetShift.UserShift.ID,
		Status:               entities.ShiftSwapStatusApproved,
		DecidedBy:            &decidedBy,
		DecidedAt:            &decidedAt,
	}
	if reason != "" {
		req.Reason = &reason
	}

	if err := s.repo.InsertShiftSwapRequestTx(ctx, tx, req); err != nil {
		return nil, responses.InternalServerError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, responses.InternalServerError(err)
	}

	// fire-and-forget email notification
	requesterName := fmt.Sprintf("User #%d", userID)
	if u, err := s.repo.GetUserByID(ctx, userID); err == nil && u != nil && u.Name != "" {
		requesterName = u.Name
	}
	targetName := fmt.Sprintf("User #%d", targetUserID)
	if u, err := s.repo.GetUserByID(ctx, targetUserID); err == nil && u != nil && u.Name != "" {
		targetName = u.Name
	}

	go email.SendShiftSwapNotification(context.Background(), email.ShiftSwapNotificationData{
		RequesterName: requesterName,
		TargetName:    targetName,
		ShiftDate:     dateOnly.Format("2006-01-02"),
		OriginalShift: requesterShift.Shift.Name,
		NewShift:      targetShift.Shift.Name,
		Timestamp:     now.Format(time.RFC3339),
	})

	return req, nil
}

func (s *Service) ListShiftSwapRequests(ctx context.Context, userID int64, status string) ([]*entities.ShiftSwapRequest, error) {
	requests, err := s.repo.ListShiftSwapRequests(ctx, userID, status)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return requests, nil
}

func (s *Service) ListShiftSwapPeers(ctx context.Context, userID int64, date time.Time) ([]*entities.User, error) {
	dateOnly := date.Truncate(24 * time.Hour)
	peers, err := s.repo.ListShiftSwapPeers(ctx, userID, dateOnly)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	return peers, nil
}

func determineLateStatus(now time.Time, shift entities.Shift) entities.LateStatus {
	loc := now.Location()
	parsed, err := time.ParseInLocation("15:04:05", shift.StartTime, loc)
	if err != nil {
		parsed, _ = time.ParseInLocation("15:04", shift.StartTime, loc)
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, loc)
	if now.Before(start) || now.Equal(start) {
		return entities.LateStatusOnTime
	}
	threshold := start.Add(time.Duration(shift.LateToleranceMinute) * time.Minute)
	if now.Before(threshold) || now.Equal(threshold) {
		return entities.LateStatusLate
	}
	return entities.LateStatusTooLate
}

func IsCrossDayShift(shift entities.Shift) bool {
	loc := time.Now().Location()
	start, err1 := time.ParseInLocation("15:04:05", shift.StartTime, loc)
	if err1 != nil {
		start, _ = time.ParseInLocation("15:04", shift.StartTime, loc)
	}
	end, err2 := time.ParseInLocation("15:04:05", shift.EndTime, loc)
	if err2 != nil {
		end, _ = time.ParseInLocation("15:04", shift.EndTime, loc)
	}
	return end.Before(start)
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

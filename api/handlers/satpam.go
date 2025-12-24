package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/models/responses"

	"github.com/gofiber/fiber/v2"
)

type clockInRequest struct {
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	ImageBase64 string   `json:"image_base64"`
}

type shiftSwapRequestBody struct {
	TargetUserID string `json:"target_user_id"`
	ShiftDate    string `json:"shift_date"`
	Reason       string `json:"reason"`
}

func getUserID(c *fiber.Ctx) (int64, error) {
	val := c.Locals("user_id")
	if id, ok := val.(int64); ok {
		return id, nil
	}
	return 0, responses.UnAuthorized(fmt.Errorf("invalid user context"))
}

// Me returns authenticated satpam profile.
func Me(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	user, err := app.Services.Satpam.GetMe(c.Context(), userID)
	if err != nil {
		return HttpError(c, err)
	}

	var workStart string
	if user.WorkStartDate != nil {
		workStart = user.WorkStartDate.Format("2006-01-02")
	}

	resp := fiber.Map{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"work_start_date": workStart,
	}

	return HttpSuccess(c, resp)
}

// SatpamDashboard returns dashboard data for given date.
func SatpamDashboard(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	rawDate := c.Query("date", "")
	var date time.Time
	if rawDate == "" {
		now := time.Now()
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		parsed, parseErr := time.Parse("2006-01-02", rawDate)
		if parseErr != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid date format, expected YYYY-MM-DD")))
		}
		date = parsed
	}

	dashboard, err := app.Services.Satpam.GetDashboard(c.Context(), userID, date)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, dashboard)
}

// SatpamClockIn handles clock-in with geofence and face verification.
func SatpamClockIn(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req clockInRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid body")))
	}
	if req.Lat == nil || req.Lng == nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("lat and lng are required")))
	}
	if !validLat(*req.Lat) || !validLng(*req.Lng) {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lat/lng range")))
	}
	if strings.TrimSpace(req.ImageBase64) == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("image_base64 is required")))
	}

	att, err := app.Services.Satpam.ClockIn(c.Context(), userID, *req.Lat, *req.Lng, req.ImageBase64)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, att)
}

func SatpamClockOut(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}
	latStr := c.FormValue("lat")
	lngStr := c.FormValue("lng")
	if latStr == "" || lngStr == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("lat and lng are required")))
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lat")))
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lng")))
	}
	if !validLat(lat) || !validLng(lng) {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lat/lng range")))
	}

	file, err := c.FormFile("photo")
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("photo is required")))
	}
	if err := validatePhotoFile(file); err != nil {
		return HttpError(c, responses.BadRequest(err))
	}

	// read file into base64 for face verification
	src, err := file.Open()
	if err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}
	defer src.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, src); err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}
	imageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	now := time.Now()
	dateStr := now.Format("2006-01-02")

	basePath := app.Config[constants.StoragePath]
	if basePath == "" {
		basePath = "./storage"
	}
	dir := filepath.Join(basePath, "attendance", dateStr, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}

	filename := fmt.Sprintf("clock-out-%d%s", now.UnixNano(), strings.ToLower(filepath.Ext(file.Filename)))
	if filepath.Ext(filename) == "" {
		filename += ".jpg"
	}
	fullPath := filepath.Join(dir, filename)

	if err := c.SaveFile(file, fullPath); err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}

	photoURL := fmt.Sprintf("/static/attendance/%s/%d/%s", dateStr, userID, filename)

	att, err := app.Services.Satpam.ClockOutWithPhoto(c.Context(), userID, lat, lng, &photoURL, imageBase64)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, att)
}

// SatpamUploadActivityPhoto handles photo upload for daily activities.
func SatpamUploadActivityPhoto(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	latStr := c.FormValue("lat")
	lngStr := c.FormValue("lng")
	if latStr == "" || lngStr == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("lat and lng are required")))
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || !validLat(lat) {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lat")))
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || !validLng(lng) {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid lng")))
	}

	attendanceIDStr := c.FormValue("attendance_id")
	if attendanceIDStr == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("attendance_id is required")))
	}
	attendanceID, err := strconv.ParseInt(attendanceIDStr, 10, 64)
	if err != nil || attendanceID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid attendance_id")))
	}

	note := c.FormValue("note")
	file, err := c.FormFile("photo")
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("photo is required")))
	}
	if err := validatePhotoFile(file); err != nil {
		return HttpError(c, responses.BadRequest(err))
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")

	basePath := app.Config[constants.StoragePath]
	if basePath == "" {
		basePath = "./storage"
	}
	dir := filepath.Join(basePath, "activity", dateStr, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}

	filename := buildStoredFilename(file, attendanceID, now)
	fullPath := filepath.Join(dir, filename)

	if err := c.SaveFile(file, fullPath); err != nil {
		return HttpError(c, responses.InternalServerError(err))
	}

	photoURL := fmt.Sprintf("/static/activity/%s/%d/%s", dateStr, userID, filename)

	photo, err := app.Services.Satpam.CreateActivityPhoto(c.Context(), userID, attendanceID, note, photoURL, now, lat, lng)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, photo)
}

// SatpamAttendanceHistory returns attendance history for the authenticated satpam.
func SatpamAttendanceHistory(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	startStr := c.Query("start_date", "")
	endStr := c.Query("end_date", "")

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var startDate time.Time
	var endDate time.Time

	if startStr == "" && endStr == "" {
		endDate = today
		startDate = today.AddDate(0, 0, -30)
	} else {
		if startStr == "" || endStr == "" {
			return HttpError(c, responses.BadRequest(fmt.Errorf("start_date and end_date are required together")))
		}
		sd, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid start_date, expected YYYY-MM-DD")))
		}
		ed, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid end_date, expected YYYY-MM-DD")))
		}
		startDate = sd
		endDate = ed
	}

	items, err := app.Services.Satpam.ListAttendanceHistory(c.Context(), userID, startDate, endDate)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, items)
}

// SatpamCreateShiftSwapRequest handles creation of shift swap request.
func SatpamCreateShiftSwapRequest(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req shiftSwapRequestBody
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid body")))
	}
	if strings.TrimSpace(req.TargetUserID) == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("target_user_id is required")))
	}
	targetUserID, err := strconv.ParseInt(req.TargetUserID, 10, 64)
	if err != nil || targetUserID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid target_user_id")))
	}
	if strings.TrimSpace(req.ShiftDate) == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("shift_date is required")))
	}
	shiftDate, err := time.Parse("2006-01-02", req.ShiftDate)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid shift_date format, expected YYYY-MM-DD")))
	}

	reason := strings.TrimSpace(req.Reason)

	r, err := app.Services.Satpam.CreateShiftSwapRequest(c.Context(), userID, targetUserID, shiftDate, reason)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, r)
}

// SatpamListMyShiftSwapDates returns upcoming shift dates for current satpam.
func SatpamListMyShiftSwapDates(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	fromStr := strings.TrimSpace(c.Query("from", ""))
	var from time.Time
	if fromStr == "" {
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		d, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid from format, expected YYYY-MM-DD")))
		}
		from = d
	}

	dates, err := app.Services.Satpam.ListMyShiftDates(c.Context(), userID, from)
	if err != nil {
		return HttpError(c, err)
	}

	out := make([]string, 0, len(dates))
	for _, d := range dates {
		out = append(out, d.Format("2006-01-02"))
	}

	return HttpSuccess(c, out)
}

// SatpamListShiftSwapPeers returns list of other satpam who have shift on given date.
func SatpamListShiftSwapPeers(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	dateStr := strings.TrimSpace(c.Query("date", ""))
	if dateStr == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("date is required (YYYY-MM-DD)")))
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid date format, expected YYYY-MM-DD")))
	}

	users, err := app.Services.Satpam.ListShiftSwapPeers(c.Context(), userID, d)
	if err != nil {
		return HttpError(c, err)
	}

	resp := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		resp = append(resp, fiber.Map{
			"id":    u.ID,
			"name":  u.Name,
			"email": u.Email,
		})
	}

	return HttpSuccess(c, resp)
}

// SatpamListShiftSwapRequests lists swap requests for the logged-in user.
func SatpamListShiftSwapRequests(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return HttpError(c, err)
	}

	status := strings.TrimSpace(c.Query("status", ""))
	if status != "" {
		switch status {
		case "PENDING", "APPROVED", "REJECTED":
		default:
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid status")))
		}
	}

	list, err := app.Services.Satpam.ListShiftSwapRequests(c.Context(), userID, status)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, list)
}

func validLat(lat float64) bool {
	return lat >= -90 && lat <= 90
}

func validLng(lng float64) bool {
	return lng >= -180 && lng <= 180
}

func validatePhotoFile(file *multipart.FileHeader) error {
	const maxSize = 5 * 1024 * 1024
	if file.Size > maxSize {
		return fmt.Errorf("file too large, max 5MB")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png":
	default:
		return fmt.Errorf("invalid file type, only jpg, jpeg, png allowed")
	}
	return nil
}

func buildStoredFilename(file *multipart.FileHeader, attendanceID int64, now time.Time) string {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	return fmt.Sprintf("%d_%d%s", attendanceID, now.UnixNano(), ext)
}

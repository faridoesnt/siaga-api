package handlers

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"siaga-api/api/models/responses"

	"github.com/gofiber/fiber/v2"
)

// Admin helpers

func getAdminID(c *fiber.Ctx) (int64, error) {
	val := c.Locals("user_id")
	if id, ok := val.(int64); ok {
		return id, nil
	}
	return 0, responses.UnAuthorized(fmt.Errorf("invalid user context"))
}

func getPaginationParams(c *fiber.Ctx) (limit, offset int, err error) {
	sizeStr := c.Query("page_size", "")
	if sizeStr == "" {
		return 0, 0, nil
	}

	pageSize, err := strconv.Atoi(sizeStr)
	if err != nil || pageSize <= 0 {
		return 0, 0, responses.BadRequest(fmt.Errorf("invalid page_size"))
	}

	pageStr := c.Query("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		return 0, 0, responses.BadRequest(fmt.Errorf("invalid page"))
	}

	limit = pageSize
	offset = (page - 1) * pageSize
	return limit, offset, nil
}

// Import / Export helpers

func AdminDownloadSatpamTemplate(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	data, err := app.Services.Admin.GenerateSatpamImportTemplate(c.Context())
	if err != nil {
		return HttpError(c, err)
	}

	filename := fmt.Sprintf("satpam_import_template_%s.xlsx", time.Now().Format("20060102"))
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(fiber.StatusOK).Send(data)
}

func AdminDownloadShiftTemplate(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	data, err := app.Services.Admin.GenerateShiftImportTemplate(c.Context())
	if err != nil {
		return HttpError(c, err)
	}

	filename := fmt.Sprintf("shift_import_template_%s.xlsx", time.Now().Format("20060102"))
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(fiber.StatusOK).Send(data)
}

func AdminImportSatpam(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("file is required")))
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".xlsx" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("file must be .xlsx")))
	}

	f, err := fileHeader.Open()
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("failed to open uploaded file")))
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("failed to read uploaded file")))
	}

	inserted, err := app.Services.Admin.ImportSatpamFromExcel(c.Context(), adminID, data)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"inserted_count": inserted,
	})
}

func AdminImportShifts(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("file is required")))
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".xlsx" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("file must be .xlsx")))
	}

	f, err := fileHeader.Open()
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("failed to open uploaded file")))
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("failed to read uploaded file")))
	}

	inserted, err := app.Services.Admin.ImportShiftsFromExcel(c.Context(), adminID, data)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"inserted_count": inserted,
	})
}

func AdminExportSatpam(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	data, err := app.Services.Admin.ExportSatpamToExcel(c.Context())
	if err != nil {
		return HttpError(c, err)
	}

	filename := fmt.Sprintf("satpam_export_%s.xlsx", time.Now().Format("20060102"))
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(fiber.StatusOK).Send(data)
}

func AdminExportAttendanceMonitoring(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	startStr := c.Query("start_date", "")
	endStr := c.Query("end_date", "")
	if startStr == "" || endStr == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("start_date and end_date are required")))
	}

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid start_date format, expected YYYY-MM-DD")))
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid end_date format, expected YYYY-MM-DD")))
	}

	data, err := app.Services.Admin.ExportAttendanceMonitoringToExcel(c.Context(), start, end)
	if err != nil {
		return HttpError(c, err)
	}

	filename := fmt.Sprintf("attendance_export_%s_%s.xlsx", start.Format("20060102"), end.Format("20060102"))
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(fiber.StatusOK).Send(data)
}

// 3) CREATE SATPAM

type adminCreateSatpamRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	WorkStartDate string `json:"work_start_date"`
}

func AdminCreateSatpam(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req adminCreateSatpamRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("email, password and name are required")))
	}

	var workStart *time.Time
	if req.WorkStartDate != "" {
		d, err := time.Parse("2006-01-02", req.WorkStartDate)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid work_start_date format, expected YYYY-MM-DD")))
		}
		workStart = &d
	}

	user, err := app.Services.Admin.CreateSatpam(c.Context(), adminID, req.Email, req.Password, req.Name, workStart)
	if err != nil {
		return HttpError(c, err)
	}

	var workStartStr string
	if user.WorkStartDate != nil {
		workStartStr = user.WorkStartDate.Format("2006-01-02")
	}

	resp := fiber.Map{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"work_start_date": workStartStr,
		"is_active":       user.Active,
	}

	return HttpSuccess(c, resp)
}

// 4) LIST SATPAM

func AdminListSatpam(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	activeParam := c.Query("active", "")
	var active *bool
	if activeParam != "" {
		val, err := strconv.ParseBool(activeParam)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid active value")))
		}
		active = &val
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	users, err := app.Services.Admin.ListSatpam(c.Context(), active, limit, offset)
	if err != nil {
		return HttpError(c, err)
	}

	items := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		var workStartStr string
		if u.WorkStartDate != nil {
			workStartStr = u.WorkStartDate.Format("2006-01-02")
		}
		items = append(items, fiber.Map{
			"id":              u.ID,
			"email":           u.Email,
			"name":            u.Name,
			"work_start_date": workStartStr,
			"is_active":       u.Active,
		})
	}

	return HttpSuccess(c, items)
}

// 5) ENABLE / DISABLE SATPAM

type satpamStatusRequest struct {
	IsActive *bool `json:"is_active"`
}

func AdminSetSatpamStatus(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req satpamStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.IsActive == nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("is_active is required")))
	}

	if err := app.Services.Admin.SetSatpamActive(c.Context(), adminID, userID, *req.IsActive); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id":        userID,
		"is_active": *req.IsActive,
	})
}

type adminUpdateSatpamRequest struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	WorkStartDate string `json:"work_start_date"`
}

func AdminUpdateSatpam(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req adminUpdateSatpamRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.Email == "" || req.Name == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("email and name are required")))
	}

	var workStart *time.Time
	if req.WorkStartDate != "" {
		d, err := time.Parse("2006-01-02", req.WorkStartDate)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid work_start_date format, expected YYYY-MM-DD")))
		}
		workStart = &d
	}

	user, err := app.Services.Admin.UpdateSatpam(c.Context(), adminID, userID, req.Email, req.Name, workStart)
	if err != nil {
		return HttpError(c, err)
	}

	var workStartStr string
	if user.WorkStartDate != nil {
		workStartStr = user.WorkStartDate.Format("2006-01-02")
	}

	resp := fiber.Map{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"work_start_date": workStartStr,
		"is_active":       user.Active,
	}

	return HttpSuccess(c, resp)
}

func AdminDeleteSatpam(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteSatpam(c.Context(), adminID, userID); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id": userID,
	})
}

// 6) ATTENDANCE SPOTS

type createAttendanceSpotRequest struct {
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters int     `json:"radius_meter"`
}

func AdminCreateAttendanceSpot(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req createAttendanceSpotRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	spot, err := app.Services.Admin.CreateAttendanceSpot(c.Context(), req.Name, req.Latitude, req.Longitude, req.RadiusMeters)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, spot)
}

func AdminListAttendanceSpots(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	spots, err := app.Services.Admin.ListAttendanceSpots(c.Context(), limit, offset)
	if err != nil {
		return HttpError(c, err)
	}
	return HttpSuccess(c, spots)
}

func AdminUpdateAttendanceSpot(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req createAttendanceSpotRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	spot, err := app.Services.Admin.UpdateAttendanceSpot(c.Context(), id, req.Name, req.Latitude, req.Longitude, req.RadiusMeters)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, spot)
}

func AdminDeleteAttendanceSpot(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteAttendanceSpot(c.Context(), id); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id": id,
	})
}

type assignUserAttendanceSpotRequest struct {
	UserID           int64  `json:"user_id"`
	AttendanceSpotID int64  `json:"attendance_spot_id"`
	ActiveFrom       string `json:"active_from"`
}

func AdminAssignUserAttendanceSpot(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req assignUserAttendanceSpotRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.UserID <= 0 || req.AttendanceSpotID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("user_id and attendance_spot_id are required")))
	}
	if req.ActiveFrom == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("active_from is required")))
	}
	activeFrom, err := time.Parse("2006-01-02", req.ActiveFrom)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid active_from format, expected YYYY-MM-DD")))
	}

	if err := app.Services.Admin.AssignUserAttendanceSpot(c.Context(), adminID, req.UserID, req.AttendanceSpotID, activeFrom); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"user_id":            req.UserID,
		"attendance_spot_id": req.AttendanceSpotID,
		"active_from":        activeFrom.Format("2006-01-02"),
	})
}

// 7) SHIFT MANAGEMENT

type createShiftRequest struct {
	Name                string `json:"name"`
	StartTime           string `json:"start_time"`
	EndTime             string `json:"end_time"`
	LateToleranceMinute int    `json:"late_tolerance_minute"`
}

func AdminCreateShift(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req createShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	if req.StartTime == "" || req.EndTime == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("start_time and end_time are required")))
	}

	start, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid start_time format, expected HH:MM")))
	}
	end, err := time.Parse("15:04", req.EndTime)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid end_time format, expected HH:MM")))
	}

	shift, err := app.Services.Admin.CreateShift(c.Context(), req.Name, start, end, req.LateToleranceMinute)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, shift)
}

func AdminListShifts(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	shifts, err := app.Services.Admin.ListShifts(c.Context(), limit, offset)
	if err != nil {
		return HttpError(c, err)
	}
	return HttpSuccess(c, shifts)
}

func AdminUpdateShift(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req createShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	if req.StartTime == "" || req.EndTime == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("start_time and end_time are required")))
	}

	start, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid start_time format, expected HH:MM")))
	}
	end, err := time.Parse("15:04", req.EndTime)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid end_time format, expected HH:MM")))
	}

	shift, err := app.Services.Admin.UpdateShift(c.Context(), id, req.Name, start, end, req.LateToleranceMinute)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, shift)
}

func AdminDeleteShift(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteShift(c.Context(), id); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id": id,
	})
}

// 8) ASSIGN SHIFT TO SATPAM

type assignUserShiftRequest struct {
	UserID    int64  `json:"user_id"`
	ShiftID   int64  `json:"shift_id"`
	ShiftDate string `json:"shift_date"`
}

func AdminAssignUserShift(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req assignUserShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.UserID <= 0 || req.ShiftID <= 0 || req.ShiftDate == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("user_id, shift_id and shift_date are required")))
	}
	shiftDate, err := time.Parse("2006-01-02", req.ShiftDate)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid shift_date format, expected YYYY-MM-DD")))
	}

	us, err := app.Services.Admin.AssignUserShift(c.Context(), adminID, req.UserID, req.ShiftID, shiftDate)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, us)
}

// List user shifts (for scheduling view)
func AdminListUserShifts(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	dateStr := c.Query("date", "")
	var datePtr *time.Time
	if dateStr != "" {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid date format, expected YYYY-MM-DD")))
		}
		datePtr = &d
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	rows, err := app.Services.Admin.ListUserShifts(c.Context(), datePtr, limit, offset)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, rows)
}

type updateUserShiftRequest struct {
	ShiftID   int64  `json:"shift_id"`
	ShiftDate string `json:"shift_date"`
}

func AdminUpdateUserShift(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req updateUserShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.ShiftID <= 0 || req.ShiftDate == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("shift_id and shift_date are required")))
	}

	shiftDate, err := time.Parse("2006-01-02", req.ShiftDate)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid shift_date format, expected YYYY-MM-DD")))
	}

	us, err := app.Services.Admin.UpdateUserShiftAssignment(c.Context(), adminID, id, req.ShiftID, shiftDate)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, us)
}

func AdminDeleteUserShift(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteUserShiftAssignment(c.Context(), adminID, id); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id": id,
	})
}

// 9) SHIFT SWAP APPROVAL

func AdminListShiftSwapRequests(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	status := c.Query("status", "")
	if status != "" {
		switch status {
		case "PENDING", "APPROVED", "REJECTED":
		default:
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid status")))
		}
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	reqs, err := app.Services.Admin.ListShiftSwapRequests(c.Context(), status, limit, offset)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, reqs)
}

func AdminApproveShiftSwapRequest(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	requestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || requestID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	req, err := app.Services.Admin.ApproveShiftSwapRequest(c.Context(), adminID, requestID)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, req)
}

type rejectShiftSwapRequest struct {
	Note string `json:"note"`
}

func AdminRejectShiftSwapRequest(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	requestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || requestID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var reqBody rejectShiftSwapRequest
	if err := c.BodyParser(&reqBody); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	req, err := app.Services.Admin.RejectShiftSwapRequest(c.Context(), adminID, requestID, reqBody.Note)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, req)
}

// 10) ATTENDANCE MONITORING

func AdminListDailyAttendance(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	dateStr := c.Query("date", "")
	var date time.Time
	if dateStr == "" {
		now := time.Now()
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid date format, expected YYYY-MM-DD")))
		}
		date = d
	}

	rows, err := app.Services.Admin.ListDailyAttendance(c.Context(), date)
	if err != nil {
		return HttpError(c, err)
	}

	respItems := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		var clockInSpot fiber.Map
		if r.ClockInSpotID != nil && r.ClockInSpotName != nil {
			clockInSpot = fiber.Map{
				"id":   *r.ClockInSpotID,
				"name": *r.ClockInSpotName,
			}
		}
		var clockOutSpot fiber.Map
		if r.ClockOutSpotID != nil && r.ClockOutSpotName != nil {
			clockOutSpot = fiber.Map{
				"id":   *r.ClockOutSpotID,
				"name": *r.ClockOutSpotName,
			}
		}

		activityItems := make([]fiber.Map, 0, len(r.Activities))
		for _, a := range r.Activities {
			var spot fiber.Map
			if a.AttendanceSpotID != nil && a.AttendanceSpotName != nil {
				spot = fiber.Map{
					"id":   *a.AttendanceSpotID,
					"name": *a.AttendanceSpotName,
				}
			}
			activityItems = append(activityItems, fiber.Map{
				"id":        a.ID,
				"photo_url": a.PhotoURL,
				"note":      a.Note,
				"taken_at":  a.TakenAt,
				"spot":      spot,
			})
		}

		respItems = append(respItems, fiber.Map{
			"attendance_id": r.AttendanceID,
			"user": fiber.Map{
				"id":   r.UserID,
				"name": r.UserName,
			},
			"shift": fiber.Map{
				"name": r.ShiftName,
			},
			"clock_in_time":       r.ClockInTime,
			"clock_out_time":      r.ClockOutTime,
			"clock_in_photo_url":  r.ClockInPhoto,
			"clock_out_photo_url": r.ClockOutPhoto,
			"status":              r.ClockInStatus,
			"face_verified":       r.FaceVerified,
			"face_match_score":    r.FaceMatchScore,
			"clock_in": fiber.Map{
				"time": r.ClockInTime,
				"spot": clockInSpot,
			},
			"clock_out": fiber.Map{
				"time": r.ClockOutTime,
				"spot": clockOutSpot,
			},
			"activities": activityItems,
		})
	}

	return HttpSuccess(c, respItems)
}

// List all open attendance records (clock-out missing)
func AdminListOpenAttendance(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	rows, err := app.Services.Admin.ListOpenAttendance(c.Context())
	if err != nil {
		return HttpError(c, err)
	}

	respItems := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		var clockInSpot fiber.Map
		if r.ClockInSpotID != nil && r.ClockInSpotName != nil {
			clockInSpot = fiber.Map{
				"id":   *r.ClockInSpotID,
				"name": *r.ClockInSpotName,
			}
		}
		var clockOutSpot fiber.Map
		if r.ClockOutSpotID != nil && r.ClockOutSpotName != nil {
			clockOutSpot = fiber.Map{
				"id":   *r.ClockOutSpotID,
				"name": *r.ClockOutSpotName,
			}
		}

		respItems = append(respItems, fiber.Map{
			"attendance_id": r.AttendanceID,
			"user": fiber.Map{
				"id":   r.UserID,
				"name": r.UserName,
			},
			"shift": fiber.Map{
				"name": r.ShiftName,
			},
			"clock_in_time":       r.ClockInTime,
			"clock_out_time":      r.ClockOutTime,
			"clock_in_photo_url":  r.ClockInPhoto,
			"clock_out_photo_url": r.ClockOutPhoto,
			"status":              r.ClockInStatus,
			"face_verified":       r.FaceVerified,
			"face_match_score":    r.FaceMatchScore,
			"clock_in": fiber.Map{
				"time": r.ClockInTime,
				"spot": clockInSpot,
			},
			"clock_out": fiber.Map{
				"time": r.ClockOutTime,
				"spot": clockOutSpot,
			},
		})
	}

	return HttpSuccess(c, respItems)
}

type adminFaceEnrollRequest struct {
	UserID int64    `json:"user_id"`
	Images []string `json:"images"`
}

func AdminFaceEnroll(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	var req adminFaceEnrollRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.UserID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("user_id is required")))
	}
	if len(req.Images) == 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("images is required")))
	}

	if err := app.Services.Admin.EnrollFace(c.Context(), adminID, req.UserID, req.Images); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"user_id":     req.UserID,
		"image_count": len(req.Images),
		"status":      "enrolled",
	})
}

func AdminGetFaceEnrollStatus(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	sum, err := app.Services.Admin.GetFaceEnrollStatus(c.Context(), adminID, userID)
	if err != nil {
		return HttpError(c, err)
	}

	enrolled := sum != nil && sum.Count > 0
	var updatedAt interface{}
	if sum != nil && sum.UpdatedAt != nil {
		updatedAt = sum.UpdatedAt
	}

	return HttpSuccess(c, fiber.Map{
		"user_id":    userID,
		"enrolled":   enrolled,
		"count":      sum.Count,
		"model":      sum.Model,
		"updated_at": updatedAt,
	})
}

func AdminDeleteFaceEnroll(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteFaceEnroll(c.Context(), adminID, userID); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"user_id": userID,
		"status":  "deleted",
	})
}

type forceClockOutRequest struct {
	Reason string `json:"reason"`
}

func AdminForceClockOutAttendance(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("attendance_id")
	attID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || attID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid attendance_id")))
	}

	var req forceClockOutRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if strings.TrimSpace(req.Reason) == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("reason is required")))
	}

	if err := app.Services.Admin.ForceClockOutAttendance(c.Context(), adminID, attID, req.Reason); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"attendance_id": attID,
		"status":        "force_clocked_out",
	})
}

// List user attendance spots (for spot assignment view)
func AdminListUserAttendanceSpots(c *fiber.Ctx) error {
	_, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	dateStr := c.Query("date", "")
	var date *time.Time
	if dateStr != "" {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid date format, expected YYYY-MM-DD")))
		}
		date = &d
	}

	limit, offset, err := getPaginationParams(c)
	if err != nil {
		return HttpError(c, err)
	}

	rows, err := app.Services.Admin.ListUserAttendanceSpots(c.Context(), date, limit, offset)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, rows)
}

type updateUserAttendanceSpotRequest struct {
	AttendanceSpotID int64  `json:"attendance_spot_id"`
	ActiveFrom       string `json:"active_from"`
	ActiveUntil      string `json:"active_until"`
}

func AdminUpdateUserAttendanceSpot(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	var req updateUserAttendanceSpotRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}
	if req.AttendanceSpotID <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("attendance_spot_id is required")))
	}
	if req.ActiveFrom == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("active_from is required")))
	}

	activeFrom, err := time.Parse("2006-01-02", req.ActiveFrom)
	if err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid active_from format, expected YYYY-MM-DD")))
	}

	var activeUntil *time.Time
	if req.ActiveUntil != "" {
		u, err := time.Parse("2006-01-02", req.ActiveUntil)
		if err != nil {
			return HttpError(c, responses.BadRequest(fmt.Errorf("invalid active_until format, expected YYYY-MM-DD")))
		}
		activeUntil = &u
	}

	uas, err := app.Services.Admin.UpdateUserAttendanceSpot(c.Context(), adminID, id, req.AttendanceSpotID, activeFrom, activeUntil)
	if err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, uas)
}

func AdminDeleteUserAttendanceSpot(c *fiber.Ctx) error {
	adminID, err := getAdminID(c)
	if err != nil {
		return HttpError(c, err)
	}

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid id")))
	}

	if err := app.Services.Admin.DeleteUserAttendanceSpot(c.Context(), adminID, id); err != nil {
		return HttpError(c, err)
	}

	return HttpSuccess(c, fiber.Map{
		"id": id,
	})
}

package contracts

import (
	"context"
	"time"

	"siaga-api/api/entities"
)

type AdminService interface {
	CreateSatpam(ctx context.Context, adminID int64, payload *entities.SatpamUpsertPayload, password string) (*entities.SatpamWithProfile, error)
	ListSatpam(ctx context.Context, active *bool, limit, offset int) ([]*entities.SatpamWithProfile, error)
	SetSatpamActive(ctx context.Context, adminID, userID int64, active bool) error
	UpdateSatpam(ctx context.Context, adminID, userID int64, payload *entities.SatpamUpsertPayload) (*entities.SatpamWithProfile, error)
	DeleteSatpam(ctx context.Context, adminID, userID int64) error
	ResetSatpamPassword(ctx context.Context, adminID, userID int64, newPassword string) error

	CreateAttendanceSpot(ctx context.Context, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error)
	ListAttendanceSpots(ctx context.Context, limit, offset int) ([]*entities.AttendanceSpot, error)
	AssignUserAttendanceSpot(ctx context.Context, adminID, userID, attendanceSpotID int64, activeFrom time.Time) error

	CreateShift(ctx context.Context, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error)
	ListShifts(ctx context.Context, limit, offset int) ([]*entities.Shift, error)
	AssignUserShift(ctx context.Context, adminID, userID, shiftID int64, shiftDate time.Time) (*entities.UserShift, error)
	UpdateShift(ctx context.Context, id int64, name string, startTime, endTime time.Time, lateTolerance int) (*entities.Shift, error)
	DeleteShift(ctx context.Context, id int64) error

	ListShiftSwapRequests(ctx context.Context, status string, limit, offset int) ([]*entities.ShiftSwapRequest, error)
	ApproveShiftSwapRequest(ctx context.Context, adminID, requestID int64) (*entities.ShiftSwapRequest, error)
	RejectShiftSwapRequest(ctx context.Context, adminID, requestID int64, note string) (*entities.ShiftSwapRequest, error)

	ListDailyAttendance(ctx context.Context, date time.Time) ([]*entities.AdminAttendanceRow, error)
	ListOpenAttendance(ctx context.Context) ([]*entities.AdminAttendanceRow, error)
	ListUserShifts(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserShiftRow, error)
	ListUserAttendanceSpots(ctx context.Context, date *time.Time, limit, offset int) ([]*entities.AdminUserAttendanceSpotRow, error)

	UpdateAttendanceSpot(ctx context.Context, id int64, name string, latitude, longitude float64, radiusMeters int) (*entities.AttendanceSpot, error)
	DeleteAttendanceSpot(ctx context.Context, id int64) error

	UpdateUserShiftAssignment(ctx context.Context, adminID, id, shiftID int64, shiftDate time.Time) (*entities.UserShift, error)
	DeleteUserShiftAssignment(ctx context.Context, adminID, id int64) error

	UpdateUserAttendanceSpot(ctx context.Context, adminID, id, attendanceSpotID int64, activeFrom time.Time, activeUntil *time.Time) (*entities.UserAttendanceSpot, error)
	DeleteUserAttendanceSpot(ctx context.Context, adminID, id int64) error

	// Face enrollment
	EnrollFace(ctx context.Context, adminID, userID int64, images []string) error
	GetFaceEnrollStatus(ctx context.Context, adminID, userID int64) (*entities.FaceEmbeddingSummary, error)
	DeleteFaceEnroll(ctx context.Context, adminID, userID int64) error

	// Import / Export
	GenerateSatpamImportTemplate(ctx context.Context) ([]byte, error)
	GenerateShiftImportTemplate(ctx context.Context) ([]byte, error)
	ImportSatpamFromExcel(ctx context.Context, adminID int64, fileData []byte) (int, error)
	ImportShiftsFromExcel(ctx context.Context, adminID int64, fileData []byte) (int, error)
	ExportSatpamToExcel(ctx context.Context) ([]byte, error)
	ExportAttendanceMonitoringToExcel(ctx context.Context, startDate, endDate time.Time) ([]byte, error)

	// Scheduling template (monthly)
	GenerateSchedulingTemplate(ctx context.Context, month, year int) ([]byte, error)
	ImportSchedulingFromExcel(ctx context.Context, adminID int64, fileData []byte) (*entities.SchedulingImportResult, error)

	// Admin override clock-out
	ForceClockOutAttendance(ctx context.Context, adminID, attendanceID int64, reason string) error

	// Dashboard
	GetDashboard(ctx context.Context, month time.Time) (*entities.AdminDashboardResponse, error)

	// RBAC / Admin management
	ListPermissions(ctx context.Context) ([]*entities.Permission, error)
	ListAdmins(ctx context.Context, limit, offset int) ([]*entities.User, error)
	GetAdminWithPermissions(ctx context.Context, id int64) (*entities.User, []string, error)
	CreateAdminUser(ctx context.Context, actorID int64, email, password, name string, perms []string) (*entities.User, []string, error)
	UpdateAdminUser(ctx context.Context, actorID, id int64, email, name string, perms []string) (*entities.User, []string, error)
	DeleteAdminUser(ctx context.Context, actorID, id int64) error
	ResetAdminPassword(ctx context.Context, actorID, id int64, newPassword string) error
}

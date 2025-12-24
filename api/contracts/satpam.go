package contracts

import (
	"context"
	"time"

	"siaga-api/api/entities"
)

type SatpamService interface {
	GetMe(ctx context.Context, userID int64) (*entities.User, error)
	GetDashboard(ctx context.Context, userID int64, date time.Time) (*entities.SatpamDashboard, error)
	ClockIn(ctx context.Context, userID int64, lat, lng float64, imageBase64 string) (*entities.Attendance, error)
	ClockOut(ctx context.Context, userID int64, lat, lng float64) (*entities.Attendance, error)
	ClockOutWithPhoto(ctx context.Context, userID int64, lat, lng float64, photoURL *string, imageBase64 string) (*entities.Attendance, error)
	CreateActivityPhoto(ctx context.Context, userID, attendanceID int64, note string, photoURL string, takenAt time.Time, lat, lng float64) (*entities.DailyActivityPhoto, error)
	ListMyShiftDates(ctx context.Context, userID int64, from time.Time) ([]time.Time, error)
	ListAttendanceHistory(ctx context.Context, userID int64, startDate, endDate time.Time) ([]*entities.SatpamAttendanceHistoryItem, error)
	CreateShiftSwapRequest(ctx context.Context, userID, targetUserID int64, shiftDate time.Time, reason string) (*entities.ShiftSwapRequest, error)
	ListShiftSwapRequests(ctx context.Context, userID int64, status string) ([]*entities.ShiftSwapRequest, error)
	ListShiftSwapPeers(ctx context.Context, userID int64, date time.Time) ([]*entities.User, error)
}

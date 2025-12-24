package routers

import (
	"strconv"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/contracts"
	"siaga-api/api/handlers"
	"siaga-api/api/middlewares"

	"github.com/gofiber/fiber/v2"
)

func Init(app *contracts.App) {
	// static files (attendance & activity photos)
	basePath := app.Config[constants.StoragePath]
	if basePath == "" {
		basePath = "./storage"
	}
	app.Fiber.Static("/static", basePath)

	app.Fiber.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"status": "ok",
			},
		})
	})

	app.Fiber.Get("/api/healthcheck", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Public auth routes (no JWT).
	authGroup := app.Fiber.Group("/v1/auth")
	authGroup.Post("/login", handlers.AuthLogin)

	adminAuthGroup := app.Fiber.Group("/v1/admin/auth")
	adminAuthGroup.Post("/login", handlers.AdminAuthLogin)

	// Protected v1 API with JWT.
	jwtSecret := []byte(app.Config[constants.JWT_SECRET])
	ttlSeconds := parseInt(app.Config[constants.JWT_TTL], constants.DefaultJwtLifetime)
	jwtTTL := time.Duration(ttlSeconds) * time.Second

	v1 := app.Fiber.Group("/v1", middlewares.JWT(jwtSecret, jwtTTL))

	// SATPAM-only routes.
	v1.Get("/me", middlewares.RequireRole("SATPAM"), handlers.Me)

	satpamGroup := v1.Group("/satpam", middlewares.RequireRole("SATPAM"))
	satpamGroup.Get("/dashboard", handlers.SatpamDashboard)
	satpamGroup.Post("/attendance/clock-in", handlers.SatpamClockIn)
	satpamGroup.Post("/attendance/clock-out", handlers.SatpamClockOut)
	satpamGroup.Post("/activity-photos", handlers.SatpamUploadActivityPhoto)
	satpamGroup.Post("/shift-swap-requests", handlers.SatpamCreateShiftSwapRequest)
	satpamGroup.Get("/shift-swap-requests", handlers.SatpamListShiftSwapRequests)
	satpamGroup.Get("/attendance/history", handlers.SatpamAttendanceHistory)
	satpamGroup.Get("/shift-swap-dates", handlers.SatpamListMyShiftSwapDates)
	satpamGroup.Get("/shift-swap-peers", handlers.SatpamListShiftSwapPeers)

	// ADMIN-only routes.
	adminGroup := v1.Group("/admin", middlewares.RequireRole("ADMIN"))

	// Import / Export templates & operations
	adminGroup.Get("/import-templates/satpam", handlers.AdminDownloadSatpamTemplate)
	adminGroup.Get("/import-templates/shifts", handlers.AdminDownloadShiftTemplate)
	adminGroup.Post("/import/satpam", handlers.AdminImportSatpam)
	adminGroup.Post("/import/shifts", handlers.AdminImportShifts)
	adminGroup.Get("/export/satpam", handlers.AdminExportSatpam)
	adminGroup.Get("/export/attendance-monitoring", handlers.AdminExportAttendanceMonitoring)

	adminGroup.Post("/satpam", handlers.AdminCreateSatpam)
	adminGroup.Get("/satpam", handlers.AdminListSatpam)
	adminGroup.Patch("/satpam/:id/status", handlers.AdminSetSatpamStatus)
	adminGroup.Patch("/satpam/:id", handlers.AdminUpdateSatpam)
	adminGroup.Delete("/satpam/:id", handlers.AdminDeleteSatpam)

	adminGroup.Post("/attendance-spots", handlers.AdminCreateAttendanceSpot)
	adminGroup.Get("/attendance-spots", handlers.AdminListAttendanceSpots)
	adminGroup.Patch("/attendance-spots/:id", handlers.AdminUpdateAttendanceSpot)
	adminGroup.Delete("/attendance-spots/:id", handlers.AdminDeleteAttendanceSpot)
	adminGroup.Post("/user-attendance-spots", handlers.AdminAssignUserAttendanceSpot)
	adminGroup.Get("/user-attendance-spots", handlers.AdminListUserAttendanceSpots)
	adminGroup.Patch("/user-attendance-spots/:id", handlers.AdminUpdateUserAttendanceSpot)
	adminGroup.Delete("/user-attendance-spots/:id", handlers.AdminDeleteUserAttendanceSpot)

	adminGroup.Post("/shifts", handlers.AdminCreateShift)
	adminGroup.Get("/shifts", handlers.AdminListShifts)
	adminGroup.Patch("/shifts/:id", handlers.AdminUpdateShift)
	adminGroup.Delete("/shifts/:id", handlers.AdminDeleteShift)

	adminGroup.Post("/user-shifts", handlers.AdminAssignUserShift)
	adminGroup.Get("/user-shifts", handlers.AdminListUserShifts)
	adminGroup.Patch("/user-shifts/:id", handlers.AdminUpdateUserShift)
	adminGroup.Delete("/user-shifts/:id", handlers.AdminDeleteUserShift)

	adminGroup.Get("/shift-swap-requests", handlers.AdminListShiftSwapRequests)
	adminGroup.Post("/shift-swap-requests/:id/approve", handlers.AdminApproveShiftSwapRequest)
	adminGroup.Post("/shift-swap-requests/:id/reject", handlers.AdminRejectShiftSwapRequest)

	adminGroup.Get("/attendance", handlers.AdminListDailyAttendance)
	adminGroup.Get("/attendance/open", handlers.AdminListOpenAttendance)
	adminGroup.Post("/face-enroll", handlers.AdminFaceEnroll)
	adminGroup.Get("/face-enroll/:id", handlers.AdminGetFaceEnrollStatus)
	adminGroup.Delete("/face-enroll/:id", handlers.AdminDeleteFaceEnroll)

	adminGroup.Post("/attendance/:attendance_id/force-clock-out", handlers.AdminForceClockOutAttendance)
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	if v, err := strconv.Atoi(raw); err == nil && v > 0 {
		return v
	}
	return fallback
}

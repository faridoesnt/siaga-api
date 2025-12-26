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

	// Admin profile / RBAC
	adminGroup.Get("/me", handlers.AdminMe)

	// Import / Export templates & operations
	adminGroup.Get("/import-templates/satpam", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminDownloadSatpamTemplate)
	adminGroup.Get("/import-templates/shifts", middlewares.RequirePermissions("SHIFT_MANAGE"), handlers.AdminDownloadShiftTemplate)
	adminGroup.Post("/import/satpam", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminImportSatpam)
	adminGroup.Post("/import/shifts", middlewares.RequirePermissions("SHIFT_MANAGE"), handlers.AdminImportShifts)
	adminGroup.Get("/export/satpam", middlewares.RequirePermissions("SATPAM_VIEW"), handlers.AdminExportSatpam)
	adminGroup.Get("/export/attendance-monitoring", middlewares.RequirePermissions("ATTENDANCE_MONITORING_VIEW"), handlers.AdminExportAttendanceMonitoring)
	adminGroup.Get("/scheduling/template", middlewares.RequirePermissions("SCHEDULING_MANAGE"), handlers.AdminDownloadSchedulingTemplate)
	adminGroup.Post("/scheduling/import", middlewares.RequirePermissions("SCHEDULING_MANAGE"), handlers.AdminImportScheduling)

	// Permissions listing (for admin management)
	adminGroup.Get("/permissions", middlewares.RequirePermissions("ADMIN_VIEW"), handlers.AdminListPermissions)

	// Admin management
	adminGroup.Get("/admins", middlewares.RequirePermissions("ADMIN_VIEW"), handlers.AdminListAdmins)
	adminGroup.Post("/admins", middlewares.RequirePermissions("ADMIN_MANAGE"), handlers.AdminCreateAdmin)
	adminGroup.Get("/admins/:id", middlewares.RequirePermissions("ADMIN_VIEW"), handlers.AdminGetAdmin)
	adminGroup.Put("/admins/:id", middlewares.RequirePermissions("ADMIN_MANAGE"), handlers.AdminUpdateAdmin)
	adminGroup.Delete("/admins/:id", middlewares.RequirePermissions("ADMIN_MANAGE"), handlers.AdminDeleteAdmin)
	adminGroup.Post("/admins/:id/reset-password", middlewares.RequirePermissions("ADMIN_MANAGE"), handlers.AdminResetAdminPassword)

	adminGroup.Post("/satpam", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminCreateSatpam)
	adminGroup.Get("/satpam", middlewares.RequirePermissions("SATPAM_VIEW"), handlers.AdminListSatpam)
	adminGroup.Patch("/satpam/:id/status", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminSetSatpamStatus)
	adminGroup.Patch("/satpam/:id", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminUpdateSatpam)
	adminGroup.Delete("/satpam/:id", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminDeleteSatpam)
	adminGroup.Post("/satpam/:id/reset-password", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminResetSatpamPassword)

	adminGroup.Post("/attendance-spots", middlewares.RequirePermissions("ATTENDANCE_SPOT_MANAGE"), handlers.AdminCreateAttendanceSpot)
	adminGroup.Get("/attendance-spots", middlewares.RequirePermissions("ATTENDANCE_SPOT_VIEW"), handlers.AdminListAttendanceSpots)
	adminGroup.Patch("/attendance-spots/:id", middlewares.RequirePermissions("ATTENDANCE_SPOT_MANAGE"), handlers.AdminUpdateAttendanceSpot)
	adminGroup.Delete("/attendance-spots/:id", middlewares.RequirePermissions("ATTENDANCE_SPOT_MANAGE"), handlers.AdminDeleteAttendanceSpot)
	adminGroup.Post("/user-attendance-spots", middlewares.RequirePermissions("SPOT_ASSIGNMENT_MANAGE"), handlers.AdminAssignUserAttendanceSpot)
	adminGroup.Get("/user-attendance-spots", middlewares.RequirePermissions("SPOT_ASSIGNMENT_VIEW"), handlers.AdminListUserAttendanceSpots)
	adminGroup.Patch("/user-attendance-spots/:id", middlewares.RequirePermissions("SPOT_ASSIGNMENT_MANAGE"), handlers.AdminUpdateUserAttendanceSpot)
	adminGroup.Delete("/user-attendance-spots/:id", middlewares.RequirePermissions("SPOT_ASSIGNMENT_MANAGE"), handlers.AdminDeleteUserAttendanceSpot)

	adminGroup.Post("/shifts", middlewares.RequirePermissions("SHIFT_MANAGE"), handlers.AdminCreateShift)
	adminGroup.Get("/shifts", middlewares.RequirePermissions("SHIFT_VIEW"), handlers.AdminListShifts)
	adminGroup.Patch("/shifts/:id", middlewares.RequirePermissions("SHIFT_MANAGE"), handlers.AdminUpdateShift)
	adminGroup.Delete("/shifts/:id", middlewares.RequirePermissions("SHIFT_MANAGE"), handlers.AdminDeleteShift)

	adminGroup.Post("/user-shifts", middlewares.RequirePermissions("SCHEDULING_MANAGE"), handlers.AdminAssignUserShift)
	adminGroup.Get("/user-shifts", middlewares.RequirePermissions("SCHEDULING_VIEW"), handlers.AdminListUserShifts)
	adminGroup.Patch("/user-shifts/:id", middlewares.RequirePermissions("SCHEDULING_MANAGE"), handlers.AdminUpdateUserShift)
	adminGroup.Delete("/user-shifts/:id", middlewares.RequirePermissions("SCHEDULING_MANAGE"), handlers.AdminDeleteUserShift)

	adminGroup.Get("/shift-swap-requests", middlewares.RequirePermissions("SHIFT_SWAP_VIEW"), handlers.AdminListShiftSwapRequests)
	adminGroup.Post("/shift-swap-requests/:id/approve", handlers.AdminApproveShiftSwapRequest)
	adminGroup.Post("/shift-swap-requests/:id/reject", handlers.AdminRejectShiftSwapRequest)

	adminGroup.Get("/attendance", middlewares.RequirePermissions("ATTENDANCE_MONITORING_VIEW"), handlers.AdminListDailyAttendance)
	adminGroup.Get("/attendance/open", middlewares.RequirePermissions("ATTENDANCE_MONITORING_VIEW"), handlers.AdminListOpenAttendance)
	adminGroup.Post("/face-enroll", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminFaceEnroll)
	adminGroup.Get("/face-enroll/:id", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminGetFaceEnrollStatus)
	adminGroup.Delete("/face-enroll/:id", middlewares.RequirePermissions("SATPAM_MANAGE"), handlers.AdminDeleteFaceEnroll)

	adminGroup.Post("/attendance/:attendance_id/force-clock-out", middlewares.RequirePermissions("ATTENDANCE_MONITORING_MANAGE"), handlers.AdminForceClockOutAttendance)
	adminGroup.Get("/dashboard", middlewares.RequirePermissions("DASHBOARD_VIEW"), handlers.AdminDashboard)
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

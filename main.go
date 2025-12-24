package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/rs/zerolog"

	"siaga-api/api/config"
	"siaga-api/api/constants"
	"siaga-api/api/contracts"
	"siaga-api/api/datasources"
	"siaga-api/api/handlers"
	"siaga-api/api/helpers"
	"siaga-api/api/middlewares"
	"siaga-api/api/routers"
	"siaga-api/api/services"
	"siaga-api/migrate"

	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/idempotency"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewApp() *contracts.App {
	os.Setenv("TZ", "Asia/Jakarta")

	zerolog.TimeFieldFormat = time.DateTime
	// zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	fiberApp := fiber.New(fiber.Config{
		ErrorHandler: handlers.HttpError,
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
	})

	conf := config.Init()

	customLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	fiberApp.Use(fiberzerolog.New(
		fiberzerolog.Config{
			Logger: &customLogger,
		}),
	)

	crs := cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowHeaders: "Access-Control-Allow-Origin, Accept, content-type, X-Requested-With, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Screen, X-Forwarded-For, Content-Disposition, X-Content-Lang",
	})
	fiberApp.Use(crs)

	// Simple rate limiter configurable via env.
	maxRequests := parseInt(conf[constants.RateLimitRequests], 200)
	window := parseDuration(conf[constants.RateLimitWindow], time.Minute)
	fiberApp.Use(limiter.New(limiter.Config{
		Max:        maxRequests,
		Expiration: window,
	}))

	fiberApp.Use(idempotency.New())
	fiberApp.Use(healthcheck.New())
	fiberApp.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	// for debugging purposes only
	if conf[constants.ServerEnv] == constants.EnvDevelopment {
		fiberApp.Use(pprof.New())
	}

	customLogger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	app := &contracts.App{
		Fiber:  fiberApp,
		Config: conf,
		Logger: &customLogger,
	}

	app.Ds = datasources.Init(app.Config)
	app.Services = services.Init(app)

	middlewares.Init(app)
	handlers.Init(app)
	routers.Init(app)
	helpers.Init(app)

	return app
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		app := NewApp()
		migrator := migrate.NewMigrator(app.Ds.WriterDB.DB, "migrations")

		if err := migrator.RunMigrations(); err != nil {
			panic(err)
		}
		fmt.Println("Migration complete")
		return
	}

	app := NewApp()

	if err := app.Fiber.Listen(":" + app.Config[constants.ServerPort]); err != nil {
		app.Logger.Fatal().Err(err).Msg("Fiber app error")
	}
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
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

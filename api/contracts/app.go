package contracts

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type App struct {
	Config   map[string]string
	Ds       *Datasources
	Fiber    *fiber.App
	Logger   *zerolog.Logger
	Services *Services
}

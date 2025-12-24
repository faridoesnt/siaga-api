package config

import (
	"fmt"
	"os"

	"siaga-api/api/constants"

	"github.com/joho/godotenv"
)

func Init() map[string]string {
	cfg := make(map[string]string)

	_ = godotenv.Load()

	required := []string{
		constants.ServerEnv,
		constants.ServerPort,

		constants.DbDialeg,
		constants.DbHostWriter,
		constants.DbHostReader,
		constants.DbPort,
		constants.DbName,
		constants.DbUser,
		constants.DbPass,

		constants.JWT_SECRET,
		constants.REFRESH_SECRET,
		constants.JWT_TTL,
		constants.REFRESH_TTL,

		constants.FaceServiceURL,
		constants.FaceVerifyBypass,

		// constants.ResendAPIKey,
		// constants.ResendWebhookSecret,
	}

	for _, key := range required {
		val := os.Getenv(key)
		if val == "" {
			panic(fmt.Sprintf("❌ ENV %s is required but not set", key))
		}
		cfg[key] = val
	}

	setDefault(cfg, constants.REFRESH_SECRET, "")
	setDefault(cfg, constants.JWT_TTL, "900")
	setDefault(cfg, constants.REFRESH_TTL, "604800")

	setDefault(cfg, constants.RateLimitRequests, "")
	setDefault(cfg, constants.RateLimitWindow, "")

	setDefault(cfg, constants.FaceServiceURL, "http://siaga-face:8000")
	setDefault(cfg, constants.FaceVerifyBypass, "false")
	setDefault(cfg, constants.StoragePath, "./storage")
	setDefault(cfg, constants.MigrationsDir, "./migrations")

	return cfg
}

func setDefault(cfg map[string]string, key, def string) {
	if v := os.Getenv(key); v != "" {
		cfg[key] = v
		return
	}
	if _, ok := cfg[key]; !ok {
		cfg[key] = def
	}
}

package constants

// Database
const (
	DbDialeg     = "DB_DIALEG"
	DbHostWriter = "DB_HOSTWRITER"
	DbHostReader = "DB_HOSTREADER"
	DbPort       = "DB_PORT"
	DbName       = "DB_NAME"
	DbUser       = "USERNAME"
	DbPass       = "PASSWORD"
)
const (
	JWT_SECRET     = "JWT_SECRET"
	REFRESH_SECRET = "REFRESH_SECRET"
	JWT_TTL        = "JWT_TTL"
	REFRESH_TTL    = "REFRESH_TTL"
)

const (
	DefaultJwtLifetime = 3600
)

const (
	AuthClaims = "AUTH_CLAIMS"
)
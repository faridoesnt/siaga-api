package middlewares

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"siaga-api/api/entities"
	"siaga-api/api/models/responses"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWT protects private routes and renews access token on each request (sliding expiration).
func JWT(secret []byte, ttl time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return unauthorized(c, "missing or invalid Authorization header")
		}
		rawToken := strings.TrimSpace(parts[1])
		if rawToken == "" {
			return unauthorized(c, "missing token")
		}

		token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			return unauthorized(c, "invalid token")
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return unauthorized(c, "invalid token")
		}

		userID, err := extractUserID(claims)
		if err != nil {
			return unauthorized(c, "invalid token")
		}

		// Load user from DB to ensure it exists and is active.
		var user entities.User
		err = app.Ds.ReaderDB.GetContext(context.Background(), &user, `
			SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
			FROM users
			WHERE id = ?
			LIMIT 1
		`, userID)
		if err != nil {
			if err == sql.ErrNoRows {
				return unauthorized(c, "user not found")
			}
			return internalError(c)
		}
		if !user.Active {
			return forbidden(c, "user inactive")
		}

		c.Locals("user_id", user.ID)
		c.Locals("user_name", user.Name)
		c.Locals("user_email", user.Email)
		c.Locals("user_role", user.Role)
		c.Locals("user", &user)

		// Renew access token and send back via header for the client to update.
		newClaims := jwt.MapClaims{
			"sub":      user.ID,
			"user_id":  user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"role":     user.Role,
			"exp":      time.Now().Add(ttl).Unix(),
		}
		if newToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims).SignedString(secret); err == nil {
			c.Set("X-Access-Token", newToken)
		}

		return c.Next()
	}
}

// RequireRole checks role against allowed list.
func RequireRole(allowed ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("user_role").(string)
		for _, a := range allowed {
			if role == a {
				return c.Next()
			}
		}
		return forbidden(c, "forbidden")
	}
}

func unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusUnauthorized).JSON(responses.Response{
		Success: false,
		Error: &responses.ErrorBody{
			Code:    "UNAUTHORIZED",
			Message: message,
		},
	})
}

func forbidden(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusForbidden).JSON(responses.Response{
		Success: false,
		Error: &responses.ErrorBody{
			Code:    "FORBIDDEN",
			Message: message,
		},
	})
}

func internalError(c *fiber.Ctx) error {
	return c.Status(http.StatusInternalServerError).JSON(responses.Response{
		Success: false,
		Error: &responses.ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		},
	})
}

func extractUserID(claims jwt.MapClaims) (int64, error) {
	v, ok := claims["sub"]
	if !ok {
		return 0, fmt.Errorf("missing sub")
	}

	switch id := v.(type) {
	case float64:
		return int64(id), nil
	case int64:
		return id, nil
	case string:
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("invalid sub type")
	}
}

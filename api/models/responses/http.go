package responses

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
}

type ErrorResponse struct {
	Status int
	Code   string
	Err    error
}

func (e *ErrorResponse) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func newError(status int, code string, err error) *ErrorResponse {
	return &ErrorResponse{
		Status: status,
		Code:   code,
		Err:    err,
	}
}

func Conflict(err error) *ErrorResponse {
	return newError(fiber.StatusConflict, "CONFLICT", err)
}

func BadRequest(err error) *ErrorResponse {
	return newError(fiber.StatusBadRequest, "BAD_REQUEST", err)
}

func Forbidden(err error) *ErrorResponse {
	return newError(fiber.StatusForbidden, "FORBIDDEN", err)
}

func InternalServerError(err error) *ErrorResponse {
	return newError(fiber.StatusInternalServerError, "INTERNAL_ERROR", err)
}

func NotFound(err error) *ErrorResponse {
	return newError(fiber.StatusNotFound, "NOT_FOUND", err)
}

func UnAuthorized(err error) *ErrorResponse {
	return newError(fiber.StatusUnauthorized, "UNAUTHORIZED", err)
}

// IsDuplicateErr returns true if error is MySQL duplicate key.
func IsDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		if me.Number == 1062 {
			return true
		}
	}
	if strings.Contains(err.Error(), "Duplicate entry") {
		return true
	}
	return false
}

// IsForeignKeyErr returns true if error is a MySQL foreign key constraint error.
func IsForeignKeyErr(err error) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		// 1451: Cannot delete or update a parent row: a foreign key constraint fails
		// 1452: Cannot add or update a child row: a foreign key constraint fails
		if me.Number == 1451 || me.Number == 1452 {
			return true
		}
	}
	if strings.Contains(err.Error(), "a foreign key constraint fails") {
		return true
	}
	return false
}

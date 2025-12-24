package handlers

import (
	"fmt"

	"siaga-api/api/models/responses"

	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthLogin authenticates user and returns access token + profile.
func AuthLogin(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	if req.Email == "" || req.Password == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("email and password are required")))
	}

	result, err := app.Services.Auth.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return HttpError(c, err)
	}

	user := result.User

	resp := fiber.Map{
		"access_token": result.AccessToken,
		"user": fiber.Map{
			"id":    user.ID,
			"role":  user.Role,
			"email": user.Email,
			"name":  user.Name,
		},
	}

	return HttpSuccess(c, resp)
}

// AdminAuthLogin authenticates admin user and returns access token + profile.
func AdminAuthLogin(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return HttpError(c, responses.BadRequest(fmt.Errorf("invalid request body")))
	}

	if req.Email == "" || req.Password == "" {
		return HttpError(c, responses.BadRequest(fmt.Errorf("email and password are required")))
	}

	result, err := app.Services.Auth.LoginAdmin(c.Context(), req.Email, req.Password)
	if err != nil {
		return HttpError(c, err)
	}

	user := result.User

	resp := fiber.Map{
		"access_token": result.AccessToken,
		"user": fiber.Map{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	}

	return HttpSuccess(c, resp)
}

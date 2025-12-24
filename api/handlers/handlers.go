package handlers

import (
	"errors"

	"siaga-api/api/models/responses"

	"github.com/gofiber/fiber/v2"
)

func HttpError(c *fiber.Ctx, err error) error {
	var errResponse *responses.ErrorResponse
	if errors.As(err, &errResponse) {
		status := errResponse.Status
		if status == 0 {
			status = fiber.StatusInternalServerError
		}
		c.Status(status)

		return c.JSON(responses.Response{
			Success: false,
			Error: &responses.ErrorBody{
				Code:    errResponse.Code,
				Message: errResponse.Error(),
			},
		})
	}

	c.Status(fiber.StatusInternalServerError)
	return c.JSON(responses.Response{
		Success: false,
		Error: &responses.ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		},
	})
}

func HttpSuccess(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(responses.Response{
		Success: true,
		Data:    data,
	})
}

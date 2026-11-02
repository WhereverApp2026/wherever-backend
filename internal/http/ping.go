package httpapi

import "github.com/gofiber/fiber/v2"

type pingResponse struct {
	Status string `json:"estado"`
}

// ping confirma que /api/v1 resuelve.
func ping(c *fiber.Ctx) error {
	return c.JSON(pingResponse{Status: "pong"})
}

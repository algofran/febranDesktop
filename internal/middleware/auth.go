package middleware

import (
	"github.com/algofran/febranDesktop/internal/config"
	"github.com/gofiber/fiber/v2"
)

func Auth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Get("X-Gateway-Key")
		if key != cfg.GatewayAPIKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		return c.Next()
	}
}

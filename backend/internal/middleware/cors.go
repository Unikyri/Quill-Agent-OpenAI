package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func CORSMiddleware(allowedOrigins string) fiber.Handler {
	origins := strings.Split(allowedOrigins, ",")
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		allowed := false
		for _, o := range origins {
			if strings.TrimSpace(o) == origin {
				allowed = true
				break
			}
		}
		if allowed {
			c.Set("Access-Control-Allow-Origin", origin)
		}
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization")
		c.Set("Access-Control-Allow-Credentials", "true")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

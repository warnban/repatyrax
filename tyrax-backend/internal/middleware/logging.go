package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RequestLogger emits one structured slog line per request.
// Must be registered BEFORE JWTAuth so that, by the time c.Next() returns,
// the "user_id" local has been populated for protected routes.
func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			userID = "-"
		}

		logger.Info("request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("user_id", userID),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
			slog.String("ip", c.IP()),
		)

		return err
	}
}

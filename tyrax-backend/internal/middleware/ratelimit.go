package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// limitReached is the shared on-brand 429 response.
func limitReached(c *fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"status":  "error",
		"message": "RATE LIMIT EXCEEDED. STAND DOWN.",
	})
}

// AuthRateLimiter throttles /auth/* endpoints: 10 requests per minute per IP.
// Stops credential stuffing and brute-force entry attempts.
func AuthRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          10,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: limitReached,
	})
}

// UserRateLimiter throttles authenticated traffic: 100 requests per minute,
// keyed by authenticated user_id (set by JWTAuth) and falling back to IP.
func UserRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
				return uid
			}
			return c.IP()
		},
		LimitReached: limitReached,
	})
}

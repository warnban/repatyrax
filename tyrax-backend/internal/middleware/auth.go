package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type TyraxClaims struct {
	UserID           string `json:"user_id"`
	SubscriptionTier string `json:"subscription_tier"`
	Email            string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

func JWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Admin panel uses its own JWT on /api/v1/admin/*. The user JWT mount
		// on /api/v1/ (Fiber Group("/", middleware)) matches that prefix too.
		if strings.HasPrefix(c.Path(), "/api/v1/admin") {
			return c.Next()
		}
		if strings.HasPrefix(c.Path(), "/api/v1/partner") {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenStr, &TyraxClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		claims, ok := token.Claims.(*TyraxClaims)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "IDENTITY NOT FOUND")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("subscription_tier", claims.SubscriptionTier)

		return c.Next()
	}
}

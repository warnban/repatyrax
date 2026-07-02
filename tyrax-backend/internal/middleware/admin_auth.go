package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const AdminRole = "admin"

type AdminClaims struct {
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func AdminJWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &AdminClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		claims, ok := token.Claims.(*AdminClaims)
		if !ok || claims.Role != AdminRole {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			return fiber.NewError(fiber.StatusUnauthorized, "SESSION EXPIRED")
		}

		c.Locals("admin_username", claims.Username)
		return c.Next()
	}
}

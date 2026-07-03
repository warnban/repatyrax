package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const PartnerRole = "partner"

type PartnerClaims struct {
	Role      string `json:"role"`
	PartnerID string `json:"partner_id"`
	Email     string `json:"email"`
	jwt.RegisteredClaims
}

func PartnerJWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &PartnerClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}

		claims, ok := token.Claims.(*PartnerClaims)
		if !ok || claims.Role != PartnerRole {
			return fiber.NewError(fiber.StatusUnauthorized, "ACCESS DENIED")
		}
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			return fiber.NewError(fiber.StatusUnauthorized, "SESSION EXPIRED")
		}

		c.Locals("partner_id", claims.PartnerID)
		c.Locals("partner_email", claims.Email)
		return c.Next()
	}
}

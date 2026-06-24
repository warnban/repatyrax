package handler

import "github.com/gofiber/fiber/v2"

// VPN connection event handlers — implemented in a future step.
// Auth handlers now live in auth_handler.go (AuthHandler).

func LogConnect(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNotImplemented)
}

func LogDisconnect(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNotImplemented)
}

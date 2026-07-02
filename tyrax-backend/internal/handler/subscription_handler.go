package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/tyrax/tyrax-backend/internal/service"
)

// SubscriptionHandler serves Happ-compatible subscription feeds (GET /sub/:token).
type SubscriptionHandler struct {
	happ service.HappSubscriptionService
}

func NewSubscriptionHandler(happ service.HappSubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{happ: happ}
}

func (h *SubscriptionHandler) HappFeed(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString("INVALID TOKEN")
	}

	feed, err := h.happ.RenderFeed(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("INTERNAL ERROR")
	}

	for k, v := range feed.Headers {
		c.Set(k, v)
	}
	return c.Status(feed.Status).Send(feed.Body)
}

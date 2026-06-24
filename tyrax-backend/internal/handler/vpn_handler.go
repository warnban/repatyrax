package handler

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/tyrax/tyrax-backend/internal/service"
)

type VPNHandler struct {
	vpnService service.VPNService
}

func NewVPNHandler(vpnService service.VPNService) *VPNHandler {
	return &VPNHandler{vpnService: vpnService}
}

type AddDeviceRequest struct {
	Name string `json:"name"`
}

func (h *VPNHandler) AddDevice(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	var req AddDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	cfg, err := h.vpnService.AddDevice(c.Context(), userID, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrDeviceLimitReached) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "DEVICE LIMIT REACHED"})
		}
		if errors.Is(err, service.ErrNodeUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "message": "NODE UNAVAILABLE"})
		}
		slog.Error("add device failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   cfg,
	})
}

func (h *VPNHandler) GetConfig(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	pubKey := c.Query("device_public_key")
	if pubKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	cfg, err := h.vpnService.GetConfig(c.Context(), userID, pubKey)
	if err != nil {
		if errors.Is(err, service.ErrNodeUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "message": "NODE UNAVAILABLE"})
		}
		slog.Error("get config failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   cfg,
	})
}

func (h *VPNHandler) DeleteDevice(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	deviceID := c.Params("deviceID")
	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	if err := h.vpnService.DeleteDevice(c.Context(), deviceID, userID); err != nil {
		slog.Error("delete device failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
	})
}

func (h *VPNHandler) GetDevices(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	devices, err := h.vpnService.ListDevices(c.Context(), userID)
	if err != nil {
		slog.Error("get devices failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	// Trim to the client contract (id, name, created_at) — never leak public keys.
	out := make([]fiber.Map, 0, len(devices))
	for _, d := range devices {
		out = append(out, fiber.Map{
			"id":         d.ID,
			"name":       d.Name,
			"created_at": d.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   out,
	})
}

func (h *VPNHandler) GetNodes(c *fiber.Ctx) error {
	nodes, err := h.vpnService.GetNodes(c.Context())
	if err != nil {
		slog.Error("get nodes failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   nodes,
	})
}

func (h *VPNHandler) GetSplitDomains(c *fiber.Ctx) error {
	domains, err := h.vpnService.GetSplitDomains(c.Context())
	if err != nil {
		slog.Error("get split domains failed", "err", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"domains":    domains,
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func extractUserID(c *fiber.Ctx) string {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return ""
	}
	return userID
}

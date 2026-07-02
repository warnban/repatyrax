package handler

import "github.com/gofiber/fiber/v2"

// DownloadHandler serves public release metadata for desktop clients.
type DownloadHandler struct {
	websiteURL        string
	windowsAppVersion string
}

func NewDownloadHandler(websiteURL, windowsAppVersion string) *DownloadHandler {
	return &DownloadHandler{
		websiteURL:        websiteURL,
		windowsAppVersion: windowsAppVersion,
	}
}

// WindowsLatest returns the in-app update manifest for the Windows installer.
func (h *DownloadHandler) WindowsLatest(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": h.windowsAppVersion,
		"url":     h.websiteURL + "/download/windows/TYRAX-Setup.exe",
	})
}

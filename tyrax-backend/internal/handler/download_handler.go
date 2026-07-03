package handler

import "github.com/gofiber/fiber/v2"

// DownloadHandler serves public release metadata for desktop and mobile clients.
type DownloadHandler struct {
	websiteURL        string
	windowsAppVersion string

	androidAppVersion      string
	androidAppVersionCode  int
	androidAppURL          string
	androidUpdateMandatory bool
	androidUpdateNotes     string
}

func NewDownloadHandler(
	websiteURL, windowsAppVersion string,
	androidAppVersion string,
	androidAppVersionCode int,
	androidAppURL string,
	androidUpdateMandatory bool,
	androidUpdateNotes string,
) *DownloadHandler {
	return &DownloadHandler{
		websiteURL:             websiteURL,
		windowsAppVersion:      windowsAppVersion,
		androidAppVersion:      androidAppVersion,
		androidAppVersionCode:  androidAppVersionCode,
		androidAppURL:          androidAppURL,
		androidUpdateMandatory: androidUpdateMandatory,
		androidUpdateNotes:     androidUpdateNotes,
	}
}

// WindowsLatest returns the in-app update manifest for the Windows installer.
func (h *DownloadHandler) WindowsLatest(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": h.windowsAppVersion,
		"url":     h.websiteURL + "/download/windows/TYRAX-Setup.exe",
	})
}

// AndroidLatest returns the in-app update manifest for the Android APK. The
// client compares version_code against its BuildConfig.VERSION_CODE.
func (h *DownloadHandler) AndroidLatest(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version":      h.androidAppVersion,
		"version_code": h.androidAppVersionCode,
		"url":          h.androidAppURL,
		"mandatory":    h.androidUpdateMandatory,
		"notes":        h.androidUpdateNotes,
	})
}

package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAndroidLatest(t *testing.T) {
	h := NewDownloadHandler(
		"https://tyrax.tech", "1.0.13",
		"1.2.0", 7, "https://tyrax.tech/download/android/TYRAX.apk",
		true, "COLD RELEASE NOTES",
	)

	app := fiber.New()
	app.Get("/download/android/latest.json", h.AndroidLatest)

	req := httptest.NewRequest("GET", "/download/android/latest.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid json: %v (%s)", err, string(body))
	}

	if got["version"] != "1.2.0" {
		t.Errorf("version = %v, want 1.2.0", got["version"])
	}
	// JSON numbers decode as float64.
	if code, ok := got["version_code"].(float64); !ok || int(code) != 7 {
		t.Errorf("version_code = %v, want 7", got["version_code"])
	}
	if got["url"] != "https://tyrax.tech/download/android/TYRAX.apk" {
		t.Errorf("url = %v", got["url"])
	}
	if got["mandatory"] != true {
		t.Errorf("mandatory = %v, want true", got["mandatory"])
	}
}

package telegrambot

import (
	"strings"
	"testing"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
)

func TestDeviceIcon(t *testing.T) {
	cases := map[string]string{
		"HAPP":         "🍎",
		"iPhone 15":    "🍎",
		"MacBook":      "🍎",
		"Windows-PC":   "💻",
		"Android S24":  "📱",
		"mystery-node": "📱",
	}
	for name, want := range cases {
		if got := deviceIcon(name); got != want {
			t.Errorf("deviceIcon(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestDeviceTypeLabelHapp(t *testing.T) {
	if got := deviceTypeLabel("HAPP"); !strings.Contains(got, "Happ") {
		t.Errorf("deviceTypeLabel(HAPP) = %q, want a Happ label", got)
	}
}

// TestRenderDeviceListDrillIn is the core UX guarantee: tapping a device must
// open its card (tyrax_devmenu:), NOT delete it (tyrax_del:).
func TestRenderDeviceListDrillIn(t *testing.T) {
	user := &model.User{SubscriptionTier: model.TierFree}
	devices := []model.Device{
		{ID: "dev-1", Name: "HAPP", CreatedAt: time.Now()},
		{ID: "dev-2", Name: "Windows-PC", CreatedAt: time.Now()},
	}

	text, kb := renderDeviceList(user, devices)
	if kb == nil {
		t.Fatal("expected a keyboard for a non-empty device list")
	}
	if !strings.Contains(text, "МОИ УСТРОЙСТВА") {
		t.Errorf("list text missing header: %q", text)
	}

	found := map[string]bool{}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == nil {
				continue
			}
			data := *btn.CallbackData
			if strings.HasPrefix(data, "tyrax_del:") {
				t.Errorf("device row must not delete on tap, got %q", data)
			}
			if strings.HasPrefix(data, "tyrax_devmenu:") {
				found[strings.TrimPrefix(data, "tyrax_devmenu:")] = true
			}
		}
	}
	if !found["dev-1"] || !found["dev-2"] {
		t.Errorf("expected tyrax_devmenu callbacks for both devices, got %v", found)
	}
}

func TestRenderDeviceListEmpty(t *testing.T) {
	user := &model.User{SubscriptionTier: model.TierFree}
	text, kb := renderDeviceList(user, nil)
	if kb != nil {
		t.Error("empty device list must have no keyboard")
	}
	if !strings.Contains(text, "Устройств нет") {
		t.Errorf("empty-state text unexpected: %q", text)
	}
}

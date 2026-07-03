package config

import (
	"encoding/base64"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	JWTSecret           string
	DatabaseURL         string
	TelegramToken       string
	TelegramProxy       string
	TelegramBotURL      string
	TelegramBotUsername string
	SupportEmail        string
	WebsiteURL          string
	PublicAPIURL        string
	WindowsAppVersion   string
	Port                string

	// Android in-app update manifest (served at /download/android/latest.json).
	AndroidAppVersion      string
	AndroidAppVersionCode  int
	AndroidAppURL          string
	AndroidUpdateMandatory bool
	AndroidUpdateNotes     string

	FreeKassaShopID      int
	FreeKassaAPIKey      string
	FreeKassaSecretWord2 string

	CryptoPayToken string

	AdminUsername         string
	AdminPassword         string
	AdminPasswordHash     string
	AdminJWTSecret        string
	TelegramSupportToken  string
	TelegramSupportBotURL string

	// SMTP (transactional email — registration confirmation). Timeweb corp mail.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// EmailVerificationEnabled reports whether email confirmation is active. It is
// gated on an SMTP password being present so dev/local (no credentials) skips
// verification and email registrations remain immediately usable.
func (c *Config) EmailVerificationEnabled() bool {
	return c.SMTPPassword != ""
}

func Load() *Config {
	return &Config{
		JWTSecret:           getEnv("JWT_SECRET", "change-me-in-production"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://tyrax:tyrax@localhost:5432/tyrax?sslmode=disable"),
		TelegramToken:       getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramProxy:       getEnv("TELEGRAM_PROXY", ""),
		TelegramBotURL:      getEnv("TELEGRAM_BOT_URL", "https://t.me/tyraxvpnbot"),
		TelegramBotUsername: getEnv("TELEGRAM_BOT_USERNAME", "tyraxvpnbot"),
		SupportEmail:        getEnv("SUPPORT_EMAIL", "support@tyrax.tech"),
		WebsiteURL:          getEnv("WEBSITE_URL", "https://tyrax.tech"),
		PublicAPIURL:        getEnv("PUBLIC_API_URL", "https://api.tyrax.tech"),
		WindowsAppVersion:   getEnv("WINDOWS_APP_VERSION", "1.0.14"),
		Port:                getEnv("PORT", "8080"),

		AndroidAppVersion:      getEnv("ANDROID_APP_VERSION", "1.0.1"),
		AndroidAppVersionCode:  getEnvInt("ANDROID_APP_VERSION_CODE", 2),
		AndroidAppURL:          getEnv("ANDROID_APP_URL", getEnv("WEBSITE_URL", "https://tyrax.tech")+"/download/android/TYRAX.apk"),
		AndroidUpdateMandatory: getEnvBool("ANDROID_UPDATE_MANDATORY", false),
		AndroidUpdateNotes:     getEnv("ANDROID_UPDATE_NOTES", ""),

		FreeKassaShopID:      getEnvInt("FREEKASSA_SHOP_ID", 0),
		FreeKassaAPIKey:      getEnv("FREEKASSA_API_KEY", ""),
		FreeKassaSecretWord2: getEnv("FREEKASSA_SECRET_WORD_2", ""),

		CryptoPayToken: getEnv("CRYPTO_PAY_TOKEN", ""),

		AdminUsername:         getAdminEnv("ADMIN_USERNAME"),
		AdminPassword:         getAdminEnv("ADMIN_PASSWORD"),
		AdminPasswordHash:     loadAdminPasswordHash(),
		AdminJWTSecret:        getAdminEnv("ADMIN_JWT_SECRET", getEnv("JWT_SECRET", "change-me-in-production")),
		TelegramSupportToken:  getEnv("TELEGRAM_SUPPORT_BOT_TOKEN", ""),
		TelegramSupportBotURL: getEnv("TELEGRAM_SUPPORT_BOT_URL", "https://t.me/tyrax_support_bot"),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.timeweb.ru"),
		SMTPPort:     getEnvInt("SMTP_PORT", 465),
		SMTPUsername: getEnv("SMTP_USERNAME", "support@tyrax.tech"),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "TYRAX <support@tyrax.tech>"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getAdminEnv reads admin credentials and strips whitespace/quotes/CRLF that
// often sneak in when .env is edited on Windows or pasted with quotes.
func getAdminEnv(key string, fallback ...string) string {
	raw := os.Getenv(key)
	if raw == "" && len(fallback) > 0 {
		return cleanEnvValue(fallback[0])
	}
	return cleanEnvValue(raw)
}

func cleanEnvValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "\ufeff")
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return fallback
}

// loadAdminPasswordHash reads a bcrypt hash from ADMIN_PASSWORD_HASH or, if unset,
// base64-decodes ADMIN_PASSWORD_HASH_B64. The B64 form avoids Docker Compose eating
// $ characters in .env values.
func loadAdminPasswordHash() string {
	if h := cleanEnvValue(os.Getenv("ADMIN_PASSWORD_HASH")); h != "" {
		return h
	}
	b64 := cleanEnvValue(os.Getenv("ADMIN_PASSWORD_HASH_B64"))
	if b64 == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		slog.Warn("admin: invalid ADMIN_PASSWORD_HASH_B64", slog.String("error", err.Error()))
		return ""
	}
	return string(decoded)
}

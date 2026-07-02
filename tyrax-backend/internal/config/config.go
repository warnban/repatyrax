package config

import (
	"os"
	"strconv"
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

	FreeKassaShopID      int
	FreeKassaAPIKey      string
	FreeKassaSecretWord2 string

	CryptoPayToken string

	AdminUsername         string
	AdminPasswordHash     string
	AdminJWTSecret        string
	TelegramSupportToken  string
	TelegramSupportBotURL string
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
		WindowsAppVersion:   getEnv("WINDOWS_APP_VERSION", "1.0.11"),
		Port:                getEnv("PORT", "8080"),

		FreeKassaShopID:      getEnvInt("FREEKASSA_SHOP_ID", 0),
		FreeKassaAPIKey:      getEnv("FREEKASSA_API_KEY", ""),
		FreeKassaSecretWord2: getEnv("FREEKASSA_SECRET_WORD_2", ""),

		CryptoPayToken: getEnv("CRYPTO_PAY_TOKEN", ""),

		AdminUsername:         getEnv("ADMIN_USERNAME", ""),
		AdminPasswordHash:     getEnv("ADMIN_PASSWORD_HASH", ""),
		AdminJWTSecret:        getEnv("ADMIN_JWT_SECRET", getEnv("JWT_SECRET", "change-me-in-production")),
		TelegramSupportToken:  getEnv("TELEGRAM_SUPPORT_BOT_TOKEN", ""),
		TelegramSupportBotURL: getEnv("TELEGRAM_SUPPORT_BOT_URL", "https://t.me/tyrax_support_bot"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

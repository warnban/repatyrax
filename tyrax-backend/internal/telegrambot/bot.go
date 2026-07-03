package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/config"
	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
)

// newBotAPI builds the Telegram client, optionally routing all API calls through
// TELEGRAM_PROXY. The backend runs on a Russian host where api.telegram.org is
// blocked by the ISP/RKN, so outbound Telegram traffic must egress via a proxy on
// a foreign node (e.g. socks5://ip:1080 or http://ip:8888). net/http supports both
// http(s):// and socks5:// proxy URLs natively — no extra dependency needed.
func newBotAPI(cfg *config.Config) (*tgbotapi.BotAPI, error) {
	if cfg.TelegramProxy == "" {
		return tgbotapi.NewBotAPI(cfg.TelegramToken)
	}
	proxyURL, err := url.Parse(cfg.TelegramProxy)
	if err != nil {
		return nil, fmt.Errorf("parse TELEGRAM_PROXY: %w", err)
	}
	client := &http.Client{
		Timeout: (updateTimeoutSec + 15) * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	slog.Info("telegram bot: using proxy", slog.String("scheme", proxyURL.Scheme), slog.String("host", proxyURL.Host))
	return tgbotapi.NewBotAPIWithClient(cfg.TelegramToken, tgbotapi.APIEndpoint, client)
}

const (
	updateTimeoutSec = 60
	dbOpTimeout      = 5 * time.Second

	happIOSAppStoreGlobal = "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215"
	happIOSAppStoreRU     = "https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973"
	happMacDMG            = "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.macOS.universal.dmg"
)

// Reply-keyboard button captions. The text a user sends when tapping a reply
// button equals the caption verbatim, so these constants are the routing keys.
const (
	btnAccount = "📊 МОЙ АККАУНТ"
	btnConnect = "🔌 ПОДКЛЮЧИТЬ ДЕВАЙС"
	btnDevices = "🛠 Мои устройства"
	btnBuy     = "💳 Купить / продлить"
	btnHelp    = "🆘 Помощь"
)

// Static, terminal-styled replies. We send these as plain text (no Markdown
// parse mode) on purpose: the copy contains '_', '−', '·', '#' and '@handles'
// that would otherwise need fragile escaping and could fail the send call.
const (
	msgAccessGranted = "▓▓▓ ACCESS GRANTED ▓▓▓\n\n" +
		"ИДЕНТИФИКАЦИЯ ПОДТВЕРЖДЕНА.\n" +
		"Открой приложение TYRAX — ты уже внутри системы."

	// msgWelcomeNew greets a Telegram account on first contact — the FREE identity
	// has just been provisioned, so we frame the value and push straight to connect.
	msgWelcomeNew = "▓▓▓ TYRAX ▓▓▓\n" +
		"ДОСТУП ОТКРЫТ.\n\n" +
		"3 ГБ/мес активны. Без срока. Без логов.\n" +
		"Оплата: карта РФ · СБП · крипта.\n\n" +
		"🔌 ПОДКЛЮЧИТЬ ДЕВАЙС — 2 минуты до туннеля."

	// msgWelcomeBack greets a returning identity.
	msgWelcomeBack = "▓▓▓ TYRAX ▓▓▓\n" +
		"СИСТЕМА УЗНАЛА ТЕБЯ.\n\n" +
		"Выбери действие ниже."

	msgLinkInvalid    = "❌ Ссылка недействительна или устарела."
	msgNoAccount      = "❌ Аккаунт не найден. Войди через приложение TYRAX."
	msgNoAccountShort = "❌ Аккаунт не найден"
	msgUseMenu        = "Используй меню ниже."
	msgDeviceLimit    = "❌ Достигнут лимит устройств. Удали старое в разделе 🛠 Мои устройства."
	msgDeviceDeleted  = "✅ УСТРОЙСТВО УДАЛЕНО."
	msgGenericErr     = "⚠️ Что-то пошло не так. Попробуй позже."
	msgPaymentErr     = "⚠️ Ошибка создания платежа. Попробуй позже."

	msgAndroid = "▓ ANDROID ▓\n\n" +
		"1. Скачай APK (кнопка ниже)\n" +
		"2. Установи и войди через Telegram или email\n" +
		"3. Нажми ENTER — готово"

	msgIOS = "▓ iPHONE / iPAD + HAPP ▓\n\n" +
		"1. Установи Happ — App Store ниже\n" +
		"2. Скопируй ключ подписки — кнопка «СКОПИРОВАТЬ КЛЮЧ»\n" +
		"3. Happ → + → Import from URL → вставь ключ\n" +
		"4. Обнови подписку → CONNECT\n\n" +
		"Протокол: VLESS + Reality + XHTTP"

	msgWindows = "▓ WINDOWS ▓\n\n" +
		"1. Скачай установщик (кнопка ниже)\n" +
		"2. Установи TYRAX от администратора\n" +
		"3. Войди через Telegram или email → ENTER"

	msgMac = "▓ macOS + HAPP ▓\n\n" +
		"1. Установи Happ — DMG или App Store ниже\n" +
		"2. Скопируй ключ подписки — кнопка «СКОПИРОВАТЬ КЛЮЧ»\n" +
		"3. Happ → + → Import from URL → вставь ключ\n" +
		"4. Обнови подписку → CONNECT\n\n" +
		"Протокол: VLESS + Reality + XHTTP"

	msgConnectPick = "▓ ПОДКЛЮЧЕНИЕ ▓\n\nВыбери платформу:"
)

// Bot bundles the long-lived dependencies the update loop needs.
type Bot struct {
	api        *tgbotapi.BotAPI
	db         *pgxpool.Pool
	cfg        *config.Config
	userRepo   repository.UserRepository
	deviceRepo repository.DeviceRepository
	vpnSvc     service.VPNService
	paymentSvc service.PaymentService
	happSub    service.HappSubscriptionService
}

// Start launches the Telegram bot worker. Safe to run in a goroutine: if the
// token is unset or the API rejects it, the worker logs and returns without
// affecting the rest of the server.
func Start(cfg *config.Config, db *pgxpool.Pool, vpnSvc service.VPNService, paymentSvc service.PaymentService, happSub service.HappSubscriptionService) {
	if cfg.TelegramToken == "" {
		slog.Warn("telegram bot: TELEGRAM_BOT_TOKEN unset, worker disabled")
		return
	}

	api, err := newBotAPI(cfg)
	if err != nil {
		slog.Error("telegram bot: init failed", slog.String("error", err.Error()))
		return
	}
	slog.Info("telegram bot: online", slog.String("username", api.Self.UserName))

	// Register command hints so Telegram shows the "Menu" button and /-autocomplete.
	if _, err := api.Request(tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "menu", Description: "Главное меню"},
		tgbotapi.BotCommand{Command: "account", Description: "Мой аккаунт"},
		tgbotapi.BotCommand{Command: "connect", Description: "Подключить устройство"},
		tgbotapi.BotCommand{Command: "buy", Description: "Купить / продлить"},
		tgbotapi.BotCommand{Command: "help", Description: "Помощь"},
	)); err != nil {
		slog.Warn("telegram bot: set commands failed", slog.String("error", err.Error()))
	}

	b := &Bot{
		api:        api,
		db:         db,
		cfg:        cfg,
		userRepo:   repository.NewUserRepository(db),
		deviceRepo: repository.NewDeviceRepository(db),
		vpnSvc:     vpnSvc,
		paymentSvc: paymentSvc,
		happSub:    happSub,
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = updateTimeoutSec
	for update := range api.GetUpdatesChan(u) {
		b.dispatch(update)
	}
}

func (b *Bot) dispatch(update tgbotapi.Update) {
	switch {
	case update.CallbackQuery != nil:
		b.handleCallback(update.CallbackQuery)
	case update.Message != nil:
		b.handleMessage(update.Message)
	}
}

// ── Message routing ──────────────────────────────────────────────────────────

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			b.handleStart(msg)
		case "menu":
			b.sendMainMenu(msg.Chat.ID, msgUseMenu)
		case "account":
			b.handleAccount(msg)
		case "connect":
			b.handleConnect(msg.Chat.ID)
		case "buy":
			b.handleBuyStart(msg.Chat.ID)
		case "help":
			b.sendSupportLink(msg.Chat.ID)
		default:
			b.sendMainMenu(msg.Chat.ID, msgUseMenu)
		}
		return
	}

	switch strings.TrimSpace(msg.Text) {
	case btnAccount:
		b.handleAccount(msg)
	case btnConnect:
		b.handleConnect(msg.Chat.ID)
	case btnDevices:
		b.handleDevices(msg)
	case btnBuy:
		b.handleBuyStart(msg.Chat.ID)
	case btnHelp:
		b.sendSupportLink(msg.Chat.ID)
	default:
		b.sendMainMenu(msg.Chat.ID, msgUseMenu)
	}
}

// handleStart confirms /start <token> deep links. The token's user_id column is
// a UUID FK to users.id, so we resolve (or create) the identity first and bind
// its UUID to the token — the app login flow reads that user_id back.
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	token := strings.TrimSpace(msg.CommandArguments())
	if token == "" {
		// Plain /start (bot opened directly, not via app deep link). Provision the
		// FREE identity now so every menu item (account, devices, buy) works instead
		// of dead-ending on "account not found".
		ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
		defer cancel()

		_, created, err := b.resolveUser(ctx, msg.From.ID, msg.From.UserName)
		if err != nil {
			b.fail(msg.Chat.ID, "start_no_token_provision", msg.From.ID, "", err)
			return
		}
		slog.Info("telegram bot", slog.String("action", "start_no_token"), slog.Int64("telegram_id", msg.From.ID), slog.Bool("created", created))
		if created {
			b.sendMainMenu(msg.Chat.ID, msgWelcomeNew)
		} else {
			b.sendMainMenu(msg.Chat.ID, msgWelcomeBack)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	pending, err := tokenPending(ctx, b.db, token)
	if err != nil {
		b.fail(msg.Chat.ID, "start_token_lookup", msg.From.ID, "", err)
		return
	}
	if !pending {
		b.sendText(msg.Chat.ID, msgLinkInvalid)
		return
	}

	user, created, err := b.resolveUser(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		b.fail(msg.Chat.ID, "start_resolve_user", msg.From.ID, "", err)
		return
	}

	confirmed, err := confirmToken(ctx, b.db, token, user.ID)
	if err != nil {
		b.fail(msg.Chat.ID, "start_confirm", msg.From.ID, user.ID, err)
		return
	}
	if !confirmed {
		b.sendText(msg.Chat.ID, msgLinkInvalid)
		return
	}

	slog.Info("telegram bot",
		slog.String("action", "start_confirmed"),
		slog.Int64("telegram_id", msg.From.ID),
		slog.String("user_id", user.ID),
		slog.Bool("created", created),
	)
	m := tgbotapi.NewMessage(msg.Chat.ID, msgAccessGranted)
	m.ReplyMarkup = mainMenuKeyboard()
	b.send(m)
}

func (b *Bot) handleAccount(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	user, err := b.userRepo.FindByTelegramID(ctx, msg.From.ID)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.sendText(msg.Chat.ID, msgNoAccount)
		return
	}
	if err != nil {
		b.fail(msg.Chat.ID, "account_find_user", msg.From.ID, "", err)
		return
	}

	count, err := b.deviceRepo.CountByUser(ctx, user.ID)
	if err != nil {
		b.fail(msg.Chat.ID, "account_count_devices", msg.From.ID, user.ID, err)
		return
	}

	slog.Info("telegram bot", slog.String("action", "account"), slog.Int64("telegram_id", msg.From.ID), slog.String("user_id", user.ID))

	// EffectiveTier downgrades an expired paid identity back to FREE so the card
	// never advertises benefits the user no longer has.
	tier := service.EffectiveTier(user)
	limit := service.DeviceLimit(tier)

	traffic := "∞ БЕЗЛИМИТ"
	validUntil := "АКТИВЕН"
	upsell := ""
	if tier == model.TierFree {
		usedGB := float64(user.TrafficUsedBytes) / (1024 * 1024 * 1024)
		traffic = fmt.Sprintf("%.2f / 3.00 ГБ", usedGB)
		validUntil = "БЕЗ СРОКА"
		upsell = "\n\n▓ Нужен безлимит и до 10 устройств?\nЖми 💳 Купить / продлить."
	} else if user.SubscriptionEnd != nil {
		validUntil = user.SubscriptionEnd.Format("02.01.2006")
	}

	text := fmt.Sprintf(
		"◈ TYRAX ID: #%s\n"+
			"◈ СТАТУС: %s\n"+
			"◈ УСТРОЙСТВ: %d/%d\n"+
			"◈ ТРАФИК: %s\n"+
			"◈ ДЕЙСТВУЕТ ДО: %s%s",
		shortTyraxID(user.ID), tier, count, limit, traffic, validUntil, upsell,
	)
	b.sendText(msg.Chat.ID, text)
}

// shortTyraxID renders a stable, human-readable identifier from the user UUID:
// the first 6 hex characters, uppercased (e.g. "A1B2C3"). The raw UUID is never
// shown to the user.
func shortTyraxID(id string) string {
	hexOnly := strings.ToUpper(strings.ReplaceAll(id, "-", ""))
	if len(hexOnly) >= 6 {
		return hexOnly[:6]
	}
	return hexOnly
}

func (b *Bot) handleConnect(chatID int64) {
	slog.Info("telegram bot", slog.String("action", "connect_pick"), slog.Int64("chat_id", chatID))
	m := tgbotapi.NewMessage(chatID, msgConnectPick)
	m.ReplyMarkup = devicePickerKeyboard()
	b.send(m)
}

func (b *Bot) handleAndroid(chatID int64) {
	slog.Info("telegram bot", slog.String("action", "android"), slog.Int64("telegram_id", chatID))
	m := tgbotapi.NewMessage(chatID, msgAndroid+"\n\n📖 "+b.websiteURL()+"/guides.html#android")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ СКАЧАТЬ APK", b.androidDownloadURL()),
		),
	)
	b.send(m)
}

func (b *Bot) handleWindows(chatID int64) {
	slog.Info("telegram bot", slog.String("action", "windows"), slog.Int64("telegram_id", chatID))
	m := tgbotapi.NewMessage(chatID, msgWindows+"\n\n📖 "+b.websiteURL()+"/guides.html#windows")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ СКАЧАТЬ ДЛЯ WINDOWS", b.windowsDownloadURL()),
		),
	)
	b.send(m)
}

func (b *Bot) androidDownloadURL() string {
	return b.websiteURL() + "/download/tyrax.apk"
}

func (b *Bot) windowsDownloadURL() string {
	return b.websiteURL() + "/download/windows/TYRAX-Setup.exe"
}

func (b *Bot) websiteURL() string {
	base := strings.TrimRight(b.cfg.WebsiteURL, "/")
	if base == "" {
		return "https://tyrax.tech"
	}
	return base
}

func (b *Bot) sendHappSubscription(chatID int64, from *tgbotapi.User, intro, platform string) {
	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	user, err := b.ensureUser(ctx, from)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.sendText(chatID, msgNoAccount)
		return
	}
	if err != nil {
		b.fail(chatID, "happ_find_user", from.ID, "", err)
		return
	}

	subURL, err := b.happSub.EnsureSubscriptionURL(ctx, user.ID)
	if errors.Is(err, service.ErrDeviceLimitReached) {
		b.sendText(chatID, msgDeviceLimit)
		return
	}
	if err != nil {
		b.fail(chatID, "happ_sub_url", from.ID, user.ID, err)
		return
	}

	slog.Info("telegram bot", slog.String("action", "happ_sub"), slog.String("platform", platform), slog.Int64("telegram_id", from.ID), slog.String("user_id", user.ID))

	guidesHash := "ios"
	if platform == "mac" {
		guidesHash = "mac"
	}
	text := intro + "\n\n📖 " + b.websiteURL() + "/guides.html#" + guidesHash +
		"\n\n▓ КЛЮЧ ПОДПИСКИ ▓\n" +
		"Тапни по ключу ниже — он скопируется.\n" +
		"Затем в Happ: + → Import from URL (не открывай ссылку в браузере)."
	m := tgbotapi.NewMessage(chatID, text)
	m.ReplyMarkup = b.happSubscriptionKeyboard(subURL, platform)
	b.send(m)
	// The key itself goes in its own message as a monospace code entity so a
	// single tap copies it — no manual long-press selection.
	b.sendCode(chatID, subURL)
}

func (b *Bot) sendHappSubscriptionCopy(chatID int64, from *tgbotapi.User) {
	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	user, err := b.ensureUser(ctx, from)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			b.sendText(chatID, msgNoAccount)
			return
		}
		b.fail(chatID, "happ_copy_user", from.ID, "", err)
		return
	}

	subURL, err := b.happSub.EnsureSubscriptionURL(ctx, user.ID)
	if errors.Is(err, service.ErrDeviceLimitReached) {
		b.sendText(chatID, msgDeviceLimit)
		return
	}
	if err != nil {
		b.fail(chatID, "happ_copy_url", from.ID, user.ID, err)
		return
	}

	b.sendText(chatID, "▓ КЛЮЧ ПОДПИСКИ ▓\nТапни по ключу ниже, чтобы скопировать:")
	b.sendCode(chatID, subURL)
}

func (b *Bot) happSubscriptionKeyboard(subURL, platform string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	switch platform {
	case "ios":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ HAPP — App Store", happIOSAppStoreGlobal),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ HAPP — App Store RU", happIOSAppStoreRU),
		))
	case "mac":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ HAPP для Mac (DMG)", happMacDMG),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("⬇️ HAPP — App Store", happIOSAppStoreGlobal),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📋 СКОПИРОВАТЬ КЛЮЧ", "tyrax_copy_sub"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonURL("💳 КУПИТЬ / ПРОДЛИТЬ", b.cfg.TelegramBotURL),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// handlePlatformWG sends legacy WireGuard onboarding copy (unused in VLESS flow).
func (b *Bot) handlePlatformWG(chatID int64, text, platform string) {
	slog.Info("telegram bot", slog.String("action", "platform_"+platform), slog.Int64("telegram_id", chatID))
	m := tgbotapi.NewMessage(chatID, text)
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 Получить конфиг", "tyrax_config:"+platform),
		),
	)
	b.send(m)
}

func (b *Bot) handleDevices(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	user, err := b.userRepo.FindByTelegramID(ctx, msg.From.ID)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.sendText(msg.Chat.ID, msgNoAccount)
		return
	}
	if err != nil {
		b.fail(msg.Chat.ID, "devices_find_user", msg.From.ID, "", err)
		return
	}

	devices, err := b.deviceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		b.fail(msg.Chat.ID, "devices_list", msg.From.ID, user.ID, err)
		return
	}

	slog.Info("telegram bot", slog.String("action", "devices"), slog.Int64("telegram_id", msg.From.ID), slog.String("user_id", user.ID))

	if len(devices) == 0 {
		b.sendText(msg.Chat.ID, "▓ МОИ УСТРОЙСТВА ▓\n\n"+
			"Устройств нет.\n"+
			"Подключи первое через Android, Windows, iPhone или Mac.")
		return
	}

	limit := service.DeviceLimit(user.SubscriptionTier)
	var sb strings.Builder
	sb.WriteString("▓ МОИ УСТРОЙСТВА ▓\n\n")
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(devices))
	for i, d := range devices {
		sb.WriteString(fmt.Sprintf("[%d] %s — создано %s\n", i+1, d.Name, d.CreatedAt.Format("02.01.2006")))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✕ "+d.Name, "tyrax_del:"+d.ID),
		))
	}
	sb.WriteString(fmt.Sprintf("\nЛимит: %d/%d слотов занято.\n", len(devices), limit))
	sb.WriteString("Нажми × чтобы удалить устройство.")

	m := tgbotapi.NewMessage(msg.Chat.ID, sb.String())
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(m)
}

func (b *Bot) handleBuyStart(chatID int64) {
	slog.Info("telegram bot", slog.String("action", "buy_start"), slog.Int64("telegram_id", chatID))
	m := tgbotapi.NewMessage(chatID, "▓ ПОДПИСКА ▓\n\nВыбери тариф:")
	m.ReplyMarkup = tierKeyboard()
	b.send(m)
}

// ── Callback routing ─────────────────────────────────────────────────────────

func (b *Bot) handleCallback(cq *tgbotapi.CallbackQuery) {
	data := cq.Data
	switch {
	case data == "tyrax_back_tier":
		b.editTierSelection(cq)
	case data == "tyrax_copy_sub":
		b.handleCopySubCallback(cq)
	case strings.HasPrefix(data, "tyrax_dev:"):
		b.handleDeviceCallback(cq, strings.TrimPrefix(data, "tyrax_dev:"))
	case strings.HasPrefix(data, "tyrax_del:"):
		b.handleDeleteCallback(cq, strings.TrimPrefix(data, "tyrax_del:"))
	case strings.HasPrefix(data, "tyrax_tier:"):
		b.handleTierCallback(cq, strings.TrimPrefix(data, "tyrax_tier:"))
	case strings.HasPrefix(data, "tyrax_period:"):
		b.handlePeriodCallback(cq, strings.TrimPrefix(data, "tyrax_period:"))
	case strings.HasPrefix(data, "tyrax_pay:"):
		b.handlePayCallback(cq, strings.TrimPrefix(data, "tyrax_pay:"))
	default:
		b.answerCallback(cq.ID, "")
	}
}

func (b *Bot) handleDeviceCallback(cq *tgbotapi.CallbackQuery, platform string) {
	b.answerCallback(cq.ID, "")
	chatID := cq.Message.Chat.ID
	switch platform {
	case "android":
		b.handleAndroid(chatID)
	case "iphone":
		b.sendHappSubscription(chatID, cq.From, msgIOS, "ios")
	case "windows":
		b.handleWindows(chatID)
	case "mac":
		b.sendHappSubscription(chatID, cq.From, msgMac, "mac")
	default:
		b.sendText(chatID, msgUseMenu)
	}
}

func (b *Bot) handleCopySubCallback(cq *tgbotapi.CallbackQuery) {
	b.answerCallback(cq.ID, "Ключ отправлен отдельным сообщением")
	b.sendHappSubscriptionCopy(cq.Message.Chat.ID, cq.From)
}

func (b *Bot) handleDeleteCallback(cq *tgbotapi.CallbackQuery, deviceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	user, err := b.userRepo.FindByTelegramID(ctx, cq.From.ID)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.answerCallback(cq.ID, msgNoAccountShort)
		return
	}
	if err != nil {
		slog.Error("telegram bot", slog.String("action", "delete_find_user"), slog.Int64("telegram_id", cq.From.ID), slog.String("error", err.Error()))
		b.answerCallback(cq.ID, msgGenericErr)
		return
	}

	// Delete filters by (id, user_id): a device not owned by this user never matches.
	if err := b.deviceRepo.Delete(ctx, deviceID, user.ID); err != nil {
		if errors.Is(err, repository.ErrDeviceNotFound) {
			b.answerCallback(cq.ID, msgNoAccountShort)
			return
		}
		slog.Error("telegram bot", slog.String("action", "delete_device"), slog.String("user_id", user.ID), slog.String("error", err.Error()))
		b.answerCallback(cq.ID, msgGenericErr)
		return
	}

	slog.Info("telegram bot", slog.String("action", "device_deleted"), slog.Int64("telegram_id", cq.From.ID), slog.String("user_id", user.ID))
	b.send(tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, msgDeviceDeleted))
	b.answerCallback(cq.ID, "")
}

func (b *Bot) handleTierCallback(cq *tgbotapi.CallbackQuery, tier string) {
	b.answerCallback(cq.ID, "")
	text := fmt.Sprintf("▓ %s ▓\n\nВыбери период:", tier)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	kb := periodKeyboard(tier)
	edit.ReplyMarkup = &kb
	b.send(edit)
}

func (b *Bot) editTierSelection(cq *tgbotapi.CallbackQuery) {
	b.answerCallback(cq.ID, "")
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, "▓ ПОДПИСКА ▓\n\nВыбери тариф:")
	kb := tierKeyboard()
	edit.ReplyMarkup = &kb
	b.send(edit)
}

func (b *Bot) handlePeriodCallback(cq *tgbotapi.CallbackQuery, rest string) {
	tier, months, ok := parseTierMonths(rest)
	if !ok {
		b.answerCallback(cq.ID, "")
		return
	}
	b.answerCallback(cq.ID, "")

	total := int(service.CalculatePrice(tier, months))
	text := fmt.Sprintf("▓ ОПЛАТА ▓\n\n%s · %d мес · %d ₽\n\nВыбери способ оплаты:", tier, months, total)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	kb := paymentKeyboard(tier, months)
	edit.ReplyMarkup = &kb
	b.send(edit)
}

func (b *Bot) handlePayCallback(cq *tgbotapi.CallbackQuery, rest string) {
	parts := strings.Split(rest, ":")
	if len(parts) != 3 {
		b.answerCallback(cq.ID, "")
		return
	}
	tier := parts[0]
	months, err := strconv.Atoi(parts[1])
	if err != nil {
		b.answerCallback(cq.ID, "")
		return
	}
	method, ok := methodMap[parts[2]]
	if !ok {
		b.answerCallback(cq.ID, "")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	b.answerCallback(cq.ID, "")

	user, err := b.ensureUser(ctx, cq.From)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.editText(cq, msgNoAccount)
		return
	}
	if err != nil {
		slog.Error("telegram bot", slog.String("action", "pay_find_user"), slog.Int64("telegram_id", cq.From.ID), slog.String("error", err.Error()))
		b.editText(cq, msgPaymentErr)
		return
	}

	result, err := b.paymentSvc.CreateOrder(ctx, user.ID, tier, method, months, b.cfg.SupportEmail, "127.0.0.1")
	if err != nil {
		slog.Error("telegram bot", slog.String("action", "create_order"), slog.String("user_id", user.ID), slog.String("tier", tier), slog.Int("months", months), slog.String("method", method), slog.String("error", err.Error()))
		b.editText(cq, msgPaymentErr)
		return
	}

	slog.Info("telegram bot", slog.String("action", "order_created"), slog.Int64("telegram_id", cq.From.ID), slog.String("user_id", user.ID), slog.String("order_id", result.OrderID), slog.Float64("amount_rub", result.AmountRUB))

	text := fmt.Sprintf("▓ СЧЁТ СОЗДАН ▓\n\n%s · %d мес · %d ₽\n\n"+
		"Нажми кнопку для оплаты.\n"+
		"После оплаты подписка активируется автоматически.", tier, months, int(result.AmountRUB))
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗 ПЕРЕЙТИ К ОПЛАТЕ", result.PaymentURL),
		),
	)
	edit.ReplyMarkup = &kb
	b.send(edit)
}

// ── Keyboards ────────────────────────────────────────────────────────────────

func mainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnAccount)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnConnect)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnDevices), tgbotapi.NewKeyboardButton(btnBuy)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnHelp)),
	)
	kb.ResizeKeyboard = true
	return kb
}

func devicePickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Android", "tyrax_dev:android"),
			tgbotapi.NewInlineKeyboardButtonData("🍎 iPhone", "tyrax_dev:iphone"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💻 Windows", "tyrax_dev:windows"),
			tgbotapi.NewInlineKeyboardButtonData("🖥 macOS", "tyrax_dev:mac"),
		),
	)
}

func tierKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("CORE — 199₽/мес", "tyrax_tier:CORE"),
			tgbotapi.NewInlineKeyboardButtonData("SHADOW — 349₽/мес", "tyrax_tier:SHADOW"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("DOMINION — 649₽/мес", "tyrax_tier:DOMINION"),
		),
	)
}

func periodKeyboard(tier string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 мес", "tyrax_period:"+tier+":1"),
			tgbotapi.NewInlineKeyboardButtonData("3 мес  −10%", "tyrax_period:"+tier+":3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("6 мес  −15%", "tyrax_period:"+tier+":6"),
			tgbotapi.NewInlineKeyboardButtonData("12 мес  −20%", "tyrax_period:"+tier+":12"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "tyrax_back_tier"),
		),
	)
}

func paymentKeyboard(tier string, months int) tgbotapi.InlineKeyboardMarkup {
	prefix := fmt.Sprintf("tyrax_pay:%s:%d:", tier, months)
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Карта РФ", prefix+"card"),
			tgbotapi.NewInlineKeyboardButtonData("📱 СБП", prefix+"sbp"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("₿ Криптовалюта", prefix+"crypto"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "tyrax_tier:"+tier),
		),
	)
}

var methodMap = map[string]string{
	"card":   string(model.PaymentCardRF),
	"sbp":    string(model.PaymentSBP),
	"crypto": string(model.PaymentCrypto),
}

// ── Persistence helpers ──────────────────────────────────────────────────────

// ensureUser returns the Telegram identity, provisioning a FREE account on first bot contact.
func (b *Bot) ensureUser(ctx context.Context, from *tgbotapi.User) (*model.User, error) {
	user, _, err := b.resolveUser(ctx, from.ID, from.UserName)
	return user, err
}

// resolveUser returns the identity for a Telegram account, provisioning a
// FREE-tier one on first contact.
func (b *Bot) resolveUser(ctx context.Context, telegramID int64, username string) (*model.User, bool, error) {
	user, err := b.userRepo.FindByTelegramID(ctx, telegramID)
	if err == nil {
		return user, false, nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, false, err
	}
	user, err = b.userRepo.CreateFromTelegram(ctx, telegramID, username)
	if err != nil {
		return nil, false, err
	}
	return user, true, nil
}

func tokenPending(ctx context.Context, db *pgxpool.Pool, token string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM telegram_auth_tokens
		    WHERE token = $1 AND confirmed = false AND expires_at > NOW()
		 )`, token).Scan(&exists)
	return exists, err
}

func confirmToken(ctx context.Context, db *pgxpool.Pool, token, userID string) (bool, error) {
	tag, err := db.Exec(ctx,
		`UPDATE telegram_auth_tokens
		    SET confirmed = true, user_id = $2
		  WHERE token = $1 AND confirmed = false AND expires_at > NOW()`,
		token, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func parseTierMonths(rest string) (tier string, months int, ok bool) {
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, false
	}
	return parts[0], m, true
}

// ── Telegram I/O helpers ─────────────────────────────────────────────────────

func (b *Bot) send(c tgbotapi.Chattable) {
	if _, err := b.api.Send(c); err != nil {
		slog.Error("telegram bot: send failed", slog.String("error", err.Error()))
	}
}

func (b *Bot) sendSupportLink(chatID int64) {
	m := tgbotapi.NewMessage(chatID, "▓ ПОМОЩЬ ▓\n\n"+
		"Не подключается? Тормозит? Вопрос по оплате?\n\n"+
		"Нажми кнопку — откроется чат поддержки.\n"+
		"DOMINION — приоритетная поддержка 24/7.")
	btn := tgbotapi.NewInlineKeyboardButtonURL("🆘 ОТКРЫТЬ ПОДДЕРЖКУ", b.cfg.TelegramSupportBotURL)
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)
	b.send(m)
}

func (b *Bot) sendText(chatID int64, text string) {
	b.send(tgbotapi.NewMessage(chatID, text))
}

// sendCode sends text rendered as a monospace `code` entity so tapping it copies
// the whole string in one gesture. We attach the entity manually (offset/length
// in UTF-16 code units, per the Bot API) instead of MarkdownV2 to avoid escaping
// the '_', '/', '.' and other characters that appear in subscription URLs.
func (b *Bot) sendCode(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.Entities = []tgbotapi.MessageEntity{{
		Type:   "code",
		Offset: 0,
		Length: len(utf16.Encode([]rune(text))),
	}}
	b.send(m)
}

func (b *Bot) sendMainMenu(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ReplyMarkup = mainMenuKeyboard()
	b.send(m)
}

func (b *Bot) editText(cq *tgbotapi.CallbackQuery, text string) {
	b.send(tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text))
}

func (b *Bot) answerCallback(id, text string) {
	if _, err := b.api.Request(tgbotapi.NewCallback(id, text)); err != nil {
		slog.Error("telegram bot: answer callback failed", slog.String("error", err.Error()))
	}
}

func (b *Bot) fail(chatID int64, action string, telegramID int64, userID string, err error) {
	slog.Error("telegram bot",
		slog.String("action", action),
		slog.Int64("telegram_id", telegramID),
		slog.String("user_id", userID),
		slog.String("error", err.Error()),
	)
	b.sendText(chatID, msgGenericErr)
}

package supportbot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/tyrax/tyrax-backend/internal/config"
	"github.com/tyrax/tyrax-backend/internal/handler"
	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
)

const (
	updateTimeoutSec = 60
	dbOpTimeout      = 5 * time.Second
)

type Bot struct {
	api         *tgbotapi.BotAPI
	cfg         *config.Config
	userRepo    repository.UserRepository
	supportRepo repository.SupportRepository
}

func newBotAPI(cfg *config.Config, token string) (*tgbotapi.BotAPI, error) {
	if cfg.TelegramProxy == "" {
		return tgbotapi.NewBotAPI(token)
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
	return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
}

// Start launches the support bot worker. Returns the messenger used by admin replies.
func Start(cfg *config.Config, userRepo repository.UserRepository, supportRepo repository.SupportRepository) handler.SupportMessenger {
	if cfg.TelegramSupportToken == "" {
		slog.Warn("support bot: TELEGRAM_SUPPORT_BOT_TOKEN unset, worker disabled")
		return nil
	}

	api, err := newBotAPI(cfg, cfg.TelegramSupportToken)
	if err != nil {
		slog.Error("support bot: init failed", slog.String("error", err.Error()))
		return nil
	}
	slog.Info("support bot: online", slog.String("username", api.Self.UserName))

	b := &Bot{
		api:         api,
		cfg:         cfg,
		userRepo:    userRepo,
		supportRepo: supportRepo,
	}

	go b.run()
	return b
}

func (b *Bot) SendUserMessage(telegramID int64, text string) error {
	msg := tgbotapi.NewMessage(telegramID, text)
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = updateTimeoutSec
	for update := range b.api.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}
		b.handleMessage(update.Message)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
	defer cancel()

	tier, userID := b.resolveIdentity(ctx, msg.From.ID, msg.From.UserName)

	if msg.IsCommand() {
		switch msg.Command() {
		case "start", "help":
			b.sendWelcome(msg.Chat.ID, tier)
		default:
			b.sendWelcome(msg.Chat.ID, tier)
		}
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	ticket, err := b.supportRepo.FindOpenByTelegramID(ctx, msg.From.ID)
	if err != nil {
		slog.Error("support bot: find ticket", slog.String("error", err.Error()))
		b.sendText(msg.Chat.ID, "⚠️ Система недоступна. Попробуй позже.")
		return
	}

	if ticket == nil {
		subject := truncate(text, 120)
		ticket, err = b.supportRepo.CreateTicket(ctx, &model.SupportTicket{
			UserID:           userID,
			TelegramID:       msg.From.ID,
			TelegramUsername: strPtr(msg.From.UserName),
			SubscriptionTier: tier,
			Subject:          subject,
		})
		if err != nil {
			slog.Error("support bot: create ticket", slog.String("error", err.Error()))
			b.sendText(msg.Chat.ID, "⚠️ Не удалось создать тикет. Попробуй позже.")
			return
		}
	}

	if _, err := b.supportRepo.AddMessage(ctx, ticket.ID, "user", text); err != nil {
		slog.Error("support bot: add message", slog.String("error", err.Error()))
		b.sendText(msg.Chat.ID, "⚠️ Сообщение не сохранено. Попробуй ещё раз.")
		return
	}

	priority := ""
	if tier == model.TierDominion {
		priority = "\n\n▓ DOMINION — ПРИОРИТЕТ ▓"
	}
	b.sendText(msg.Chat.ID, "▓ ПРИНЯТО ▓\n\nСообщение передано оператору."+priority)
}

func (b *Bot) resolveIdentity(ctx context.Context, telegramID int64, username string) (model.SubscriptionTier, *string) {
	user, err := b.userRepo.FindByTelegramID(ctx, telegramID)
	if err != nil {
		return model.TierFree, nil
	}
	return service.EffectiveTier(user), &user.ID
}

func (b *Bot) sendWelcome(chatID int64, tier model.SubscriptionTier) {
	tierLine := fmt.Sprintf("Тариф: %s", tier)
	if tier == model.TierDominion {
		tierLine = "Тариф: DOMINION — приоритетная поддержка"
	}
	text := "▓ TYRAX SUPPORT ▓\n\n" +
		tierLine + "\n\n" +
		"Опиши проблему одним сообщением — мы ответим здесь.\n" +
		"Не подключается, оплата, устройства — всё сюда."
	b.sendText(chatID, text)
}

func (b *Bot) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		slog.Error("support bot: send", slog.String("error", err.Error()))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

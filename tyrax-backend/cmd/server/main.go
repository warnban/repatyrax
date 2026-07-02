package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/config"
	"github.com/tyrax/tyrax-backend/internal/handler"
	"github.com/tyrax/tyrax-backend/internal/middleware"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
	"github.com/tyrax/tyrax-backend/internal/telegrambot"
	"github.com/tyrax/tyrax-backend/pkg/cryptopay"
	"github.com/tyrax/tyrax-backend/pkg/freekassa"
	"github.com/tyrax/tyrax-backend/pkg/threexui"
)

func main() {
	// Structured JSON logging — single source of truth for the whole process.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("DATABASE CONNECTION FAILED", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// ── Repositories ─────────────────────────────────────────────────────────
	nodeRepo   := repository.NewNodeRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)
	userRepo   := repository.NewUserRepository(db)
	orderRepo  := repository.NewOrderRepository(db)
	inviteRepo := repository.NewInviteRepository(db)

	// ── External clients ──────────────────────────────────────────────────────
	fkClient := freekassa.New(cfg.FreeKassaShopID, cfg.FreeKassaAPIKey, cfg.FreeKassaSecretWord2)
	cpClient := cryptopay.New(cfg.CryptoPayToken)

	// ── Services ─────────────────────────────────────────────────────────────
	// Panel syncer registers per-device VLESS UUIDs on each node's 3x-ui inbound
	// and also reads per-device traffic counters for FREE-tier metering.
	panelSyncer := threexui.NewSyncer()
	trafficSvc := service.NewTrafficService(userRepo, deviceRepo, nodeRepo, panelSyncer)
	// Balancer samples live per-node online counts so GetNodes can steer clients
	// to the least-loaded node. Fail-open: no data ⇒ default ping ordering.
	nodeBalancer := service.NewNodeBalancer(nodeRepo, panelSyncer)
	vpnSvc     := service.NewVPNService(nodeRepo, deviceRepo, userRepo, panelSyncer, trafficSvc, nodeBalancer)
	paymentSvc := service.NewPaymentService(orderRepo, userRepo, fkClient, cpClient)
	inviteSvc  := service.NewInviteService(userRepo, inviteRepo)
	happSubSvc := service.NewHappSubscriptionService(
		userRepo, deviceRepo, nodeRepo, vpnSvc, panelSyncer, trafficSvc,
		cfg.PublicAPIURL, cfg.WebsiteURL, cfg.TelegramBotURL,
	)

	// Traffic accounting sweep — reads node panels, credits usage, blocks FREE
	// identities over quota. Fail-open: never affects the tunnel on error.
	go trafficSvc.RunLoop(ctx)

	// Live load sampler for node balancing. Fail-open: never affects the tunnel.
	go nodeBalancer.RunLoop(ctx)

	// ── Telegram bot worker ────────────────────────────────────────────────────
	// Full bot: auth deep links, account, config delivery, devices, payments.
	// No-op if TELEGRAM_BOT_TOKEN is unset.
	go telegrambot.Start(cfg, db, vpnSvc, paymentSvc, happSubSvc)

	// ── Handlers ─────────────────────────────────────────────────────────────
	authH    := handler.NewAuthHandler(userRepo, cfg.JWTSecret, cfg.TelegramBotUsername)
	vpnH     := handler.NewVPNHandler(vpnSvc)
	paymentH := handler.NewPaymentHandler(paymentSvc, inviteSvc, deviceRepo, userRepo, trafficSvc)
	subH     := handler.NewSubscriptionHandler(happSubSvc)
	dlH      := handler.NewDownloadHandler(cfg.WebsiteURL, cfg.WindowsAppVersion)

	// ── App ───────────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})
	app.Use(recover.New())
	app.Use(middleware.RequestLogger(logger))

	// Liveness probe — used by docker-compose / orchestrator health checks.
	app.Get("/health", func(c *fiber.Ctx) error {
		if err := db.Ping(c.Context()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "error", "message": "NODE OFFLINE",
			})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// ── Public webhook routes (no JWT, no rate limit — external services) ──────
	app.Post("/webhooks/freekassa",  paymentH.FreekassaWebhook)
	app.Post("/webhooks/crypto-pay", paymentH.CryptoPayWebhook)

	// Happ subscription feed (iOS / macOS). Token auth — no JWT.
	app.Get("/sub/:token", subH.HappFeed)

	// Desktop release manifest (Windows in-app update checker).
	app.Get("/download/windows/latest.json", dlH.WindowsLatest)

	// ── API v1 ────────────────────────────────────────────────────────────────
	api := app.Group("/api/v1")

	// Public auth routes — 10 req/min per IP to block brute force.
	auth := api.Group("/auth", middleware.AuthRateLimiter())
	auth.Post("/register",          authH.Register)
	auth.Post("/login",             authH.Login)
	auth.Get("/telegram-init",      authH.TelegramInit)
	auth.Post("/telegram-callback", authH.TelegramCallback)
	auth.Get("/telegram-status",    authH.TelegramStatus)

	// Protected routes — JWT first (sets user_id), then 100 req/min per user.
	protected := api.Group("/", middleware.JWTAuth(cfg.JWTSecret), middleware.UserRateLimiter())

	// Profile
	protected.Get("/auth/profile", authH.GetProfile)

	// VPN
	protected.Post("/vpn/device",            vpnH.AddDevice)
	protected.Delete("/vpn/device/:deviceID", vpnH.DeleteDevice)
	protected.Get("/vpn/config",             vpnH.GetConfig)
	protected.Get("/vpn/devices",            vpnH.GetDevices)
	protected.Get("/vpn/split-domains",      vpnH.GetSplitDomains)
	protected.Get("/nodes",                  vpnH.GetNodes)
	protected.Post("/vpn/connect",           vpnH.Connect)
	protected.Post("/vpn/disconnect",        handler.LogDisconnect)

	// Payments
	protected.Post("/payment/create",            paymentH.CreatePayment)
	protected.Get("/payment/status/:orderID",    paymentH.GetPaymentStatus)
	protected.Get("/subscription",               paymentH.GetSubscription)

	// Subscription invites
	protected.Get("/subscription/invites",        paymentH.GetInvites)
	protected.Post("/subscription/invite",        paymentH.SendInvite)
	protected.Delete("/subscription/invite/:accountID", paymentH.RemoveInvite)
	protected.Post("/subscription/invite/accept", paymentH.AcceptInvite)
	protected.Post("/subscription/invite/leave",  paymentH.LeaveInvite)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("TYRAX BACKEND ONLINE", slog.String("port", port))
	if err := app.Listen(":" + port); err != nil {
		logger.Error("SERVER STOPPED", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"status":  "error",
		"message": err.Error(),
	})
}

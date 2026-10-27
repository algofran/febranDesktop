package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/algofran/febranDesktop/internal/config"
	"github.com/algofran/febranDesktop/internal/gateway"
	"github.com/algofran/febranDesktop/internal/handler"
	"github.com/algofran/febranDesktop/internal/middleware"
	"github.com/algofran/febranDesktop/internal/router"
	"github.com/algofran/febranDesktop/internal/webhook"
	"github.com/algofran/febranDesktop/pkg/whatsapp"
	"github.com/gofiber/fiber/v2"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := os.MkdirAll(cfg.SessionDir, 0755); err != nil {
		log.Fatalf("Failed to create session directory: %v", err)
	}

	nodeID, err := config.LoadOrCreateNodeID(cfg.SessionDir)
	if err != nil {
		log.Fatalf("Failed to load/create node ID: %v", err)
	}

	// Initialize webhook client for central backend
	webhookClient := webhook.NewClient(cfg.CentralWebhookURL, cfg.CentralAPIKey)

	// Initialize WhatsApp session manager
	mgr := whatsapp.NewManager(cfg.SessionDir)

	// WebSocket client: registers this node with the central backend and
	// executes commands routed to it (pair QR/code, send, groups).
	wsClient := gateway.NewClient(cfg.CentralWSURL, cfg.GatewayServerKey, cfg.CentralAPIKey, nodeID, mgr)
	if cfg.CentralWSURL != "" && cfg.GatewayServerKey != "" {
		go wsClient.Run(context.Background())
	} else {
		log.Println("⚠️  CENTRAL_WS_URL / GATEWAY_SERVER_KEY not set — node will not register with backend")
	}

	mgr.OnMessage = func(msg whatsapp.InboundMessage) {
		webhookClient.SendEvent("message.inbound", msg)
	}
	mgr.OnStatusChange = func(status whatsapp.StatusChange) {
		webhookClient.SendEvent("status.changed", status)
		connected := status.Status == "connected"
		wsClient.PushDeviceUpdate(status.NumberID, connected, mgr.DeviceJID(status.NumberID), mgr.PairingKey(status.NumberID))
	}

	// Recover existing sessions on boot
	mgr.RecoverConnected()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "WA Gateway v1.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
	app.Use(recover.New())
	app.Use(fiberLogger.New(fiberLogger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	// Initialize handler
	gatewayHandler := handler.NewGatewayHandler(mgr, wsClient, webhookClient, wsClient.IsConnected)
	authMiddleware := middleware.Auth(cfg)

	// Register routes
	router.Setup(app, gatewayHandler, authMiddleware)

	// Start server in goroutine
	go func() {
		log.Printf("🚀 WA Gateway starting on %s", cfg.GatewayBind)
		if err := app.Listen(cfg.GatewayBind); err != nil {
			log.Fatalf("Gateway server error: %v", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("⏳ Shutting down gateway...")
	if err := app.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	mgr.StopAll()
	log.Println("✅ Shutdown complete.")
}

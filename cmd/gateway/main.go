package main

import (
	"context"
	"fmt"
	"io"
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

// logOut is where request logs go: stdout on the console, a rotating file
// when running as a Windows service (set by service_windows.go).
var logOut io.Writer = os.Stdout

func main() {
	// `gateway service <install|uninstall|start|stop|restart|status>` manages the
	// Windows service and exits; see service_windows.go.
	if len(os.Args) > 1 && os.Args[1] == "service" {
		os.Exit(runServiceCommand(os.Args[2:]))
	}
	if runningAsService() {
		os.Exit(runAsService())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx); err != nil {
		log.Fatalf("%v", err)
	}
}

// serve runs the gateway until ctx is cancelled, then shuts down gracefully.
// It returns an error for startup failures (bad config, port in use).
func serve(ctx context.Context) error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := os.MkdirAll(cfg.SessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	nodeID, err := config.LoadOrCreateNodeID(cfg.SessionDir)
	if err != nil {
		return fmt.Errorf("failed to load/create node ID: %w", err)
	}

	// Initialize webhook client for central backend
	webhookClient := webhook.NewClient(cfg.CentralWebhookURL, cfg.CentralAPIKey)

	// Initialize WhatsApp session manager
	mgr := whatsapp.NewManager(cfg.SessionDir)

	// WebSocket client: registers this node with the central backend and
	// executes commands routed to it (pair QR/code, send, groups).
	wsClient := gateway.NewClient(cfg.CentralWSURL, cfg.GatewayServerKey, cfg.CentralAPIKey, nodeID, mgr)
	if cfg.CentralWSURL != "" && cfg.GatewayServerKey != "" {
		go wsClient.Run(ctx)
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
		Output:     logOut,
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	// Initialize handler
	gatewayHandler := handler.NewGatewayHandler(mgr, wsClient, webhookClient, wsClient.IsConnected)
	authMiddleware := middleware.Auth(cfg)

	// Register routes
	router.Setup(app, gatewayHandler, authMiddleware)

	// Start server in goroutine; a listen failure ends serve instead of the process.
	listenErr := make(chan error, 1)
	go func() {
		log.Printf("🚀 WA Gateway starting on %s", cfg.GatewayBind)
		if err := app.Listen(cfg.GatewayBind); err != nil {
			listenErr <- fmt.Errorf("gateway server error: %w", err)
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-listenErr:
	}

	log.Println("⏳ Shutting down gateway...")
	if err := app.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	mgr.StopAll()
	log.Println("✅ Shutdown complete.")
	return serveErr
}

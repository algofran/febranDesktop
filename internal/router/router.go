package router

import (
	"github.com/algofran/febranDesktop/internal/handler"
	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App, h *handler.GatewayHandler, authMiddleware fiber.Handler) {
	app.Get("/health", h.Health)

	api := app.Group("/api/v1", authMiddleware)
	api.Get("/sessions", h.ListSessions)
	api.Post("/pair", h.Pair)
	api.Post("/pair-qr", h.PairQR)
	api.Get("/qr/:id", h.GetQR)
	api.Get("/status/:id", h.GetStatus)
	api.Post("/send", h.SendText)
	api.Post("/send-group", h.SendGroup)
	api.Get("/groups", h.GetAllGroups)
	api.Get("/groups/:id", h.GetGroups)
	api.Get("/contacts", h.GetAllContacts)
	api.Get("/contacts/:id", h.GetContacts)
	api.Get("/sync/all", h.SyncAllData)
	api.Post("/sync/all", h.SyncAllData)
	api.Post("/sync/:id", h.SyncSessionData)
	api.Post("/logout/:id", h.Logout)
}

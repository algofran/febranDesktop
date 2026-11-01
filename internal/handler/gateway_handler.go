package handler

import (
	"github.com/algofran/febranDesktop/internal/gateway"
	"github.com/algofran/febranDesktop/internal/webhook"
	"github.com/algofran/febranDesktop/pkg/whatsapp"
	"github.com/gofiber/fiber/v2"
)

type GatewayHandler struct {
	manager       *whatsapp.Manager
	wsClient      *gateway.Client
	webhookClient *webhook.Client
	wsStatus      func() bool
}

func NewGatewayHandler(manager *whatsapp.Manager, wsClient *gateway.Client, webhookClient *webhook.Client, wsStatus func() bool) *GatewayHandler {
	return &GatewayHandler{
		manager:       manager,
		wsClient:      wsClient,
		webhookClient: webhookClient,
		wsStatus:      wsStatus,
	}
}

type PairRequest struct {
	NumberID string `json:"number_id"`
}

func (h *GatewayHandler) Pair(c *fiber.Ctx) error {
	var req PairRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.NumberID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "number_id is required"})
	}

	err := h.manager.Pair(req.NumberID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "pairing", "number_id": req.NumberID})
}

func (h *GatewayHandler) PairQR(c *fiber.Ctx) error {
	sessionID, err := h.manager.StartQRPairing()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "pairing", "session_id": sessionID})
}

func (h *GatewayHandler) GetQR(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	qr, err := h.manager.GetQR(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if qr == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no QR available"})
	}
	return c.JSON(fiber.Map{"qr": qr, "number_id": id})
}

func (h *GatewayHandler) ListSessions(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"sessions": h.manager.ListSessions()})
}

func (h *GatewayHandler) GetStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	connected := h.manager.IsConnected(id)
	return c.JSON(fiber.Map{"number_id": id, "connected": connected})
}

type SendRequest struct {
	NumberID string `json:"number_id"`
	To       string `json:"to"`
	Text     string `json:"text"`
}

func (h *GatewayHandler) SendText(c *fiber.Ctx) error {
	var req SendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.NumberID == "" || req.To == "" || req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "number_id, to, and text are required"})
	}

	err := h.manager.SendMessage(req.NumberID, req.To, req.Text)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "sent"})
}

type SendGroupRequest struct {
	NumberID string `json:"number_id"`
	GroupJID string `json:"group_jid"`
	Text     string `json:"text"`
}

func (h *GatewayHandler) SendGroup(c *fiber.Ctx) error {
	var req SendGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.NumberID == "" || req.GroupJID == "" || req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "number_id, group_jid, and text are required"})
	}

	err := h.manager.SendGroupMessage(req.NumberID, req.GroupJID, req.Text)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "sent"})
}

func (h *GatewayHandler) GetGroups(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	groups, err := h.manager.GetGroups(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"groups": groups})
}

func (h *GatewayHandler) GetContacts(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	contacts, err := h.manager.GetContacts(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"contacts": contacts})
}

func (h *GatewayHandler) GetAllContacts(c *fiber.Ctx) error {
	contacts, err := h.manager.GetAllContacts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"contacts": contacts, "total": len(contacts)})
}

func (h *GatewayHandler) GetAllGroups(c *fiber.Ctx) error {
	groups, err := h.manager.GetAllGroups()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"groups": groups, "total": len(groups)})
}

func (h *GatewayHandler) SyncAllData(c *fiber.Ctx) error {
	var result *whatsapp.AllSyncResult
	var err error

	if h.wsClient != nil {
		result, err = h.wsClient.SyncAllToBackend()
	} else {
		result, err = h.manager.FetchAllConnectedData()
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if h.webhookClient != nil && result != nil {
		_ = h.webhookClient.SendEvent("sync.completed", result)
	}

	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "Semua data kontak dan grup dari nomor yang terhubung berhasil ditarik dan disinkronkan",
		"data":    result,
	})
}

func (h *GatewayHandler) SyncSessionData(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	var result *whatsapp.SessionSyncData
	var err error

	if h.wsClient != nil {
		result, err = h.wsClient.SyncSessionToBackend(id)
	} else {
		result, err = h.manager.FetchSessionData(id)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if h.webhookClient != nil && result != nil {
		_ = h.webhookClient.SendEvent("sync.session", result)
	}

	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "Data kontak dan grup untuk sesi berhasil ditarik dan disinkronkan",
		"data":    result,
	})
}


type LogoutRequest struct {
	KeepSession bool `json:"keep_session"`
}

func (h *GatewayHandler) Logout(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		// Ignore parse errors as body is optional
	}

	err := h.manager.Logout(id, req.KeepSession)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "logged_out"})
}

func (h *GatewayHandler) Health(c *fiber.Ctx) error {
	wsConnected := false
	if h.wsStatus != nil {
		wsConnected = h.wsStatus()
	}
	return c.JSON(fiber.Map{"status": "ok", "service": "wa-gateway", "ws_connected": wsConnected})
}

package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	GatewayBind       string // GATEWAY_BIND, default "0.0.0.0:8989"
	GatewayAPIKey     string // GATEWAY_API_KEY (required)
	CentralWebhookURL string // CENTRAL_WEBHOOK_URL (required)
	CentralWSURL      string // CENTRAL_WS_URL, e.g. ws://localhost:8080/ws/gateway
	GatewayServerKey  string // GATEWAY_SERVER_KEY, identifies this node to the backend
	CentralAPIKey     string // CENTRAL_API_KEY, shared WS auth key
	SessionDir        string // SESSION_DIR, default "./sessions"
	LogLevel          string // LOG_LEVEL, default "info"
}

func Load() (*Config, error) {
	cfg := &Config{
		GatewayBind:       getEnvOrDefault("GATEWAY_BIND", "0.0.0.0:8989"),
		GatewayAPIKey:     os.Getenv("GATEWAY_API_KEY"),
		CentralWebhookURL: os.Getenv("CENTRAL_WEBHOOK_URL"),
		CentralWSURL:      os.Getenv("CENTRAL_WS_URL"),
		GatewayServerKey:  os.Getenv("GATEWAY_SERVER_KEY"),
		CentralAPIKey:     os.Getenv("CENTRAL_API_KEY"),
		SessionDir:        getEnvOrDefault("SESSION_DIR", "./sessions"),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
	}

	if cfg.GatewayAPIKey == "" {
		return nil, errors.New("GATEWAY_API_KEY is required")
	}
	if cfg.CentralWebhookURL == "" {
		return nil, errors.New("CENTRAL_WEBHOOK_URL is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// LoadOrCreateNodeID returns this node's persistent identity, generating and
// storing it as <sessionDir>/node_id on first run. The backend keys the hub by
// server_key plus this ID so it can tell a legit reconnect (same node) apart
// from a second server that misuses the same server_key. It must survive
// restarts or every restart would look like a duplicate.
func LoadOrCreateNodeID(sessionDir string) (string, error) {
	p := filepath.Join(sessionDir, "node_id")
	if b, err := os.ReadFile(p); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id, nil
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	if err := os.WriteFile(p, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

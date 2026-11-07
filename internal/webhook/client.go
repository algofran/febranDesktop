package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type Payload struct {
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

func (c *Client) SendEvent(eventType string, payload interface{}) error {
	data := Payload{
		Event:     eventType,
		Data:      payload,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	go func() {
		retries := 3
		backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

		for i := 0; i <= retries; i++ {
			err := c.doRequest(body)
			if err == nil {
				// Success
				return
			}

			log.Printf("Webhook event %s failed (attempt %d/%d): %v\n", eventType, i+1, retries+1, err)
			if i < retries {
				time.Sleep(backoff[i])
			} else {
				log.Printf("Webhook event %s fully failed after %d retries\n", eventType, retries)
			}
		}
	}()

	return nil
}

func (c *Client) doRequest(body []byte) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

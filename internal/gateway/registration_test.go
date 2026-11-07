package gateway

import "testing"

func TestRegistrationNoticeIsKeptPerNumber(t *testing.T) {
	c := NewClient("ws://x", "k", "key", "node", nil)
	if _, ok := c.Registration("6281"); ok {
		t.Fatal("answer before the backend sent one")
	}
	c.handle(Message{Type: "registration", NumberID: "6281", Code: "limit", Error: "penuh"})
	c.handle(Message{Type: "registration", NumberID: "6282", Code: "registered", OK: true})
	if r, ok := c.Registration("6281"); !ok || r.Status != "limit" || r.Message != "penuh" {
		t.Fatalf("limit answer lost: %+v %v", r, ok)
	}
	c.handle(Message{Type: "registration", NumberID: "6281", Code: "registered", OK: true})
	if r, _ := c.Registration("6281"); r.Status != "registered" {
		t.Fatalf("a later answer must replace the old one: %+v", r)
	}
}

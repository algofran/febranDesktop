package whatsapp

import (
	"os"
	"testing"
	"time"
)

func TestInboundMediaKeptThenExpired(t *testing.T) {
	m := NewManager(t.TempDir())
	ref, err := m.SaveInboundMedia("+62812345", "3EB0ABC", []byte("data"))
	if err != nil || ref != "3EB0ABC" {
		t.Fatalf("ref=%q err=%v", ref, err)
	}
	if b, err := m.InboundMedia("62812345", ref); err != nil || string(b) != "data" {
		t.Fatalf("read back: %q %v", b, err)
	}
	if _, err := m.InboundMedia("62812345", "../../etc/passwd"); err == nil {
		t.Fatal("reference escaped the media folder")
	}
	p, _ := m.mediaFile("62812345", ref)
	old := time.Now().Add(-MediaRetention - time.Hour)
	_ = os.Chtimes(p, old, old)
	if _, err := m.InboundMedia("62812345", ref); err == nil {
		t.Fatal("expired media still served")
	}
	m.CleanupMedia()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("expired media not deleted")
	}
}

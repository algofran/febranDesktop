package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchMediaOnlyFromBackendUploads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PNGDATA:" + r.URL.Path))
	}))
	defer srv.Close()
	c := &Client{url: strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws/gateway"}

	data, name, err := c.fetchMedia("/api/v1/uploads/media/t1/a.png")
	if err != nil || string(data) != "PNGDATA:/api/v1/uploads/media/t1/a.png" || name != "a.png" {
		t.Fatalf("fetch: %q %q %v", data, name, err)
	}
	for _, p := range []string{
		"https://evil.example/x.png", "http://169.254.169.254/latest", "/api/v1/uploads/avatars/a.png",
		"/api/v1/uploads/media/../../etc/passwd", "/api/v1/uploads/media/t1/a.png?x=1", "//evil.example/api/v1/uploads/media/a",
	} {
		if _, _, err := c.fetchMedia(p); err == nil {
			t.Errorf("%q should be rejected", p)
		}
	}
}

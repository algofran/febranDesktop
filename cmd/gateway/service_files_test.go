package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRotatingWriterRotatesAndKeepsN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "gateway.log")
	w := newRotatingWriter(path, 20, 2)
	defer w.Close()
	for _, line := range []string{"aaaaaaaaaaaa\n", "bbbbbbbbbbbb\n", "cccccccccccc\n", "dddddddddddd\n"} {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, path); got != "dddddddddddd\n" {
		t.Fatalf("current log: %q", got)
	}
	if got := read(t, path+".1"); got != "cccccccccccc\n" {
		t.Fatalf(".1: %q", got)
	}
	if got := read(t, path+".2"); got != "bbbbbbbbbbbb\n" {
		t.Fatalf(".2: %q", got)
	}
	if _, err := os.Stat(path + ".3"); err == nil {
		t.Fatal("kept more than 2 rotated files")
	}
}

func TestCopySessionsNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "app"), filepath.Join(dir, "svc")
	write(t, filepath.Join(src, "6281.db"), "app-copy")
	write(t, filepath.Join(src, "node_id"), "node-abc\n")

	copied, err := copySessionsIfEmpty(src, dst)
	if err != nil || !copied {
		t.Fatalf("first copy: %v %v", copied, err)
	}
	if read(t, filepath.Join(dst, "node_id")) != "node-abc\n" {
		t.Fatal("node identity not carried over (backend would see a different node)")
	}

	// The service has moved on; re-installing must not roll it back.
	write(t, filepath.Join(dst, "6281.db"), "service-newer")
	copied, err = copySessionsIfEmpty(src, dst)
	if err != nil || copied || read(t, filepath.Join(dst, "6281.db")) != "service-newer" {
		t.Fatalf("overwrote newer service sessions: copied=%v err=%v", copied, err)
	}
	if c, err := copySessionsIfEmpty(filepath.Join(dir, "missing"), filepath.Join(dir, "x")); c || err != nil {
		t.Fatalf("missing source: %v %v", c, err)
	}
}

func TestRestoreDirBacksUpOldCopy(t *testing.T) {
	dir := t.TempDir()
	svc, app := filepath.Join(dir, "svc"), filepath.Join(dir, "app")
	write(t, filepath.Join(svc, "6281.db"), "newest")
	write(t, filepath.Join(app, "6281.db"), "stale")

	backup, err := restoreDir(svc, app)
	if err != nil || backup == "" {
		t.Fatalf("restore: %q %v", backup, err)
	}
	if read(t, filepath.Join(app, "6281.db")) != "newest" || read(t, filepath.Join(backup, "6281.db")) != "stale" {
		t.Fatal("app should hold newest sessions and the old ones be kept as backup")
	}
	if b, err := restoreDir(filepath.Join(dir, "none"), app); b != "" || err != nil {
		t.Fatalf("nothing to restore: %q %v", b, err)
	}
}

func TestCopyDirSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	write(t, filepath.Join(dir, "secret.txt"), "outside")
	write(t, filepath.Join(src, "ok.db"), "x")
	if err := os.Symlink(filepath.Join(dir, "secret.txt"), filepath.Join(src, "link")); err != nil {
		t.Skip("symlinks not permitted here")
	}
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "link")); err == nil {
		t.Fatal("symlink was followed/copied")
	}
}

func TestWriteServiceEnv(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "app.env"), filepath.Join(dir, "svc.env")
	write(t, src, "GATEWAY_API_KEY=\"abc def#1\"\nCENTRAL_WEBHOOK_URL=https://x.test/hook\nSESSION_DIR=D:/somewhere/else\nCENTRAL_API_KEY='k$ey'\n")
	if err := writeServiceEnv(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := godotenv.Read(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got["SESSION_DIR"] != "sessions" {
		t.Fatalf("SESSION_DIR must be forced next to the service config, got %q", got["SESSION_DIR"])
	}
	if got["GATEWAY_API_KEY"] != "abc def#1" || got["CENTRAL_API_KEY"] != "k$ey" || got["CENTRAL_WEBHOOK_URL"] != "https://x.test/hook" {
		t.Fatalf("values changed in round trip: %v", got)
	}

	write(t, src, "CENTRAL_WEBHOOK_URL=https://x.test/hook\n")
	if err := writeServiceEnv(src, dst); err == nil || !strings.Contains(err.Error(), "GATEWAY_API_KEY") {
		t.Fatalf("incomplete config accepted: %v", err)
	}
}

func TestTailLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "g.log")
	write(t, p, "one\r\ntwo\r\n\r\nthree\r\nfour\r\n")
	if got := tailLines(p, 2); got != "three\nfour" {
		t.Fatalf("got %q", got)
	}
	if tailLines(filepath.Join(t.TempDir(), "none"), 3) != "" {
		t.Fatal("missing file should give empty tail")
	}
}

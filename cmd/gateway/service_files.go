package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// ---- file helpers shared by the Windows service installer (no OS-specific code,
// so they are unit-tested everywhere) ----

// copyFile copies src to dst via a temp file + rename, so a crash never leaves a
// half-written binary or config behind.
func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// copyDir copies a directory tree. Only regular files and directories are
// copied (no symlinks), so a link inside the sessions folder can't pull in
// files from elsewhere.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(path, target, 0o600)
		}
		return nil
	})
}

func dirIsEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	return len(entries) == 0, err
}

// copySessionsIfEmpty seeds the service's session store from the app's, but
// never overwrites existing service sessions: they may be newer than the copy
// (WhatsApp rotates keys on every message). Reports whether it copied.
func copySessionsIfEmpty(src, dst string) (bool, error) {
	if src == "" {
		return false, nil
	}
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	empty, err := dirIsEmpty(dst)
	if err != nil || !empty {
		return false, err
	}
	return true, copyDir(src, dst)
}

// restoreDir replaces dst with the contents of src, keeping the previous dst as
// <dst>.bak-<timestamp>. Used when leaving service mode so the app continues
// from the newest sessions instead of a stale copy.
func restoreDir(src, dst string) (backup string, err error) {
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if _, err := os.Stat(dst); err == nil {
		backup = dst + ".bak-" + time.Now().Format("20060102-150405")
		if err := os.Rename(dst, backup); err != nil {
			return "", err
		}
	}
	return backup, copyDir(src, dst)
}

// writeServiceEnv writes the service's .env from the app's config. The service
// always keeps its sessions next to its config, whatever SESSION_DIR the app
// uses, so SESSION_DIR is forced to the relative "sessions".
func writeServiceEnv(srcEnv, dstEnv string) error {
	vars, err := godotenv.Read(srcEnv)
	if err != nil {
		return fmt.Errorf("gagal membaca konfigurasi: %w", err)
	}
	for _, k := range []string{"GATEWAY_API_KEY", "CENTRAL_WEBHOOK_URL"} {
		if strings.TrimSpace(vars[k]) == "" {
			return fmt.Errorf("konfigurasi belum lengkap: %s kosong", k)
		}
	}
	vars["SESSION_DIR"] = "sessions"
	body, err := godotenv.Marshal(vars)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dstEnv), ".env-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(body + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dstEnv)
}

// ---- rotating log file ----

// rotatingWriter appends to path and rotates to path.1 … path.<keep> once the
// file would exceed max bytes.
type rotatingWriter struct {
	mu   sync.Mutex
	path string
	max  int64
	keep int
	f    *os.File
	size int64
}

func newRotatingWriter(path string, max int64, keep int) *rotatingWriter {
	return &rotatingWriter{path: path, max: max, keep: keep}
}

func (w *rotatingWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

func (w *rotatingWriter) rotate() error {
	w.f.Close()
	w.f = nil
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.keep))
	for i := w.keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	_ = os.Rename(w.path, w.path+".1")
	return w.open()
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.size > 0 && w.size+int64(len(p)) > w.max {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// tailLines returns the last n non-empty lines of the file (best effort).
func tailLines(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	const window = 4096
	off := max(st.Size()-window, 0)
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && err != io.EOF {
		return ""
	}
	var lines []string
	for l := range strings.SplitSeq(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

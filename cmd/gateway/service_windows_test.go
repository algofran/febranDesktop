//go:build windows

package main

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// drive runs the service handler with fake SCM channels.
func drive(t *testing.T, env map[string]string) (reqs chan svc.ChangeRequest, status chan svc.Status, done chan [2]uint32) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	reqs, status, done = make(chan svc.ChangeRequest), make(chan svc.Status, 16), make(chan [2]uint32, 1)
	go func() {
		specific, code := gatewayService{}.Execute(nil, reqs, status)
		var b uint32
		if specific {
			b = 1
		}
		done <- [2]uint32{b, code}
	}()
	return
}

func TestExecuteStopsGracefully(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	reqs, status, done := drive(t, map[string]string{
		"GATEWAY_API_KEY": "k", "CENTRAL_WEBHOOK_URL": "http://127.0.0.1:1/x",
		"GATEWAY_BIND": "127.0.0.1:0", "SESSION_DIR": filepath.Join(dir, "sessions"),
		"CENTRAL_WS_URL": "", "GATEWAY_SERVER_KEY": "",
	})
	if s := <-status; s.State != svc.StartPending {
		t.Fatalf("first status %v", s.State)
	}
	if s := <-status; s.State != svc.Running || s.Accepts&svc.AcceptStop == 0 {
		t.Fatalf("second status %+v", s)
	}
	time.Sleep(500 * time.Millisecond) // let serve() come up
	reqs <- svc.ChangeRequest{Cmd: svc.Stop}
	if s := <-status; s.State != svc.StopPending {
		t.Fatalf("stop status %v", s.State)
	}
	select {
	case r := <-done:
		if r != [2]uint32{0, 0} {
			t.Fatalf("clean stop returned %v", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("service did not stop")
	}
}

func TestExecuteReportsStartupFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	_, _, done := drive(t, map[string]string{"GATEWAY_API_KEY": "", "CENTRAL_WEBHOOK_URL": ""})
	select {
	case r := <-done:
		if r != [2]uint32{1, 1} { // service-specific exit code 1 lets the SCM recovery policy retry
			t.Fatalf("failure returned %v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not exit on bad config")
	}
}

func TestStatusWithoutAdmin(t *testing.T) {
	res := dispatch("status", options{name: "FebranGatewayDoesNotExist", root: t.TempDir()})
	if !res.OK || res.Installed || res.State != "not_installed" {
		t.Fatalf("status of missing service: %+v", res)
	}
}

func TestMutatingCommandsNeedAdmin(t *testing.T) {
	if elevated() {
		t.Skip("running elevated")
	}
	for _, c := range []string{"install", "uninstall", "start", "stop", "restart"} {
		if res := dispatch(c, options{name: "FebranGatewayDoesNotExist", root: t.TempDir()}); res.OK || res.Message != "perlu hak Administrator" {
			t.Errorf("%s without admin: %+v", c, res)
		}
	}
}

func TestLayoutKeepsBinaryOutOfWritableData(t *testing.T) {
	l := layoutFor(`C:\ProgramData\Febran\Gateway`)
	if filepath.Dir(l.exe) != l.root || filepath.Dir(l.env) != l.data || filepath.Dir(l.sessions) != l.data || filepath.Dir(l.logs) != l.data {
		t.Fatalf("unexpected layout %+v", l)
	}
}

//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	defaultServiceName = "FebranGateway"
	serviceDisplayName = "Febran WA Gateway"
	serviceDescription = "Febran WhatsApp gateway node: connects WhatsApp devices to the Febran backend."
	installedExeName   = "febran-gateway.exe" // not "gateway.exe": the desktop app's orphan cleanup kills that name
	serviceAccount     = `NT AUTHORITY\LocalService`
)

// Well-known SIDs, so ACLs work on any Windows language.
const (
	sidSystem         = "*S-1-5-18"
	sidAdministrators = "*S-1-5-32-544"
	sidUsers          = "*S-1-5-32-545"
	sidLocalService   = "*S-1-5-19"
)

// layout of an installation:
//
//	<root>\febran-gateway.exe   binary (admins write; LocalService read/execute)
//	<root>\data\.env            config (LocalService modify; not readable by users)
//	<root>\data\sessions\       WhatsApp sessions
//	<root>\data\logs\           gateway.log (readable by all users)
type layout struct{ root, exe, data, env, sessions, logs string }

func defaultRoot() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "Febran", "Gateway")
}

func layoutFor(root string) layout {
	data := filepath.Join(root, "data")
	return layout{
		root: root, exe: filepath.Join(root, installedExeName), data: data,
		env: filepath.Join(data, ".env"), sessions: filepath.Join(data, "sessions"),
		logs: filepath.Join(data, "logs"),
	}
}

// ---------------- running as a service ----------------

func runningAsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// runAsService is the process entry when the Service Control Manager starts us.
// A service starts in System32, so move to the data dir first: godotenv and
// the relative SESSION_DIR resolve there.
func runAsService() int {
	l := layoutFor(defaultRoot())
	if err := os.Chdir(l.data); err != nil {
		return 1
	}
	closeLog := captureOutput(filepath.Join(l.logs, "gateway.log"))
	defer closeLog()

	if err := svc.Run(defaultServiceName, gatewayService{}); err != nil {
		log.Printf("service failed: %v", err)
		return 1
	}
	return 0
}

// captureOutput sends stdout, stderr and the log package (and the Go runtime's
// panic traces, which write to the OS stderr handle) into a rotating file.
func captureOutput(path string) (closeFn func()) {
	rw := newRotatingWriter(path, 10<<20, 3)
	r, w, err := os.Pipe()
	if err != nil {
		return func() {}
	}
	os.Stdout, os.Stderr = w, w
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(w.Fd()))
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(w.Fd()))
	log.SetOutput(w)
	logOut = w

	done := make(chan struct{})
	go func() { _, _ = io.Copy(rw, r); close(done) }()
	return func() {
		w.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		rw.Close()
	}
}

type gatewayService struct{}

func (gatewayService) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending, WaitHint: 30000}
				cancel()
				select {
				case <-done:
				case <-time.After(25 * time.Second):
				}
				return false, 0
			}
		case err := <-done:
			// serve ended by itself (bad config, port in use): report failure so
			// the SCM recovery policy can retry.
			if err != nil {
				log.Printf("gateway stopped: %v", err)
				return true, 1
			}
			return false, 0
		}
	}
}

// ---------------- `gateway service ...` commands ----------------

type result struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	Installed bool   `json:"installed"`
	State     string `json:"state"`
	DataDir   string `json:"data_dir"`
	LogFile   string `json:"log_file"`
}

type options struct {
	name, root, envFile, sessions, restoreSessions, resultFile string
	purge, skipACL                                             bool
}

// runServiceCommand returns the process exit code (0 ok, 1 failure). The result
// is printed as JSON and, when --result is given, written there too: an elevated
// process launched via UAC has no console the caller can read.
func runServiceCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gateway service <install|uninstall|start|stop|restart|status> [flags]")
		return 1
	}
	cmd := args[0]
	var o options
	fs := flag.NewFlagSet("service "+cmd, flag.ContinueOnError)
	fs.StringVar(&o.name, "name", defaultServiceName, "service name")
	fs.StringVar(&o.root, "root", defaultRoot(), "installation directory")
	fs.StringVar(&o.envFile, "env", "", "config (.env) to install")
	fs.StringVar(&o.sessions, "sessions", "", "existing sessions directory to copy on first install")
	fs.StringVar(&o.restoreSessions, "restore-sessions", "", "uninstall: copy the service's sessions back to this directory")
	fs.StringVar(&o.resultFile, "result", "", "write the JSON result to this file")
	fs.BoolVar(&o.purge, "purge", false, "uninstall: also delete the installation directory (WhatsApp sessions!)")
	fs.BoolVar(&o.skipACL, "skip-acl", false, "testing only: do not change permissions")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}

	res := dispatch(cmd, o)
	if b, err := json.Marshal(res); err == nil {
		fmt.Println(string(b))
		if o.resultFile != "" {
			_ = os.WriteFile(o.resultFile, b, 0o600)
		}
	}
	if res.OK {
		return 0
	}
	return 1
}

func dispatch(cmd string, o options) (res result) {
	l := layoutFor(o.root)
	defer func() {
		res.DataDir, res.LogFile = l.data, filepath.Join(l.logs, "gateway.log")
		res.Installed, res.State, _ = queryState(o.name)
	}()
	fail := func(err error) result { return result{Message: err.Error()} }

	if cmd != "status" && !elevated() {
		return fail(errors.New("perlu hak Administrator"))
	}
	var msg string
	var err error
	switch cmd {
	case "status":
		return result{OK: true}
	case "install":
		msg, err = install(o, l)
	case "uninstall":
		msg, err = uninstall(o, l)
	case "start":
		msg, err = control(o.name, "start")
	case "stop":
		msg, err = control(o.name, "stop")
	case "restart":
		if msg, err = control(o.name, "stop"); err == nil {
			msg, err = control(o.name, "start")
		}
	default:
		err = fmt.Errorf("perintah tidak dikenal: %s", cmd)
	}
	if err != nil {
		return fail(err)
	}
	return result{OK: true, Message: msg}
}

func elevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

func stateName(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	}
	return "unknown"
}

// queryState needs only query rights, so it works without Administrator
// (mgr.Connect/OpenService would demand full access).
func queryState(name string) (installed bool, state string, err error) {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false, "unknown", err
	}
	m := &mgr.Mgr{Handle: h}
	defer m.Disconnect()
	namePtr, _ := windows.UTF16PtrFromString(name)
	sh, err := windows.OpenService(h, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return false, "not_installed", nil
		}
		return false, "unknown", err
	}
	s := &mgr.Service{Name: name, Handle: sh}
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return true, "unknown", err
	}
	return true, stateName(st.State), nil
}

func openService(m *mgr.Mgr, name string) (*mgr.Service, error) {
	s, err := m.OpenService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil, nil
	}
	return s, err
}

func waitState(s *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service tidak mencapai status %s dalam %s (sekarang %s)", stateName(want), timeout, stateName(st.State))
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func stopService(s *mgr.Service) error {
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	if st.State != svc.StopPending {
		if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	return waitState(s, svc.Stopped, 35*time.Second)
}

// startService starts the service and confirms it stays up: a bad config or an
// occupied port makes the process exit a moment after "running" is reported.
func startService(s *mgr.Service, logFile string) error {
	if err := s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	if err := waitState(s, svc.Running, 20*time.Second); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State != svc.Running {
		return fmt.Errorf("service berhenti beberapa detik setelah start. Log terakhir:\n%s", tailLines(logFile, 5))
	}
	return nil
}

func control(name, action string) (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()
	s, err := openService(m, name)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", errors.New("service belum terpasang")
	}
	defer s.Close()
	if action == "stop" {
		if err := stopService(s); err != nil {
			return "", err
		}
		return "service dihentikan", nil
	}
	if err := startService(s, ""); err != nil {
		return "", err
	}
	return "service berjalan", nil
}

// icacls runs the system icacls (absolute path: this process is elevated, so
// never resolve helper programs through PATH or the working directory).
func icacls(args ...string) error {
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "icacls.exe")
	if out, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("icacls %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// secureDirs creates the layout with least-privilege permissions: the binary is
// read-only for the service account; only data\ is writable by it.
func secureDirs(l layout, applyACL bool) error {
	for _, d := range []string{l.root, l.data, l.sessions, l.logs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if !applyACL {
		return nil
	}
	steps := [][]string{
		{l.root, "/inheritance:r", "/grant:r", sidSystem + ":(OI)(CI)F", sidAdministrators + ":(OI)(CI)F", sidLocalService + ":(OI)(CI)RX"},
		{l.data, "/grant:r", sidLocalService + ":(OI)(CI)M"},
		{l.logs, "/grant", sidUsers + ":(OI)(CI)R"}, // log lines hold no message text
	}
	for _, s := range steps {
		if err := icacls(s...); err != nil {
			return err
		}
	}
	return nil
}

func install(o options, l layout) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := secureDirs(l, !o.skipACL); err != nil {
		return "", fmt.Errorf("gagal menyiapkan folder: %w", err)
	}
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()
	s, err := openService(m, o.name)
	if err != nil {
		return "", err
	}
	existed := s != nil
	if existed {
		defer s.Close()
		if err := stopService(s); err != nil {
			return "", fmt.Errorf("gagal menghentikan service lama: %w", err)
		}
	}

	if !strings.EqualFold(filepath.Clean(self), filepath.Clean(l.exe)) {
		if err := copyFile(self, l.exe, 0o755); err != nil {
			return "", fmt.Errorf("gagal menyalin program: %w", err)
		}
	}
	if o.envFile != "" {
		if err := writeServiceEnv(o.envFile, l.env); err != nil {
			return "", err
		}
	} else if _, err := os.Stat(l.env); err != nil {
		return "", errors.New("konfigurasi (.env) tidak diberikan")
	}
	copied, err := copySessionsIfEmpty(o.sessions, l.sessions)
	if err != nil {
		return "", fmt.Errorf("gagal menyalin sesi WhatsApp: %w", err)
	}

	cfg := mgr.Config{
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
		ServiceStartName: serviceAccount,
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
	}
	if existed {
		cur, err := s.Config()
		if err != nil {
			return "", err
		}
		cur.StartType, cur.DelayedAutoStart = cfg.StartType, cfg.DelayedAutoStart
		cur.DisplayName, cur.Description = cfg.DisplayName, cfg.Description
		cur.BinaryPathName = windows.ComposeCommandLine([]string{l.exe})
		if err := s.UpdateConfig(cur); err != nil {
			return "", err
		}
	} else {
		if s, err = m.CreateService(o.name, l.exe, cfg); err != nil {
			return "", err
		}
		defer s.Close()
	}
	// Restart automatically if the process crashes (5s, 30s, 60s; counter resets daily).
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 86400); err != nil {
		return "", err
	}

	if err := startService(s, filepath.Join(l.logs, "gateway.log")); err != nil {
		return "", err
	}
	msg := "service terpasang dan berjalan"
	if copied {
		msg += "; sesi WhatsApp disalin dari aplikasi"
	}
	return msg, nil
}

func uninstall(o options, l layout) (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()
	s, err := openService(m, o.name)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", errors.New("service belum terpasang")
	}
	defer s.Close()
	if err := stopService(s); err != nil {
		return "", err
	}
	if err := s.Delete(); err != nil {
		return "", err
	}
	msg := "service dihapus"
	if o.restoreSessions != "" {
		backup, err := restoreDir(l.sessions, o.restoreSessions)
		if err != nil {
			return "", fmt.Errorf("service dihapus, tapi gagal memulihkan sesi: %w", err)
		}
		msg += "; sesi WhatsApp dipulihkan ke aplikasi"
		if backup != "" {
			msg += " (salinan lama: " + backup + ")"
		}
	}
	if o.purge {
		// Only delete a directory that really is a gateway installation.
		if _, err := os.Stat(l.exe); err != nil {
			return "", errors.New("folder instalasi tidak dikenali, tidak dihapus")
		}
		if err := os.RemoveAll(l.root); err != nil {
			return "", fmt.Errorf("service dihapus, tapi gagal menghapus data: %w", err)
		}
		msg += "; data service dihapus"
	}
	return msg, nil
}

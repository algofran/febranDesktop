use std::fs;
use std::io::{BufRead, BufReader};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, State};

#[derive(Default)]
pub struct AppState {
    process: Arc<Mutex<Option<Child>>>,
    config: Arc<Mutex<GatewayConfig>>,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct GatewayConfig {
    pub gateway_bind: String,
    pub gateway_api_key: String,
    pub central_webhook_url: String,
    pub central_ws_url: String,
    pub gateway_server_key: String,
    pub central_api_key: String,
    pub session_dir: String,
    pub log_level: String,
}

/// The central server every installed node talks to. Users do not choose it: the panel hides these
/// fields and the saved .env cannot override them. The API key is not in the source; it is baked in
/// at build time (see build.rs) from FEBRAN_CENTRAL_API_KEY or src-tauri/central-key.txt.
const CENTRAL_WEBHOOK_URL: &str = "https://febran.com/api/v1/webhooks/gateway";
const CENTRAL_WS_URL: &str = "wss://febran.com/ws/gateway";
const CENTRAL_API_KEY: &str = match option_env!("FEBRAN_CENTRAL_API_KEY") {
    Some(k) => k,
    None => "dev-central-key-2024",
};

/// Developers pointing a build at a local backend can set FEBRAN_CENTRAL_OVERRIDE=1 and use .env.
fn central_override() -> bool {
    std::env::var("FEBRAN_CENTRAL_OVERRIDE").map(|v| v == "1").unwrap_or(false)
}

fn lock_central(config: &mut GatewayConfig) {
    config.central_webhook_url = CENTRAL_WEBHOOK_URL.to_string();
    config.central_ws_url = CENTRAL_WS_URL.to_string();
    config.central_api_key = CENTRAL_API_KEY.to_string();
}

impl Default for GatewayConfig {
    fn default() -> Self {
        Self {
            gateway_bind: "127.0.0.1:8989".to_string(),
            gateway_api_key: "dev-gateway-key-2024".to_string(),
            central_webhook_url: CENTRAL_WEBHOOK_URL.to_string(),
            central_ws_url: CENTRAL_WS_URL.to_string(),
            gateway_server_key: "node-1".to_string(),
            central_api_key: CENTRAL_API_KEY.to_string(),
            session_dir: "./sessions".to_string(),
            log_level: "info".to_string(),
        }
    }
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ProcessStatus {
    pub running: bool,
    pub pid: Option<u32>,
    pub bind: String,
    pub is_healthy: bool,
    pub ws_connected: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct LogMessage {
    pub level: String,
    pub message: String,
    pub timestamp: String,
}

fn get_env_file_path() -> PathBuf {
    let current_dir = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    current_dir.join(".env")
}

fn load_env_config() -> GatewayConfig {
    let env_path = get_env_file_path();
    let mut config = GatewayConfig::default();

    if let Ok(content) = fs::read_to_string(&env_path) {
        for line in content.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if let Some((k, v)) = line.split_once('=') {
                let key = k.trim();
                let val = v.trim().trim_matches('"').trim_matches('\'');
                match key {
                    "GATEWAY_BIND" => config.gateway_bind = val.to_string(),
                    "GATEWAY_API_KEY" => config.gateway_api_key = val.to_string(),
                    "CENTRAL_WEBHOOK_URL" => config.central_webhook_url = val.to_string(),
                    "CENTRAL_WS_URL" => config.central_ws_url = val.to_string(),
                    "GATEWAY_SERVER_KEY" => config.gateway_server_key = val.to_string(),
                    "CENTRAL_API_KEY" => config.central_api_key = val.to_string(),
                    "SESSION_DIR" => config.session_dir = val.to_string(),
                    "LOG_LEVEL" => config.log_level = val.to_string(),
                    _ => {}
                }
            }
        }
    }
    if !central_override() {
        lock_central(&mut config);
    }
    config
}

/// A value with a line break would inject extra KEY=VALUE lines into the .env.
fn check_env_values(config: &GatewayConfig) -> Result<(), String> {
    for v in [
        &config.gateway_bind, &config.gateway_api_key, &config.central_webhook_url,
        &config.central_ws_url, &config.gateway_server_key, &config.central_api_key,
        &config.session_dir, &config.log_level,
    ] {
        if v.contains('\n') || v.contains('\r') {
            return Err("Konfigurasi tidak boleh mengandung baris baru".into());
        }
    }
    Ok(())
}

fn render_env(config: &GatewayConfig) -> String {
    format!(
        "# Gateway Configuration\n\
        GATEWAY_BIND={}\n\
        GATEWAY_API_KEY={}\n\n\
        # Central Backend\n\
        CENTRAL_WEBHOOK_URL={}\n\
        CENTRAL_WS_URL={}\n\
        GATEWAY_SERVER_KEY={}\n\
        CENTRAL_API_KEY={}\n\n\
        # Session Storage\n\
        SESSION_DIR={}\n\
        LOG_LEVEL={}\n",
        config.gateway_bind,
        config.gateway_api_key,
        config.central_webhook_url,
        config.central_ws_url,
        config.gateway_server_key,
        config.central_api_key,
        config.session_dir,
        config.log_level
    )
}

fn save_env_config(config: &GatewayConfig) -> Result<(), String> {
    check_env_values(config)?;
    fs::write(get_env_file_path(), render_env(config)).map_err(|e| e.to_string())
}

// ---------------------------------------------------------------------------
// Windows service support
// ---------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
#[serde(default)]
pub struct ServiceInfo {
    pub ok: bool,
    pub message: String,
    pub installed: bool,
    /// running | stopped | start_pending | stop_pending | not_installed | unknown
    pub state: String,
    pub data_dir: String,
    pub log_file: String,
}

fn project_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| PathBuf::from("."))
}

/// The gateway program the app runs and installs as a service: the dev sidecar,
/// then one next to the app executable (bundled build), then a root build.
fn gateway_binary() -> Option<PathBuf> {
    let crate_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let sidecar = if cfg!(windows) { "gateway-x86_64-pc-windows-msvc.exe" } else { "gateway" };
    let mut candidates = vec![crate_dir.join("binaries").join(sidecar)];
    if let Some(dir) = std::env::current_exe().ok().and_then(|p| p.parent().map(|d| d.to_path_buf())) {
        candidates.push(dir.join(if cfg!(windows) { "gateway.exe" } else { "gateway" }));
    }
    candidates.push(project_root().join("gateway.exe"));
    candidates.into_iter().find(|p| p.exists())
}

/// Session folder as the app's own gateway resolves it (relative to the project root).
fn resolved_session_dir(config: &GatewayConfig) -> PathBuf {
    let p = PathBuf::from(&config.session_dir);
    if p.is_absolute() { p } else { project_root().join(p) }
}

#[cfg(windows)]
mod winsvc {
    use super::*;
    use std::os::windows::process::CommandExt;

    const CREATE_NO_WINDOW: u32 = 0x08000000;

    fn system32() -> PathBuf {
        PathBuf::from(std::env::var("SystemRoot").unwrap_or_else(|_| r"C:\Windows".into())).join("System32")
    }

    fn temp_file(ext: &str) -> PathBuf {
        let nanos = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_nanos())
            .unwrap_or_default();
        std::env::temp_dir().join(format!("febran-gw-{}-{}.{}", std::process::id(), nanos, ext))
    }

    /// One command-line argument, quoted for CreateProcess. Paths cannot contain
    /// a double quote on Windows; a trailing backslash would escape the closing one.
    fn quote(arg: &str) -> Result<String, String> {
        if arg.contains('"') {
            return Err("Argumen tidak valid".into());
        }
        Ok(format!("\"{}\"", arg.trim_end_matches('\\')))
    }

    /// `gateway service status` - needs no Administrator rights.
    pub fn status() -> Result<ServiceInfo, String> {
        let Some(exe) = gateway_binary() else {
            return Ok(ServiceInfo { state: "not_installed".into(), ..Default::default() });
        };
        let out = Command::new(exe)
            .args(["service", "status"])
            .creation_flags(CREATE_NO_WINDOW)
            .output()
            .map_err(|e| format!("Gagal menjalankan gateway: {}", e))?;
        let text = String::from_utf8_lossy(&out.stdout);
        let line = text.lines().rev().find(|l| l.trim_start().starts_with('{')).unwrap_or("");
        serde_json::from_str::<ServiceInfo>(line).map_err(|_| "Status service tidak terbaca".to_string())
    }

    /// Runs `gateway service <sub> ...` elevated (one UAC prompt) and waits. The
    /// elevated process has no console we can read, so it reports through a file.
    pub fn run_elevated(sub: &str, extra: &[String]) -> Result<ServiceInfo, String> {
        let exe = gateway_binary()
            .ok_or("Program gateway tidak ditemukan. Jalankan `npm run build:sidecar` terlebih dulu.")?;
        let result_path = temp_file("json");

        let mut args = vec!["service".to_string(), sub.to_string()];
        args.extend(extra.iter().cloned());
        args.push("--result".into());
        args.push(result_path.to_string_lossy().into_owned());
        let arg_line = args.iter().map(|a| quote(a)).collect::<Result<Vec<_>, _>>()?.join(" ");

        // Program and arguments travel in environment variables so no user value
        // is ever spliced into the PowerShell script text.
        let script = "try { $p = Start-Process -FilePath $env:FG_EXE -ArgumentList $env:FG_ARGS -Verb RunAs -Wait -PassThru -WindowStyle Hidden; exit $p.ExitCode } catch { [Console]::Error.WriteLine($_.Exception.Message); exit 1223 }";
        let out = Command::new(system32().join(r"WindowsPowerShell\v1.0\powershell.exe"))
            .args(["-NoProfile", "-NonInteractive", "-Command", script])
            .env("FG_EXE", &exe)
            .env("FG_ARGS", &arg_line)
            .creation_flags(CREATE_NO_WINDOW)
            .output()
            .map_err(|e| format!("Gagal meminta izin Administrator: {}", e))?;

        let parsed = fs::read_to_string(&result_path)
            .ok()
            .and_then(|s| serde_json::from_str::<ServiceInfo>(&s).ok());
        let _ = fs::remove_file(&result_path);
        match parsed {
            Some(info) if info.ok => Ok(info),
            Some(info) => Err(info.message),
            None if out.status.code() == Some(1223) => {
                let why = String::from_utf8_lossy(&out.stderr).trim().to_string();
                Err(format!("Izin Administrator (UAC) dibatalkan atau gagal. {}", why).trim().to_string())
            }
            None => Err(format!("Perintah service gagal (kode {}).", out.status.code().unwrap_or(-1))),
        }
    }

    /// Writes the app's current settings to a private temp file for the installer.
    pub fn temp_env(config: &GatewayConfig) -> Result<PathBuf, String> {
        check_env_values(config)?;
        let path = temp_file("env");
        fs::write(&path, render_env(config)).map_err(|e| e.to_string())?;
        Ok(path)
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn quote_handles_spaces_and_trailing_backslash() {
            assert_eq!(quote(r"C:\Program Files\x.exe").unwrap(), r#""C:\Program Files\x.exe""#);
            // a trailing backslash must not escape the closing quote
            assert_eq!(quote(r"D:\data\sessions\").unwrap(), r#""D:\data\sessions""#);
        }

        #[test]
        fn quote_rejects_embedded_quotes() {
            assert!(quote(r#"a" --env "b"#).is_err());
        }
    }
}

/// Stops the app's own gateway child (and strays) so only one process owns the
/// WhatsApp sessions. Two processes on one session log the device out.
#[cfg(windows)]
fn stop_app_gateway(state: &AppState) {
    if let Some(mut child) = state.process.lock().unwrap().take() {
        let _ = child.kill();
        let _ = child.wait();
    }
    kill_orphan_gateways();
    std::thread::sleep(std::time::Duration::from_millis(800)); // let SQLite release the files
}

#[tauri::command]
async fn service_status() -> Result<ServiceInfo, String> {
    #[cfg(windows)]
    {
        tauri::async_runtime::spawn_blocking(winsvc::status).await.map_err(|e| e.to_string())?
    }
    #[cfg(not(windows))]
    {
        Ok(ServiceInfo { state: "not_installed".into(), ..Default::default() })
    }
}

/// Installs (or updates) the service from the app's current settings, copies the
/// WhatsApp sessions on first install, and starts it. One UAC prompt.
#[tauri::command]
async fn service_install(state: State<'_, AppState>) -> Result<ServiceInfo, String> {
    #[cfg(windows)]
    {
        let config = load_env_config();
        let sessions = resolved_session_dir(&config);
        stop_app_gateway(&state);
        let env_file = winsvc::temp_env(&config)?;
        let extra = vec![
            "--env".to_string(), env_file.to_string_lossy().into_owned(),
            "--sessions".to_string(), sessions.to_string_lossy().into_owned(),
        ];
        let res = tauri::async_runtime::spawn_blocking(move || winsvc::run_elevated("install", &extra))
            .await
            .map_err(|e| e.to_string());
        let _ = fs::remove_file(&env_file);
        res?
    }
    #[cfg(not(windows))]
    {
        let _ = state;
        Err("Service hanya didukung di Windows".into())
    }
}

/// Removes the service. The newest WhatsApp sessions are copied back for the app
/// (its older copy is kept as a .bak folder). `purge` also deletes the service data.
#[tauri::command]
async fn service_uninstall(purge: bool) -> Result<ServiceInfo, String> {
    #[cfg(windows)]
    {
        let sessions = resolved_session_dir(&load_env_config());
        let mut extra = vec!["--restore-sessions".to_string(), sessions.to_string_lossy().into_owned()];
        if purge {
            extra.push("--purge".into());
        }
        tauri::async_runtime::spawn_blocking(move || winsvc::run_elevated("uninstall", &extra))
            .await
            .map_err(|e| e.to_string())?
    }
    #[cfg(not(windows))]
    {
        let _ = purge;
        Err("Service hanya didukung di Windows".into())
    }
}

/// action: "start" | "stop" | "restart"
#[tauri::command]
async fn service_control(action: String) -> Result<ServiceInfo, String> {
    if !["start", "stop", "restart"].contains(&action.as_str()) {
        return Err("Aksi tidak dikenal".into());
    }
    #[cfg(windows)]
    {
        tauri::async_runtime::spawn_blocking(move || winsvc::run_elevated(&action, &[]))
            .await
            .map_err(|e| e.to_string())?
    }
    #[cfg(not(windows))]
    {
        Err("Service hanya didukung di Windows".into())
    }
}

/// Last lines of the service's log file (readable by every user by design: the
/// gateway logs request lines only, never message text).
#[tauri::command]
async fn service_log_tail(lines: usize) -> Result<Vec<String>, String> {
    use std::io::{Read, Seek, SeekFrom};
    #[cfg(windows)]
    let log_file = tauri::async_runtime::spawn_blocking(winsvc::status)
        .await
        .map_err(|e| e.to_string())??
        .log_file;
    #[cfg(not(windows))]
    let log_file = String::new();
    if log_file.is_empty() {
        return Ok(vec![]);
    }
    let mut f = match fs::File::open(&log_file) {
        Ok(f) => f,
        Err(_) => return Ok(vec![]), // not created yet
    };
    let len = f.metadata().map_err(|e| e.to_string())?.len();
    let start = len.saturating_sub(64 * 1024);
    f.seek(SeekFrom::Start(start)).map_err(|e| e.to_string())?;
    let mut buf = Vec::new();
    f.read_to_end(&mut buf).map_err(|e| e.to_string())?;
    let text = String::from_utf8_lossy(&buf);
    let mut out: Vec<String> = text.lines().filter(|l| !l.trim().is_empty()).map(|l| l.to_string()).collect();
    if start > 0 && !out.is_empty() {
        out.remove(0); // first line is cut mid-way by the seek
    }
    let keep = lines.clamp(1, 1000);
    let skip = out.len().saturating_sub(keep);
    Ok(out.split_off(skip))
}

#[tauri::command]
fn get_config(state: State<AppState>) -> GatewayConfig {
    let cfg = load_env_config();
    let mut current_cfg = state.config.lock().unwrap();
    *current_cfg = cfg.clone();
    // the panel never needs the central key; do not hand it to the webview
    GatewayConfig { central_api_key: String::new(), ..cfg }
}

#[tauri::command]
fn save_config(mut config: GatewayConfig, state: State<AppState>) -> Result<bool, String> {
    if !central_override() {
        lock_central(&mut config); // whatever the panel sent, the central server is fixed
    }
    save_env_config(&config)?;
    let mut current_cfg = state.config.lock().unwrap();
    *current_cfg = config;
    Ok(true)
}

#[tauri::command]
async fn start_gateway(app_handle: AppHandle, state: State<'_, AppState>) -> Result<ProcessStatus, String> {
    {
        let mut proc_guard = state.process.lock().unwrap();
        if let Some(ref mut child) = *proc_guard {
            match child.try_wait() {
                Ok(None) => {
                    let config = state.config.lock().unwrap().clone();
                    return Ok(ProcessStatus {
                        running: true,
                        pid: Some(child.id()),
                        bind: config.gateway_bind,
                        is_healthy: true,
                        ws_connected: false,
                    });
                }
                _ => {
                    *proc_guard = None;
                }
            }
        }
    }

    // While the Windows service is installed it owns the WhatsApp sessions; a
    // second gateway on a stale copy would desync them and log the device out.
    #[cfg(windows)]
    if winsvc::status().map(|s| s.installed).unwrap_or(false) {
        return Err("Gateway terpasang sebagai service Windows. Kelola lewat panel Service, atau hapus service untuk kembali ke mode aplikasi.".into());
    }

    let config = load_env_config();
    {
        let mut cfg_lock = state.config.lock().unwrap();
        *cfg_lock = config.clone();
    }

    // Kill any orphaned gateway from a previous run that is still holding the
    // bind port. A fresh daemon cannot bind otherwise and would exit instantly.
    kill_orphan_gateways();

    // Resolve the sidecar against the crate dir (absolute, compile-time), NOT the
    // runtime cwd. `tauri dev` runs this process with cwd = src-tauri/, so the old
    // relative "src-tauri/binaries/..." and "gateway.exe" lookups both missed and
    // the code fell through to `go run` with a broken relative path.
    let crate_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let project_root = crate_dir
        .parent()
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| PathBuf::from("."));

    let exe_name = if cfg!(windows) { "gateway-x86_64-pc-windows-msvc.exe" } else { "gateway" };
    let sidecar_path = crate_dir.join("binaries").join(exe_name);
    let fallback_path = project_root.join("gateway.exe");
    // The gateway is a multi-file package, so `go run` needs the package path, not main.go.
    let gateway_pkg = PathBuf::from("./cmd/gateway");

    let binary_to_run = if sidecar_path.exists() {
        sidecar_path
    } else if fallback_path.exists() {
        fallback_path
    } else {
        PathBuf::from("go") // Fallback: run via `go run ./cmd/gateway`
    };

    let mut cmd = if binary_to_run.to_string_lossy() == "go" {
        let mut c = Command::new("go");
        c.arg("run").arg(&gateway_pkg);
        c.current_dir(&project_root);
        c
    } else {
        let mut c = Command::new(&binary_to_run);
        // .env and ./sessions are resolved by the Go daemon against its cwd.
        c.current_dir(&project_root);
        c
    };

    cmd.env("GATEWAY_BIND", &config.gateway_bind)
        .env("GATEWAY_API_KEY", &config.gateway_api_key)
        .env("CENTRAL_WEBHOOK_URL", &config.central_webhook_url)
        .env("CENTRAL_WS_URL", &config.central_ws_url)
        .env("GATEWAY_SERVER_KEY", &config.gateway_server_key)
        .env("CENTRAL_API_KEY", &config.central_api_key)
        .env("SESSION_DIR", &config.session_dir)
        .env("LOG_LEVEL", &config.log_level)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        // CREATE_NO_WINDOW = 0x08000000
        cmd.creation_flags(0x08000000);
    }

    let mut child = cmd.spawn().map_err(|e| format!("Failed to spawn gateway process: {}", e))?;
    let pid = child.id();

    // Stream stdout & stderr to Tauri frontend
    if let Some(stdout) = child.stdout.take() {
        let handle = app_handle.clone();
        std::thread::spawn(move || {
            let reader = BufReader::new(stdout);
            for line in reader.lines().map_while(Result::ok) {
                let _ = handle.emit("gateway-log", LogMessage {
                    level: "stdout".into(),
                    message: line,
                    timestamp: chrono_now(),
                });
            }
        });
    }

    if let Some(stderr) = child.stderr.take() {
        let handle = app_handle.clone();
        std::thread::spawn(move || {
            let reader = BufReader::new(stderr);
            for line in reader.lines().map_while(Result::ok) {
                let _ = handle.emit("gateway-log", LogMessage {
                    level: "stderr".into(),
                    message: line,
                    timestamp: chrono_now(),
                });
            }
        });
    }

    {
        let mut proc_guard = state.process.lock().unwrap();
        *proc_guard = Some(child);
    }

    Ok(ProcessStatus {
        running: true,
        pid: Some(pid),
        bind: config.gateway_bind,
        is_healthy: true,
        ws_connected: false,
    })
}

#[tauri::command]
fn stop_gateway(state: State<AppState>) -> Result<bool, String> {
    let mut proc_guard = state.process.lock().unwrap();
    if let Some(mut child) = proc_guard.take() {
        let _ = child.kill();
        let _ = child.wait();
        return Ok(true);
    }
    Ok(false)
}

#[tauri::command]
async fn get_gateway_status(state: State<'_, AppState>) -> Result<ProcessStatus, String> {
    let (pid, is_running) = {
        let mut proc_guard = state.process.lock().unwrap();
        if let Some(ref mut child) = *proc_guard {
            match child.try_wait() {
                Ok(None) => (Some(child.id()), true),
                _ => {
                    *proc_guard = None;
                    (None, false)
                }
            }
        } else {
            (None, false)
        }
    };

    let config = {
        let cfg_guard = state.config.lock().unwrap();
        cfg_guard.clone()
    };

    let (is_healthy, ws_connected) = check_health(&config.gateway_bind).await;

    Ok(ProcessStatus {
        running: is_running || is_healthy,
        pid,
        bind: config.gateway_bind,
        is_healthy,
        ws_connected,
    })
}

async fn check_health(bind: &str) -> (bool, bool) {
    let target = if bind.starts_with("0.0.0.0:") {
        bind.replacen("0.0.0.0:", "127.0.0.1:", 1)
    } else {
        bind.to_string()
    };
    let url = format!("http://{}/health", target);
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_millis(500))
        .build()
        .unwrap_or_default();

    match client.get(&url).send().await {
        Ok(resp) => {
            let healthy = resp.status().is_success();
            let ws = resp
                .json::<serde_json::Value>()
                .await
                .map(|v| v.get("ws_connected").and_then(|x| x.as_bool()).unwrap_or(false))
                .unwrap_or(false);
            (healthy, ws)
        }
        Err(_) => (false, false),
    }
}

fn kill_orphan_gateways() {
    #[cfg(windows)]
    {
        for name in ["gateway-x86_64-pc-windows-msvc.exe", "gateway.exe"] {
            let _ = Command::new("taskkill")
                .args(["/F", "/IM", name])
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .status();
        }
    }
}

fn chrono_now() -> String {
    let now = std::time::SystemTime::now();
    let duration = now.duration_since(std::time::UNIX_EPOCH).unwrap_or_default();
    let secs = duration.as_secs();
    let hours = (secs / 3600) % 24;
    let mins = (secs / 60) % 60;
    let s = secs % 60;
    format!("{:02}:{:02}:{:02}", hours, mins, s)
}

// Forwarding gateway API commands directly through Rust client
#[tauri::command]
async fn proxy_gateway_api(
    method: String,
    path: String,
    body: Option<serde_json::Value>,
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let config = {
        let cfg_guard = state.config.lock().unwrap();
        cfg_guard.clone()
    };
    
    let target = if config.gateway_bind.starts_with("0.0.0.0:") {
        config.gateway_bind.replacen("0.0.0.0:", "127.0.0.1:", 1)
    } else {
        config.gateway_bind.clone()
    };

    let url = format!("http://{}{}", target, path);
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(15))
        .build()
        .map_err(|e| e.to_string())?;

    let mut req = match method.to_uppercase().as_str() {
        "POST" => client.post(&url),
        "PUT" => client.put(&url),
        "DELETE" => client.delete(&url),
        _ => client.get(&url),
    };

    req = req.header("X-Gateway-Key", &config.gateway_api_key);
    req = req.header("Content-Type", "application/json");

    if let Some(json_body) = body {
        req = req.json(&json_body);
    }

    let resp = req.send().await.map_err(|e| format!("Gateway request failed: {}", e))?;
    let status = resp.status();
    let json = resp.json::<serde_json::Value>().await.map_err(|e| format!("Failed to parse response: {}", e))?;

    if !status.is_success() {
        return Err(format!("Gateway returned status {}: {:?}", status, json));
    }

    Ok(json)
}

pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_http::init())
        .plugin(tauri_plugin_process::init())
        .manage(AppState::default())
        .invoke_handler(tauri::generate_handler![
            get_config,
            save_config,
            start_gateway,
            stop_gateway,
            get_gateway_status,
            service_status,
            service_install,
            service_uninstall,
            service_control,
            service_log_tail,
            proxy_gateway_api
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn env_values_with_line_breaks_are_rejected() {
        let mut cfg = GatewayConfig::default();
        assert!(check_env_values(&cfg).is_ok());
        cfg.gateway_api_key = "abc\nCENTRAL_API_KEY=evil".into();
        assert!(check_env_values(&cfg).is_err());
        cfg.gateway_api_key = "abc".into();
        cfg.session_dir = "x\r\ny".into();
        assert!(check_env_values(&cfg).is_err());
    }

    #[test]
    fn central_server_cannot_be_overridden_from_the_panel() {
        let mut cfg = GatewayConfig::default();
        cfg.central_webhook_url = "http://evil.example/hook".into();
        cfg.central_ws_url = "ws://evil.example/ws".into();
        cfg.central_api_key = "stolen".into();
        lock_central(&mut cfg);
        assert_eq!(cfg.central_webhook_url, CENTRAL_WEBHOOK_URL);
        assert_eq!(cfg.central_ws_url, CENTRAL_WS_URL);
        assert_eq!(cfg.central_api_key, CENTRAL_API_KEY);
    }

    #[test]
    fn rendered_env_has_one_key_per_line() {
        let cfg = GatewayConfig::default();
        let text = render_env(&cfg);
        assert!(text.contains("GATEWAY_BIND=127.0.0.1:8989\n"));
        assert!(text.contains(&format!("CENTRAL_WS_URL={}\n", cfg.central_ws_url)));
    }
}

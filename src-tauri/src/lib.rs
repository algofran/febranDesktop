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

impl Default for GatewayConfig {
    fn default() -> Self {
        Self {
            gateway_bind: "127.0.0.1:8989".to_string(),
            gateway_api_key: "dev-gateway-key-2024".to_string(),
            central_webhook_url: "http://localhost:8080/api/v1/webhooks/gateway".to_string(),
            central_ws_url: "ws://localhost:8080/ws/gateway".to_string(),
            gateway_server_key: "node-1".to_string(),
            central_api_key: "dev-central-key-2024".to_string(),
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
    config
}

fn save_env_config(config: &GatewayConfig) -> Result<(), String> {
    let env_path = get_env_file_path();
    let content = format!(
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
    );
    fs::write(&env_path, content).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_config(state: State<AppState>) -> GatewayConfig {
    let cfg = load_env_config();
    let mut current_cfg = state.config.lock().unwrap();
    *current_cfg = cfg.clone();
    cfg
}

#[tauri::command]
fn save_config(config: GatewayConfig, state: State<AppState>) -> Result<bool, String> {
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
    let main_go = project_root.join("cmd").join("gateway").join("main.go");

    let binary_to_run = if sidecar_path.exists() {
        sidecar_path
    } else if fallback_path.exists() {
        fallback_path
    } else {
        PathBuf::from("go") // Fallback: run via `go run ./cmd/gateway/main.go`
    };

    let mut cmd = if binary_to_run.to_string_lossy() == "go" {
        let mut c = Command::new("go");
        c.arg("run").arg(&main_go);
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
            proxy_gateway_api
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

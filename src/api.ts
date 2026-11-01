import type { GatewayConfig, ProcessStatus, LogMessage, GroupInfo, ContactInfo, SessionInfo, SessionSyncData, AllSyncResult, ServiceInfo } from "./types";

declare global {
  interface Window {
    __TAURI_INTERNALS__?: unknown;
    __TAURI__?: unknown;
  }
}

export const isTauri = (): boolean => {
  return typeof window !== "undefined" && (!!window.__TAURI_INTERNALS__ || !!window.__TAURI__);
};

// Dynamic Tauri invoke
async function tauriInvoke<T>(cmd: string, args?: Record<string, unknown>): Promise<T> {
  if (isTauri()) {
    try {
      const { invoke } = await import("@tauri-apps/api/core");
      return await invoke<T>(cmd, args);
    } catch (err) {
      console.warn(`Tauri invoke error (${cmd}):`, err);
      throw err;
    }
  }
  throw new Error("Not running in Tauri environment");
}

export const api = {
  // Config
  async getConfig(): Promise<GatewayConfig> {
    if (isTauri()) {
      return await tauriInvoke<GatewayConfig>("get_config");
    }
    return {
      gateway_bind: "127.0.0.1:8989",
      gateway_api_key: "dev-gateway-key-2024",
      central_webhook_url: "http://localhost:8080/api/v1/webhooks/gateway",
      central_ws_url: "ws://localhost:8080/ws/gateway",
      gateway_server_key: "node-1",
      central_api_key: "dev-central-key-2024",
      session_dir: "./sessions",
      log_level: "info",
    };
  },

  async saveConfig(config: GatewayConfig): Promise<boolean> {
    if (isTauri()) {
      return await tauriInvoke<boolean>("save_config", { config });
    }
    console.log("Mock saved config in browser mode:", config);
    return true;
  },

  // Gateway Lifecycle
  async getStatus(): Promise<ProcessStatus> {
    if (isTauri()) {
      return await tauriInvoke<ProcessStatus>("get_gateway_status");
    }
    try {
      const res = await fetch("http://localhost:8989/health", { method: "GET" });
      const body = await res.json().catch(() => ({ ws_connected: false }));
      return {
        running: res.ok,
        pid: null,
        bind: "127.0.0.1:8989",
        is_healthy: res.ok,
        ws_connected: !!body.ws_connected,
      };
    } catch {
      return {
        running: false,
        pid: null,
        bind: "127.0.0.1:8989",
        is_healthy: false,
        ws_connected: false,
      };
    }
  },

  async startGateway(): Promise<ProcessStatus> {
    if (isTauri()) {
      return await tauriInvoke<ProcessStatus>("start_gateway");
    }
    return {
      running: true,
      pid: 1234,
      bind: "127.0.0.1:8989",
      is_healthy: true,
      ws_connected: false,
    };
  },

  async stopGateway(): Promise<boolean> {
    if (isTauri()) {
      return await tauriInvoke<boolean>("stop_gateway");
    }
    return true;
  },

  // Gateway Proxy API
  async gatewayRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
    if (isTauri()) {
      return await tauriInvoke<T>("proxy_gateway_api", {
        method,
        path,
        body,
      });
    }

    const res = await fetch(`http://localhost:8989${path}`, {
      method,
      headers: {
        "Content-Type": "application/json",
        "X-Gateway-Key": "dev-gateway-key-2024",
      },
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    return await res.json();
  },

  // Session & WA actions
  async pair(numberId: string): Promise<{ status: string; number_id: string }> {
    return await this.gatewayRequest("POST", "/api/v1/pair", { number_id: numberId });
  },

  async pairQR(): Promise<{ session_id: string }> {
    return await this.gatewayRequest("POST", "/api/v1/pair-qr");
  },

  async getQR(numberId: string): Promise<{ qr: string; number_id: string }> {
    return await this.gatewayRequest("GET", `/api/v1/qr/${encodeURIComponent(numberId)}`);
  },

  async getSessionStatus(numberId: string): Promise<{ number_id: string; connected: boolean }> {
    return await this.gatewayRequest("GET", `/api/v1/status/${encodeURIComponent(numberId)}`);
  },

  async listSessions(): Promise<{ sessions: SessionInfo[] }> {
    return await this.gatewayRequest("GET", "/api/v1/sessions");
  },

  async sendText(numberId: string, to: string, text: string): Promise<{ status: string }> {
    return await this.gatewayRequest("POST", "/api/v1/send", {
      number_id: numberId,
      to,
      text,
    });
  },

  async sendGroup(numberId: string, groupJid: string, text: string): Promise<{ status: string }> {
    return await this.gatewayRequest("POST", "/api/v1/send-group", {
      number_id: numberId,
      group_jid: groupJid,
      text,
    });
  },

  async getGroups(numberId: string): Promise<{ groups: GroupInfo[] }> {
    return await this.gatewayRequest("GET", `/api/v1/groups/${encodeURIComponent(numberId)}`);
  },

  async getContacts(numberId: string): Promise<{ contacts: ContactInfo[] }> {
    return await this.gatewayRequest("GET", `/api/v1/contacts/${encodeURIComponent(numberId)}`);
  },

  async getAllContacts(): Promise<{ contacts: ContactInfo[]; total: number }> {
    return await this.gatewayRequest("GET", "/api/v1/contacts");
  },

  async getAllGroups(): Promise<{ groups: GroupInfo[]; total: number }> {
    return await this.gatewayRequest("GET", "/api/v1/groups");
  },

  async syncAll(): Promise<{ status: string; message: string; data: AllSyncResult }> {
    return await this.gatewayRequest("POST", "/api/v1/sync/all");
  },

  async syncDevice(numberId: string): Promise<{ status: string; message: string; data: SessionSyncData }> {
    return await this.gatewayRequest("POST", `/api/v1/sync/${encodeURIComponent(numberId)}`);
  },

  async logout(numberId: string, keepSession: boolean = false): Promise<{ status: string }> {
    return await this.gatewayRequest("POST", `/api/v1/logout/${encodeURIComponent(numberId)}`, {
      keep_session: keepSession,
    });
  },

  // Windows service mode
  async serviceStatus(): Promise<ServiceInfo> {
    if (isTauri()) {
      return await tauriInvoke<ServiceInfo>("service_status");
    }
    return { ok: true, message: "", installed: false, state: "not_installed", data_dir: "", log_file: "" };
  },

  async serviceInstall(): Promise<ServiceInfo> {
    if (isTauri()) {
      return await tauriInvoke<ServiceInfo>("service_install");
    }
    return { ok: true, message: "Mock: service terpasang", installed: true, state: "running", data_dir: "C:\ProgramData\Febran\Gateway\data", log_file: "C:\ProgramData\Febran\Gateway\gateway.log" };
  },

  async serviceUninstall(purge: boolean): Promise<ServiceInfo> {
    if (isTauri()) {
      return await tauriInvoke<ServiceInfo>("service_uninstall", { purge });
    }
    return { ok: true, message: "Mock: service dihapus", installed: false, state: "not_installed", data_dir: "", log_file: "" };
  },

  async serviceControl(action: "start" | "stop" | "restart"): Promise<ServiceInfo> {
    if (isTauri()) {
      return await tauriInvoke<ServiceInfo>("service_control", { action });
    }
    return { ok: true, message: "", installed: false, state: "not_installed", data_dir: "", log_file: "" };
  },

  async serviceLogTail(lines: number): Promise<string[]> {
    if (isTauri()) {
      return await tauriInvoke<string[]>("service_log_tail", { lines });
    }
    return [];
  },

  // Log Event Listener
  async onGatewayLog(callback: (log: LogMessage) => void): Promise<() => void> {
    if (isTauri()) {
      try {
        const { listen } = await import("@tauri-apps/api/event");
        return await listen<LogMessage>("gateway-log", (event) => {
          callback(event.payload);
        });
      } catch (err) {
        console.warn("Failed to attach log listener:", err);
      }
    }
    return () => {};
  },
};

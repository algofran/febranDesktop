<template>
  <div class="app-root">
    <!-- Top Navigation Bar -->
    <Navbar 
      :current-tab="currentTab"
      :status="status"
      :loading="loading"
      :sessions-count="sessions.length"
      @select-tab="currentTab = $event"
      @start-gateway="startGateway"
      @stop-gateway="stopGateway"
      @restart-gateway="restartGateway"
    />

    <!-- Main View Router Container -->
    <main class="view-container">
      <DashboardView 
        v-if="currentTab === 'dashboard'"
        :status="status"
        :config="config"
        :sessions="sessions"
        :loading="loading"
        @start-gateway="startGateway"
        @stop-gateway="stopGateway"
        @restart-gateway="restartGateway"
        @open-settings="currentTab = 'settings'"
        @open-pairing="currentTab = 'sessions'"
        @open-test="currentTab = 'sandbox'"
        @refresh-sessions="fetchSessions"
        @reconnect-device="openPairWithNumber"
        @logout-device="handleLogout"
        @sync-all="handleSyncAll"
        @sync-device="handleSyncDevice"
      />

      <SessionsView 
        v-else-if="currentTab === 'sessions'"
        :sessions="sessions"
        :loading="loading"
        @refresh-sessions="fetchSessions"
        @toast="showToast"
      />

      <MessageSandbox 
        v-else-if="currentTab === 'sandbox'"
        :sessions="sessions"
        @toast="showToast"
      />

      <LogsView 
        v-else-if="currentTab === 'logs'"
        :logs="logs"
        @clear-logs="logs = []"
        @toast="showToast"
      />

      <SettingsView 
        v-else-if="currentTab === 'settings'"
        :config="config"
        @save-config="saveConfig"
        @toast="showToast"
      />
    </main>

    <!-- Global Toast Notification Banner -->
    <transition name="fade">
      <div v-if="toast.visible" :class="['toast-notification', toast.type]">
        <span>{{ toast.message }}</span>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import Navbar from "./components/Navbar.vue";
import DashboardView from "./components/DashboardView.vue";
import SessionsView from "./components/SessionsView.vue";
import MessageSandbox from "./components/MessageSandbox.vue";
import LogsView from "./components/LogsView.vue";
import SettingsView from "./components/SettingsView.vue";
import { api } from "./api";
import type { ProcessStatus, GatewayConfig, SessionInfo, LogMessage } from "./types";

const currentTab = ref("dashboard");
const loading = ref(false);

const status = ref<ProcessStatus>({
  running: false,
  pid: null,
  bind: "127.0.0.1:8989",
  is_healthy: false,
  ws_connected: false,
});

const config = ref<GatewayConfig>({
  gateway_bind: "127.0.0.1:8989",
  gateway_api_key: "dev-gateway-key-2024",
  central_webhook_url: "http://localhost:8080/api/v1/webhooks/gateway",
  central_ws_url: "ws://localhost:8080/ws/gateway",
  gateway_server_key: "node-1",
  central_api_key: "dev-central-key-2024",
  session_dir: "./sessions",
  log_level: "info",
});

const sessions = ref<SessionInfo[]>([]);
const logs = ref<LogMessage[]>([]);

const toast = ref<{ visible: boolean; message: string; type: "success" | "error" }>({
  visible: false,
  message: "",
  type: "success",
});

let statusPollInterval: ReturnType<typeof setInterval> | null = null;
let unlistenLogs: (() => void) | null = null;

const showToast = (message: string, type: "success" | "error" = "success") => {
  toast.value = { visible: true, message, type };
  setTimeout(() => {
    toast.value.visible = false;
  }, 3500);
};

const refreshStatus = async () => {
  try {
    const s = await api.getStatus();
    status.value = s;
    if (s.running) {
      fetchSessions();
    }
  } catch (err) {
    console.warn("Status check failed:", err);
  }
};

const startGateway = async () => {
  loading.value = true;
  try {
    const res = await api.startGateway();
    status.value = res;
    showToast("Gateway process started successfully", "success");
    addLog("info", `Gateway daemon started on ${res.bind}`);
    setTimeout(fetchSessions, 1000);
  } catch (err: unknown) {
    showToast(`Failed to start gateway: ${err instanceof Error ? err.message : String(err)}`, "error");
    addLog("error", `Start error: ${err}`);
  } finally {
    loading.value = false;
  }
};

const stopGateway = async () => {
  loading.value = true;
  try {
    await api.stopGateway();
    status.value = {
      running: false,
      pid: null,
      bind: config.value.gateway_bind,
      is_healthy: false,
      ws_connected: false,
    };
    showToast("Gateway process stopped", "success");
    addLog("warn", "Gateway daemon stopped by user");
  } catch (err: unknown) {
    showToast(`Failed to stop: ${err instanceof Error ? err.message : String(err)}`, "error");
  } finally {
    loading.value = false;
  }
};

const restartGateway = async () => {
  loading.value = true;
  showToast("Restarting Go daemon...", "success");
  addLog("info", "Restarting Go daemon...");
  try {
    await api.stopGateway();
    setTimeout(async () => {
      try {
        const res = await api.startGateway();
        status.value = res;
        showToast("Gateway restarted successfully", "success");
        addLog("info", `Gateway daemon started on ${res.bind}`);
        setTimeout(fetchSessions, 1000);
      } catch (err) {
        showToast(`Failed to start: ${err instanceof Error ? err.message : String(err)}`, "error");
      } finally {
        loading.value = false;
      }
    }, 600);
  } catch (err) {
    showToast(`Restart error: ${err instanceof Error ? err.message : String(err)}`, "error");
    loading.value = false;
  }
};

const fetchSessions = async () => {
  if (!status.value.running) return;
  try {
    const res = await api.listSessions();
    sessions.value = res.sessions;
  } catch {
    // List endpoint unavailable — keep whatever we already have.
  }

  for (const s of sessions.value) {
    try {
      const res = await api.getSessionStatus(s.number_id);
      s.connected = res.connected;
    } catch {
      s.connected = false;
    }
  }
};

const openPairWithNumber = (numberId: string) => {
  currentTab.value = "sessions";
};

const handleLogout = async (numberId: string) => {
  try {
    await api.logout(numberId, false);
    showToast(`Logged out session for ${numberId}`);
    fetchSessions();
  } catch (err: unknown) {
    showToast(`Error: ${err instanceof Error ? err.message : String(err)}`, "error");
  }
};

const handleSyncAll = async () => {
  try {
    showToast("Menarik & menyinkronkan semua data kontak dan grup...", "success");
    const res = await api.syncAll();
    showToast(`Berhasil sinkronkan ${res.data.total_contacts} kontak dan ${res.data.total_groups} grup dari ${res.data.total_sessions} nomor`, "success");
    addLog("info", `[SyncAll] Berhasil sinkronkan ${res.data.total_contacts} kontak dan ${res.data.total_groups} grup dari ${res.data.total_sessions} nomor terhubung`);
    fetchSessions();
  } catch (err: unknown) {
    showToast(`Sinkronisasi gagal: ${err instanceof Error ? err.message : String(err)}`, "error");
    addLog("error", `[SyncAll] Gagal: ${err}`);
  }
};

const handleSyncDevice = async (numberId: string) => {
  try {
    showToast(`Menyinkronkan kontak dan grup untuk ${numberId}...`, "success");
    const res = await api.syncDevice(numberId);
    showToast(`Berhasil sinkronkan ${res.data.contacts.length} kontak & ${res.data.groups.length} grup untuk ${numberId}`, "success");
    addLog("info", `[SyncDevice] Berhasil sinkronkan ${res.data.contacts.length} kontak dan ${res.data.groups.length} grup untuk ${numberId}`);
    fetchSessions();
  } catch (err: unknown) {
    showToast(`Sinkronisasi gagal: ${err instanceof Error ? err.message : String(err)}`, "error");
    addLog("error", `[SyncDevice] Gagal untuk ${numberId}: ${err}`);
  }
};

const saveConfig = async (newCfg: GatewayConfig) => {
  try {
    await api.saveConfig(newCfg);
    config.value = newCfg;
    showToast("Configuration saved successfully", "success");
    addLog("info", "Configuration saved. Restarting daemon to apply changes...");
    if (status.value.running) {
      await stopGateway();
      setTimeout(startGateway, 500);
    }
  } catch (err: unknown) {
    showToast(`Failed to save config: ${err instanceof Error ? err.message : String(err)}`, "error");
  }
};

const addLog = (level: string, message: string) => {
  const now = new Date().toLocaleTimeString();
  logs.value.push({ level, message, timestamp: now });
  if (logs.value.length > 500) {
    logs.value.shift();
  }
};

onMounted(async () => {
  try {
    config.value = await api.getConfig();
  } catch (err) {
    console.warn("Failed to load initial config:", err);
  }

  await refreshStatus();

  // Attach log listener from Tauri backend
  unlistenLogs = await api.onGatewayLog((logMsg) => {
    logs.value.push(logMsg);
    if (logs.value.length > 500) {
      logs.value.shift();
    }
  });

  statusPollInterval = setInterval(refreshStatus, 4000);
});

onUnmounted(() => {
  if (statusPollInterval) clearInterval(statusPollInterval);
  if (unlistenLogs) unlistenLogs();
});
</script>

<style scoped>
.app-root {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: var(--bg-main);
  overflow: hidden;
}

.view-container {
  flex: 1;
  overflow: hidden;
}

.toast-notification {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  padding: 0.75rem 1.25rem;
  border-radius: var(--radius-md);
  font-size: 0.85rem;
  font-weight: 600;
  box-shadow: var(--shadow-lg);
  z-index: 9999;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.toast-notification.success {
  background: #064e3b;
  color: #34d399;
  border: 1px solid #059669;
}

.toast-notification.error {
  background: #7f1d1d;
  color: #fca5a5;
  border: 1px solid #b91c1c;
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>

<template>
  <div class="settings-view">
    <div class="glass-panel settings-card">
      <div class="settings-header">
        <div>
          <h2 class="settings-title">Node Configuration</h2>
          <p class="settings-subtitle">Manage environment parameters and transport security for this daemon</p>
        </div>
        <button class="btn-primary" :disabled="saving" @click="save">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
            <polyline points="17 21 17 13 7 13 7 21"/>
            <polyline points="7 3 7 8 15 8"/>
          </svg>
          <span>{{ saving ? 'Saving...' : 'Save & Apply Config' }}</span>
        </button>
      </div>

      <div class="settings-grid">
        <!-- Gateway Section -->
        <div class="section-group">
          <h3 class="group-title">Gateway Transport Parameters</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Gateway Bind Address (GATEWAY_BIND)</label>
              <input v-model="form.gateway_bind" type="text" class="font-mono" />
              <span class="form-hint">Address and port the gateway HTTP server listens on (e.g. 127.0.0.1:8989 or 0.0.0.0:8989).</span>
            </div>

            <div class="form-group">
              <label class="form-label">Gateway Security Key (GATEWAY_API_KEY)</label>
              <div class="input-with-action">
                <input v-model="form.gateway_api_key" type="text" class="font-mono" />
                <button class="btn-secondary btn-sm" @click="generateKey">Generate</button>
              </div>
              <span class="form-hint">Required in the <code>X-Gateway-Key</code> request header when accessing this node.</span>
            </div>
          </div>
        </div>

        <!-- Central Webhook Section -->
        <div class="section-group">
          <h3 class="group-title">Central Backend Connection</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Central Webhook URL (CENTRAL_WEBHOOK_URL)</label>
              <input v-model="form.central_webhook_url" type="text" class="font-mono" />
              <span class="form-hint">Destination endpoint where inbound messages and status changes are posted.</span>
            </div>

            <div class="form-group">
              <label class="form-label">Central WebSocket URL (CENTRAL_WS_URL)</label>
              <input v-model="form.central_ws_url" type="text" class="font-mono" />
              <span class="form-hint">WebSocket address of the tenant backend this node registers to (e.g. ws://localhost:8080/ws/gateway).</span>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Gateway Server Key (GATEWAY_SERVER_KEY)</label>
              <div class="input-with-action">
                <input v-model="form.gateway_server_key" type="text" class="font-mono" />
                <button class="btn-secondary btn-sm" @click="generateServerKey">Generate</button>
              </div>
              <span class="form-hint">Unique identifier for this node, matching the <code>server_key</code> stored on the tenant number. Must differ from every other server — the backend rejects a duplicate and pairing only reaches the node that owns this key.</span>
            </div>

            <div class="form-group">
              <label class="form-label">Central API Key (CENTRAL_API_KEY)</label>
              <input v-model="form.central_api_key" type="text" class="font-mono" />
              <span class="form-hint">Shared key used to authenticate with the central backend WebSocket.</span>
            </div>
          </div>
        </div>

        <!-- Storage & System -->
        <div class="section-group">
          <h3 class="group-title">Storage & Logging</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Session Directory (SESSION_DIR)</label>
              <input v-model="form.session_dir" type="text" class="font-mono" />
              <span class="form-hint">Local filesystem folder where SQLite session databases (*.db) are stored.</span>
            </div>

            <div class="form-group">
              <label class="form-label">Log Level (LOG_LEVEL)</label>
              <select v-model="form.log_level">
                <option value="debug">Debug (Verbose)</option>
                <option value="info">Info (Standard)</option>
                <option value="warn">Warn (Warnings only)</option>
                <option value="error">Error (Errors only)</option>
              </select>
              <span class="form-hint">Verbosity level of the Go daemon log output.</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ServicePanel :dirty="dirty" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from "vue";
import ServicePanel from "./ServicePanel.vue";
import type { GatewayConfig } from "../types";

const props = defineProps<{
  config: GatewayConfig;
}>();

const emit = defineEmits<{
  (e: "save-config", config: GatewayConfig): void;
  (e: "toast", msg: string, type?: "success" | "error"): void;
}>();

const form = ref<GatewayConfig>({ ...props.config });
const saving = ref(false);
const dirty = computed(() => JSON.stringify(form.value) !== JSON.stringify(props.config));

watch(
  () => props.config,
  (newVal) => {
    form.value = { ...newVal };
  },
  { deep: true }
);

const generateKey = () => {
  const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  let key = "gw_";
  for (let i = 0; i < 32; i++) {
    key += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  form.value.gateway_api_key = key;
  emit("toast", "Generated new random API key");
};

const generateServerKey = () => {
  const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  let key = "node-";
  for (let i = 0; i < 16; i++) {
    key += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  form.value.gateway_server_key = key;
  emit("toast", "Generated new unique server key");
};

const save = () => {
  saving.value = true;
  emit("save-config", { ...form.value });
  setTimeout(() => {
    saving.value = false;
  }, 400);
};
</script>

<style scoped>
.settings-view {
  padding: 1.5rem;
  overflow-y: auto;
  height: calc(100vh - 65px);
}

.settings-card {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  max-width: 960px;
  margin: 0 auto;
}

.settings-view > * + * {
  margin-top: 1.5rem;
}

.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 1rem;
}

.settings-title {
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
}

.settings-subtitle {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.settings-grid {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.section-group {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.group-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.form-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
}

.form-hint {
  font-size: 0.72rem;
  color: var(--text-muted);
  line-height: 1.4;
}

.form-hint code {
  background: var(--bg-surface-elevated);
  padding: 0.1rem 0.3rem;
  border-radius: var(--radius-sm);
  color: #34d399;
}

.input-with-action {
  display: flex;
  gap: 0.5rem;
}
</style>

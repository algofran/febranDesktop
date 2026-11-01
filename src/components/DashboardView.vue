<template>
  <div class="dashboard-view">
    <!-- Hero Daemon Controller Card -->
    <div :class="['glass-panel hero-controller', status.running ? 'online' : 'offline']">
      <div class="controller-main">
        <div class="status-header">
          <div class="status-indicator-badge">
            <span :class="['status-dot-large', status.running ? 'active' : 'inactive', { loading }]"></span>
            <span class="status-title">{{ loading ? 'STARTING GO DAEMON...' : (status.running ? 'GO DAEMON IS RUNNING' : 'GO DAEMON IS STOPPED') }}</span>
          </div>
          <span v-if="status.running" class="badge badge-success">HTTP 200 OK</span>
          <span v-else class="badge badge-danger">OFFLINE</span>
        </div>

        <h1 class="controller-title">WhatsApp Session Transport Node</h1>
        <p class="controller-desc">
          Pure-Go daemon running WhatsApp multi-device engine (<code>whatsmeow</code>). Handles local SQLite sessions, manages QR code pairing, dispatches outbound messages, and relays inbound events to your central backend.
        </p>

        <!-- Live Daemon Metadata Pills -->
        <div class="daemon-meta-bar font-mono">
          <div class="meta-pill">
            <span class="meta-label">Local Host:</span>
            <span class="meta-val">{{ status.bind }}</span>
          </div>
          <div class="meta-pill">
            <span class="meta-label">Process PID:</span>
            <span class="meta-val">{{ status.pid || 'Inactive' }}</span>
          </div>
          <div class="meta-pill">
            <span class="meta-label">Storage:</span>
            <span class="meta-val">{{ config.session_dir }}</span>
          </div>
          <div class="meta-pill">
            <span class="meta-label">Webhook:</span>
            <span class="meta-val truncate-pill">{{ config.central_webhook_url }}</span>
          </div>
        </div>
      </div>

      <!-- Primary Action Buttons Box -->
      <div class="controller-actions">
        <div v-if="svc.installed" class="service-note">
          <span>Berjalan sebagai service Windows — kelola di Pengaturan › Mode Service</span>
          <button class="btn-secondary btn-sm" @click="$emit('open-settings')">Buka Pengaturan</button>
        </div>
        <button 
          v-if="!status.running" 
          class="btn-primary-large start-daemon-btn" 
          :disabled="loading || svc.installed"
          @click="$emit('start-gateway')"
        >
          <svg v-if="loading" class="spin-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
          </svg>
          <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
            <polygon points="5 3 19 12 5 21 5 3"/>
          </svg>
          <div class="btn-text-block">
            <span class="btn-main-label">{{ loading ? 'Starting...' : 'Start Go Daemon' }}</span>
            <span class="btn-sub-label">Launch background service</span>
          </div>
        </button>

        <div v-else class="running-actions-box">
          <button 
            class="btn-secondary restart-daemon-btn" 
            :disabled="loading || svc.installed" 
            @click="$emit('restart-gateway')"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="23 4 23 10 17 10"/>
              <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
            </svg>
            <span>Restart Daemon</span>
          </button>

          <button 
            class="btn-danger stop-daemon-btn" 
            :disabled="loading || svc.installed" 
            @click="$emit('stop-gateway')"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <rect x="6" y="6" width="12" height="12" rx="2"/>
            </svg>
            <span>Stop Daemon</span>
          </button>
        </div>

        <div class="quick-nav-actions">
          <button class="btn-secondary btn-sm" @click="$emit('open-pairing')">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="5" y="2" width="14" height="20" rx="2"/>
              <line x1="12" y1="18" x2="12.01" y2="18"/>
            </svg>
            <span>Scan QR Code</span>
          </button>
          <button class="btn-secondary btn-sm" @click="$emit('open-test')">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="22" y1="2" x2="11" y2="13"/>
              <polygon points="22 2 15 22 11 13 2 9 22 2"/>
            </svg>
            <span>Test Message</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Statistics Cards -->
    <div class="stats-grid">
      <div class="glass-panel stat-card">
        <div class="stat-header">
          <span class="stat-label">Daemon State</span>
          <span :class="['badge', status.running ? 'badge-success' : 'badge-danger']">
            {{ status.running ? 'ONLINE' : 'STOPPED' }}
          </span>
        </div>
        <div class="stat-value">{{ status.running ? 'Active & Listening' : 'Inactive' }}</div>
        <div class="stat-sub">
          <span class="stat-mono">Port :{{ status.bind.split(':')[1] || '8989' }}</span>
          <span v-if="status.pid" class="stat-mono">• PID {{ status.pid }}</span>
        </div>
      </div>

      <div class="glass-panel stat-card">
        <div class="stat-header">
          <span class="stat-label">Active WA Sessions</span>
          <span class="badge badge-success">{{ connectedSessionsCount }} Connected</span>
        </div>
        <div class="stat-value">{{ totalSessionsCount }}</div>
        <div class="stat-sub">SQLite multi-device files</div>
      </div>

      <div class="glass-panel stat-card">
        <div class="stat-header">
          <span class="stat-label">Central Webhook</span>
          <span :class="['badge', webhookConfigured ? 'badge-success' : 'badge-warning']">
            {{ webhookConfigured ? 'CONNECTED' : 'NOT SET' }}
          </span>
        </div>
        <div class="stat-value truncate">{{ truncateUrl(config.central_webhook_url) }}</div>
        <div class="stat-sub">Inbound & status dispatcher</div>
      </div>

      <div class="glass-panel stat-card">
        <div class="stat-header">
          <span class="stat-label">Transport Driver</span>
          <span class="badge badge-neutral">pure-Go SQLite</span>
        </div>
        <div class="stat-value stat-mono">{{ config.session_dir }}</div>
        <div class="stat-sub">Zero CGO / pure Go binary</div>
      </div>
    </div>

    <!-- Connected Devices & Architecture Grid -->
    <div class="main-sections-grid">
      <!-- Active Sessions list -->
      <div class="glass-panel section-card">
        <div class="section-header">
          <div class="section-title-box">
            <h3 class="section-title">Connected WhatsApp Devices</h3>
            <span class="badge badge-neutral">{{ sessions.length }} Total</span>
          </div>
          <div class="section-header-actions">
            <button 
              class="btn-secondary btn-sm" 
              :disabled="loading || !hasConnectedSessions" 
              @click="$emit('sync-all')"
              title="Tarik dan sinkronkan semua kontak dan grup ke backend pusat"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
              </svg>
              <span>Sync Semua Data</span>
            </button>
            <button class="btn-secondary btn-sm" @click="$emit('refresh-sessions')">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23 4 23 10 17 10"/>
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
              </svg>
              <span>Refresh</span>
            </button>
          </div>
        </div>

        <div v-if="sessions.length === 0" class="empty-state">
          <div class="empty-icon">📱</div>
          <div class="empty-text">No WhatsApp sessions registered yet.</div>
          <button class="btn-primary btn-sm" @click="$emit('open-pairing')">Pair First Device</button>
        </div>

        <div v-else class="devices-table">
          <div v-for="session in sessions" :key="session.number_id" class="device-row">
            <div class="device-info">
              <div class="device-avatar">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#E15A32" stroke-width="2">
                  <rect x="5" y="2" width="14" height="20" rx="2"/>
                  <line x1="12" y1="18" x2="12.01" y2="18"/>
                </svg>
              </div>
              <div>
                <div class="device-number font-mono">{{ session.number_id }}</div>
                <div class="device-meta">Storage: {{ session.number_id }}.db</div>
                <div v-if="session.pairing_key" class="pairing-key-row">
                  <span class="pairing-key-label">Pairing Key:</span>
                  <code class="pairing-key-code">{{ session.pairing_key }}</code>
                  <button
                    class="pairing-key-copy"
                    title="Salin Kode Pairing"
                    @click="copyKey(session.pairing_key)"
                  >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                  <span v-if="copiedKey === session.number_id" class="pairing-key-copied">✓ Disalin</span>
                </div>
              </div>
            </div>

            <div class="device-status">
              <span :class="['badge', session.connected ? 'badge-success' : 'badge-warning']">
                {{ session.connected ? 'Connected' : 'Disconnected' }}
              </span>
            </div>

            <div class="device-actions">
              <button 
                v-if="session.connected" 
                class="btn-secondary btn-sm" 
                @click="$emit('sync-device', session.number_id)"
                title="Tarik kontak & grup dan sinkronkan ke backend pusat"
              >
                Sync Data
              </button>
              <button 
                v-if="!session.connected" 
                class="btn-primary btn-sm" 
                @click="$emit('reconnect-device', session.number_id)"
              >
                Scan QR
              </button>
              <button 
                class="btn-danger btn-sm" 
                @click="$emit('logout-device', session.number_id)"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Quick Architecture Info -->
      <div class="glass-panel section-card">
        <div class="section-header">
          <h3 class="section-title">Transport Architecture</h3>
          <span class="badge badge-success">HTTP & Webhook</span>
        </div>

        <div class="arch-flow">
          <div class="arch-node">
            <div class="arch-node-title">febranBackend</div>
            <div class="arch-node-desc">Central API</div>
          </div>
          <div class="arch-arrow">
            <span>⇄</span>
            <div class="arch-label">Commands / Webhook</div>
          </div>
          <div class="arch-node current">
            <div class="arch-node-title">febranDesktop</div>
            <div class="arch-node-desc">Go Daemon :8989</div>
          </div>
          <div class="arch-arrow">
            <span>⇄</span>
            <div class="arch-label">Noise Proto</div>
          </div>
          <div class="arch-node">
            <div class="arch-node-title">WhatsApp</div>
            <div class="arch-node-desc">Web Servers</div>
          </div>
        </div>

        <div class="arch-details">
          <div class="arch-item">
            <span class="arch-key">Command API:</span>
            <span class="arch-val font-mono">POST /api/v1/send, /pair, /groups</span>
          </div>
          <div class="arch-item">
            <span class="arch-key">Inbound Stream:</span>
            <span class="arch-val font-mono">POST {{ config.central_webhook_url }}</span>
          </div>
          <div class="arch-item">
            <span class="arch-key">Security:</span>
            <span class="arch-val">X-Gateway-Key Header Auth</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useServiceState } from "../useServiceState";

const { svc, refreshServiceState } = useServiceState();
onMounted(refreshServiceState);
import type { ProcessStatus, GatewayConfig, SessionInfo } from "../types";

const copiedKey = ref<string | null>(null);
const copyKey = async (key?: string) => {
  if (!key) return;
  await navigator.clipboard.writeText(key);
  copiedKey.value = key;
  setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = null;
  }, 1500);
};

const props = defineProps<{
  status: ProcessStatus;
  config: GatewayConfig;
  sessions: SessionInfo[];
  loading?: boolean;
}>();

defineEmits<{
  (e: "start-gateway"): void;
  (e: "stop-gateway"): void;
  (e: "restart-gateway"): void;
  (e: "open-settings"): void;
  (e: "open-pairing"): void;
  (e: "open-test"): void;
  (e: "refresh-sessions"): void;
  (e: "reconnect-device", numberId: string): void;
  (e: "logout-device", numberId: string): void;
  (e: "sync-all"): void;
  (e: "sync-device", numberId: string): void;
}>();

const hasConnectedSessions = computed(() => {
  return props.sessions.some((s) => s.connected);
});

const connectedSessionsCount = computed(() => {
  return props.sessions.filter((s) => s.connected).length;
});

const totalSessionsCount = computed(() => {
  return props.sessions.length;
});

const webhookConfigured = computed(() => {
  return !!props.config.central_webhook_url && props.config.central_webhook_url.startsWith("http");
});

const truncateUrl = (url: string) => {
  if (!url) return "Not configured";
  if (url.length > 32) return url.substring(0, 30) + "...";
  return url;
};
</script>

<style scoped>
.dashboard-view {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
  overflow-y: auto;
  height: 100%;
  min-height: 0;
}

.hero-controller {
  display: grid;
  grid-template-columns: 1.35fr 0.85fr;
  gap: 1.5rem;
  padding: 1.75rem;
  border-radius: var(--radius-lg);
  position: relative;
  transition: all 0.2s ease;
}

.hero-controller.online {
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.95) 0%, rgba(16, 185, 129, 0.12) 100%);
  border: 1px solid rgba(16, 185, 129, 0.3);
  box-shadow: 0 0 30px rgba(16, 185, 129, 0.08);
}

.hero-controller.offline {
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.95) 0%, rgba(239, 68, 68, 0.08) 100%);
  border: 1px solid rgba(239, 68, 68, 0.25);
}

.controller-main {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.status-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
}

.status-indicator-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.hero-controller.online .status-title {
  color: #34d399;
}
.hero-controller.offline .status-title {
  color: #f87171;
}

.status-dot-large {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  position: relative;
}
.status-dot-large.active {
  background-color: #10b981;
  box-shadow: 0 0 12px #10b981;
}
.status-dot-large.inactive {
  background-color: #ef4444;
}
.status-dot-large.loading {
  background-color: #f59e0b;
  animation: pulse 1s infinite;
}

.controller-title {
  font-size: 1.45rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.controller-desc {
  font-size: 0.85rem;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-bottom: 0.5rem;
}

.controller-desc code {
  background: var(--bg-surface-elevated);
  padding: 0.15rem 0.35rem;
  border-radius: var(--radius-sm);
  color: #FDB000;
}

.daemon-meta-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.65rem;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
}

.meta-label {
  color: var(--text-muted);
}
.meta-val {
  color: var(--text-primary);
  font-weight: 600;
}
.truncate-pill {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Controller Actions Column */
.controller-actions {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.75rem;
  border-left: 1px solid var(--border-subtle);
  padding-left: 1.5rem;
}

.btn-primary-large {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.9rem 1.25rem;
  background: #E15A32;
  color: #022c1b;
  border-radius: var(--radius-md);
  font-weight: 700;
  border: none;
  cursor: pointer;
  transition: all 0.15s ease;
  box-shadow: 0 0 25px rgba(225, 90, 50, 0.35);
}
.btn-primary-large:hover {
  background: #C94A25;
  box-shadow: 0 0 30px rgba(225, 90, 50, 0.5);
  transform: translateY(-1px);
}

.btn-text-block {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  line-height: 1.2;
}

.btn-main-label {
  font-size: 1rem;
  font-weight: 700;
}

.btn-sub-label {
  font-size: 0.7rem;
  opacity: 0.85;
  font-weight: 500;
}

.running-actions-box {
  display: flex;
  gap: 0.5rem;
}

.running-actions-box button {
  flex: 1;
  padding: 0.65rem;
  font-size: 0.85rem;
  font-weight: 600;
}

.quick-nav-actions {
  display: flex;
  gap: 0.5rem;
}

.quick-nav-actions button {
  flex: 1;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.stat-card {
  padding: 1.1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.stat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.stat-value {
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
}

.stat-value.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-sub {
  font-size: 0.75rem;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.stat-mono {
  font-family: var(--font-mono);
}

.main-sections-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1rem;
}

.section-card {
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.section-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem 1rem;
}

.section-title-box {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.section-title-box .badge {
  flex-shrink: 0;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.section-header-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
}

.btn-sm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0.3rem 0.65rem;
  font-size: 0.75rem;
  white-space: nowrap;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1rem;
  gap: 0.75rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px dashed var(--border-subtle);
}

.empty-icon {
  font-size: 2rem;
}

.empty-text {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.devices-table {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.device-row {
  /* info and status on the first line, actions on their own line below */
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  grid-template-areas: 'info status' 'actions actions';
  align-items: center;
  gap: 0.7rem 0.85rem;
  padding: 0.8rem 0.9rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.device-info {
  grid-area: info;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.device-info > div:last-child {
  min-width: 0;
}

.device-status {
  grid-area: status;
}

.device-avatar {
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: rgba(225, 90, 50, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
}

.device-number {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-primary);
}

.device-meta {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.pairing-key-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.4rem;
  margin-top: 0.4rem;
}

.pairing-key-label {
  white-space: nowrap;
  font-size: 0.68rem;
  color: var(--text-muted);
}

.pairing-key-code {
  font-family: var(--font-mono);
  white-space: nowrap;
  font-size: 0.7rem;
  font-weight: 700;
  color: #FDB000;
  background: #1e293b;
  padding: 0.18rem 0.4rem;
  border-radius: 4px;
  border: 1px solid #334155;
  user-select: all;
}

.pairing-key-copy {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.18rem;
  border-radius: 4px;
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.pairing-key-copy:hover {
  background: var(--bg-surface-elevated);
  color: var(--text-primary);
}

.pairing-key-copied {
  font-size: 0.65rem;
  font-weight: 600;
  color: #10b981;
}

.device-actions {
  grid-area: actions;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  padding-top: 0.7rem;
  border-top: 1px solid var(--border-subtle);
}

/* Architecture Flow */
.arch-flow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 0.75rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.arch-node {
  padding: 0.5rem 0.65rem;
  background: var(--bg-surface-elevated);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  text-align: center;
}

.arch-node.current {
  border-color: #E15A32;
  background: rgba(225, 90, 50, 0.1);
}

.arch-node-title {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--text-primary);
}

.arch-node-desc {
  font-size: 0.65rem;
  color: var(--text-muted);
}

.arch-arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  font-size: 0.9rem;
  color: var(--accent-wa);
}

.arch-label {
  font-size: 0.6rem;
  color: var(--text-muted);
}

.arch-details {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.arch-item {
  display: flex;
  justify-content: space-between;
  font-size: 0.8rem;
  padding: 0.4rem 0.5rem;
  background: var(--bg-input);
  border-radius: var(--radius-sm);
}

.arch-key {
  color: var(--text-muted);
}

.arch-val {
  color: var(--text-primary);
  font-weight: 500;
  text-align: right;
  word-break: break-all;
  max-width: 70%;
}

.service-note {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
  font-size: 0.78rem;
  color: #fbbf24;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.35);
  border-radius: var(--radius-md);
  padding: 0.5rem 0.75rem;
}

.spin-icon {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Narrow windows: stack hero columns, drop the actions divider */
@media (max-width: 820px) {
  .hero-controller {
    grid-template-columns: 1fr;
  }
  .controller-actions {
    border-left: none;
    padding-left: 0;
    padding-top: 1rem;
    border-top: 1px solid var(--border-subtle);
  }
}
</style>

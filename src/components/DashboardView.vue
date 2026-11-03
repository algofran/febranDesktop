<template>
  <div class="dashboard-view">
    <!-- Status: one line, one main action -->
    <div class="glass-panel status-card">
      <div class="status-text">
        <span :class="['status-dot', status.running ? 'active' : 'inactive', { loading }]"></span>
        <div>
          <h1 class="status-title">{{ loading ? t('dashboard.status.starting') : (status.running ? t('dashboard.status.running') : t('dashboard.status.stopped')) }}</h1>
          <p class="status-sub">{{ status.running ? t('dashboard.status.runningSub') : t('dashboard.status.stoppedSub') }}</p>
        </div>
      </div>

      <div class="status-actions">
        <template v-if="svc.installed">
          <span class="service-note">{{ t('dashboard.serviceNote') }}</span>
          <button class="btn-secondary btn-sm" @click="$emit('open-settings')">{{ t('dashboard.openSettings') }}</button>
        </template>
        <template v-else-if="!status.running">
          <button class="btn-primary main-btn" :disabled="loading" @click="$emit('start-gateway')">
            <svg v-if="loading" class="spin-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            <span>{{ loading ? t('dashboard.btn.starting') : t('dashboard.btn.start') }}</span>
          </button>
        </template>
        <template v-else>
          <button class="btn-secondary btn-sm" :disabled="loading" @click="$emit('restart-gateway')">{{ t('dashboard.btn.restart') }}</button>
          <button class="btn-danger btn-sm" :disabled="loading" @click="$emit('stop-gateway')">{{ t('dashboard.btn.stop') }}</button>
        </template>
      </div>
    </div>

    <!-- Numbers -->
    <div class="glass-panel numbers-card">
      <div class="numbers-header">
        <div class="numbers-title-box">
          <h2 class="numbers-title">{{ t('dashboard.devices.title') }}</h2>
          <span v-if="sessions.length" class="numbers-count">{{ t('dashboard.devices.connectedOf', { n: connectedSessionsCount, total: totalSessionsCount }) }}</span>
        </div>
        <div class="numbers-actions">
          <button v-if="sessions.length" class="btn-secondary btn-sm" :disabled="loading || !hasConnectedSessions" :title="t('dashboard.devices.syncAllTitle')" @click="$emit('sync-all')">{{ t('dashboard.devices.syncAll') }}</button>
          <button class="btn-secondary btn-sm icon-only" :title="t('dashboard.devices.refresh')" :aria-label="t('dashboard.devices.refresh')" @click="$emit('refresh-sessions')">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          </button>
          <button class="btn-primary btn-sm" @click="$emit('open-pairing')">{{ t('dashboard.devices.add') }}</button>
        </div>
      </div>

      <div v-if="sessions.length === 0" class="empty-state">
        <div class="empty-text">{{ t('dashboard.devices.empty') }}</div>
        <button class="btn-primary btn-sm" @click="$emit('open-pairing')">{{ t('dashboard.devices.pairFirst') }}</button>
      </div>

      <ul v-else class="number-list">
        <li v-for="session in sessions" :key="session.number_id" class="number-row">
          <div class="number-main">
            <span :class="['status-dot', 'small', session.connected ? 'active' : 'warn']"></span>
            <div class="number-text">
              <span class="number-id font-mono">{{ session.number_id }}</span>
              <span class="number-state">{{ session.connected ? t('dashboard.devices.connected') : t('dashboard.devices.disconnected') }}</span>
            </div>
            <span v-if="session.pairing_key" class="pairing">
              <span class="pairing-label">{{ t('dashboard.devices.pairingKey') }}</span>
              <code class="pairing-code">{{ session.pairing_key }}</code>
              <button class="pairing-copy" :title="t('dashboard.devices.copyPairingKey')" :aria-label="t('dashboard.devices.copyPairingKey')" @click="copyKey(session.pairing_key)">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
              </button>
              <span v-if="copiedKey === session.pairing_key" class="pairing-copied">{{ t('dashboard.devices.copied') }}</span>
            </span>
          </div>
          <div class="number-actions">
            <button v-if="session.connected" class="btn-secondary btn-sm" :title="t('dashboard.devices.syncTitle')" @click="$emit('sync-device', session.number_id)">{{ t('dashboard.devices.sync') }}</button>
            <button v-else class="btn-primary btn-sm" @click="$emit('reconnect-device', session.number_id)">{{ t('dashboard.devices.scanQr') }}</button>
            <button class="btn-danger btn-sm" @click="$emit('logout-device', session.number_id)">{{ t('dashboard.devices.logout') }}</button>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useServiceState } from "../useServiceState";
import { t } from "../i18n";

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
</script>

<style scoped>
.dashboard-view {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.5rem;
  overflow-y: auto;
  height: 100%;
  min-height: 0;
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
}

.status-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 1rem 1.5rem;
  padding: 1.25rem 1.5rem;
}

.status-text {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  min-width: 0;
}

.status-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}

.status-sub {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 0.1rem;
}

.status-dot {
  flex-shrink: 0;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--text-muted);
}

.status-dot.small {
  width: 8px;
  height: 8px;
}

.status-dot.active {
  background: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.6);
}

.status-dot.inactive {
  background: #ef4444;
}

.status-dot.warn {
  background: #f59e0b;
}

.status-dot.loading {
  background: #f59e0b;
  animation: pulse 1.2s ease-in-out infinite;
}

@keyframes pulse {
  50% { opacity: 0.35; }
}

.status-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.main-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem 1.4rem;
  font-size: 0.9rem;
}

.service-note {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.numbers-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.25rem 1.5rem;
}

.numbers-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.6rem 1rem;
}

.numbers-title-box {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
}

.numbers-title {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-primary);
}

.numbers-count {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.numbers-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
}

.btn-sm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0.3rem 0.7rem;
  font-size: 0.75rem;
  white-space: nowrap;
}

.icon-only {
  padding: 0.35rem 0.5rem;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem 1rem;
}

.empty-text {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.number-list {
  list-style: none;
  display: flex;
  flex-direction: column;
}

.number-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.6rem 1rem;
  padding: 0.8rem 0;
  border-top: 1px solid var(--border-subtle);
}

.number-row:first-child {
  border-top: 0;
}

.number-main {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem 0.9rem;
  min-width: 0;
}

.number-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.number-id {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.number-state {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.pairing {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.7rem;
  color: var(--text-muted);
}

.pairing-label {
  white-space: nowrap;
}

.pairing-code {
  font-family: var(--font-mono);
  font-weight: 700;
  white-space: nowrap;
  color: #fdb000;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: 4px;
  padding: 0.15rem 0.4rem;
  user-select: all;
}

.pairing-copy {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem;
  border-radius: 4px;
  color: var(--text-secondary);
}

.pairing-copy:hover {
  background: var(--bg-surface-elevated);
  color: var(--text-primary);
}

.pairing-copied {
  font-weight: 600;
  color: #10b981;
}

.number-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

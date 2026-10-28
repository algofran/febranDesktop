<template>
  <header class="navbar">
    <div class="brand">
      <div class="logo-box">
        <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" stroke="#10b981" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </div>
      <span class="brand-title">Febran</span>
    </div>

    <!-- Navigation Tabs -->
    <nav class="nav-tabs">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="['tab-btn', { active: currentTab === tab.id }]"
        @click="$emit('select-tab', tab.id)"
      >
        <span class="tab-icon" v-html="tab.icon"></span>
        <span>{{ tab.label }}</span>
      </button>
    </nav>

    <!-- Node State & Controls -->
    <div class="status-panel">
      <div class="status-group">
        <!-- Daemon status -->
        <div :class="['node-status-chip', status.running ? 'online' : 'offline', { loading }]" :title="status.running ? `Daemon on :${status.bind.split(':')[1] || '8989'}` : 'Daemon stopped'">
          <span :class="['status-dot', status.running ? 'active' : 'inactive']"></span>
          <span class="status-text">{{ loading ? 'Starting…' : (status.running ? 'Online' : 'Offline') }}</span>
          <span v-if="status.running" class="port-label">:{{ status.bind.split(':')[1] || '8989' }}</span>
        </div>

        <!-- Tenant WebSocket link -->
        <div
          v-if="status.running"
          :class="['ws-status-chip', status.ws_connected ? 'connected' : 'disconnected']"
          :title="status.ws_connected ? 'Connected to tenant backend' : 'Not connected to tenant backend'"
        >
          <span :class="['ws-dot', status.ws_connected ? 'active' : 'inactive']"></span>
          <span>{{ status.ws_connected ? 'Linked' : 'Offline' }}</span>
        </div>
      </div>

      <!-- Action buttons -->
      <button
        v-if="!status.running"
        class="btn-primary start-btn"
        :disabled="loading || svc.installed"
        @click="$emit('start-gateway')"
        title="Start Go WhatsApp Daemon"
      >
        <svg v-if="loading" class="spin-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
        </svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <polygon points="5 3 19 12 5 21 5 3"/>
        </svg>
        <span>{{ loading ? 'Starting…' : 'Start' }}</span>
      </button>

      <div v-else class="action-btn-group">
        <button
          class="icon-btn"
          :disabled="loading || svc.installed"
          @click="$emit('restart-gateway')"
          title="Restart Go Daemon"
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="23 4 23 10 17 10"/>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
          </svg>
        </button>
        <button
          class="icon-btn stop-btn"
          :disabled="loading || svc.installed"
          @click="$emit('stop-gateway')"
          title="Stop Go Daemon"
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
            <rect x="6" y="6" width="12" height="12" rx="2"/>
          </svg>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import type { ProcessStatus } from "../types";
import { useServiceState } from "../useServiceState";

const { svc } = useServiceState();

defineProps<{
  currentTab: string;
  status: ProcessStatus;
  loading: boolean;
  sessionsCount: number;
}>();

defineEmits<{
  (e: "select-tab", tabId: string): void;
  (e: "start-gateway"): void;
  (e: "stop-gateway"): void;
  (e: "restart-gateway"): void;
}>();

const tabs = [
  {
    id: "dashboard",
    label: "Dashboard",
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/></svg>`,
  },
  {
    id: "sessions",
    label: "Devices",
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="5" y="2" width="14" height="20" rx="2"/><line x1="12" y1="18" x2="12.01" y2="18"/></svg>`,
  },
  {
    id: "sandbox",
    label: "Test",
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>`,
  },
  {
    id: "logs",
    label: "Logs",
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>`,
  },
  {
    id: "settings",
    label: "Settings",
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
  },
];
</script>

<style scoped>
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1.5rem;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  user-select: none;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.logo-box {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.brand-title {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.nav-tabs {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  background: var(--bg-input);
  padding: 0.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
}

.tab-btn {
  background: transparent;
  color: var(--text-secondary);
  padding: 0.4rem 0.85rem;
  font-size: 0.8rem;
  font-weight: 500;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
}

.tab-btn.active {
  background: var(--bg-surface-elevated);
  color: #34d399;
  border-color: rgba(255, 255, 255, 0.08);
  font-weight: 600;
  box-shadow: var(--shadow-sm);
}

.tab-icon {
  display: flex;
  align-items: center;
}

.status-panel {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.status-group {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.node-status-chip {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.3rem 0.7rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
  white-space: nowrap;
}

.node-status-chip.online {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.25);
}

.node-status-chip.offline {
  background: rgba(239, 68, 68, 0.12);
  color: #fca5a5;
  border: 1px solid rgba(239, 68, 68, 0.25);
}

.node-status-chip.loading {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border-color: rgba(245, 158, 11, 0.25);
}

.ws-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.3rem 0.7rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
  white-space: nowrap;
}

.ws-status-chip.connected {
  background: rgba(59, 130, 246, 0.12);
  color: #93c5fd;
  border: 1px solid rgba(59, 130, 246, 0.25);
}

.ws-status-chip.disconnected {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.25);
}

.ws-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.ws-dot.active {
  background-color: #3b82f6;
  box-shadow: 0 0 8px #3b82f6;
}

.ws-dot.inactive {
  background-color: #f59e0b;
}

.port-label {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  opacity: 0.85;
}

.action-btn-group {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.start-btn {
  height: 32px;
  padding: 0 0.9rem;
  font-size: 0.8rem;
  font-weight: 600;
  background: #10b981;
  color: #022c1b;
  box-shadow: 0 0 15px rgba(16, 185, 129, 0.25);
}
.start-btn:hover {
  background: #059669;
  box-shadow: 0 0 20px rgba(16, 185, 129, 0.4);
}

.icon-btn {
  width: 32px;
  height: 32px;
  padding: 0;
  background: var(--bg-surface-elevated);
  color: var(--text-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
}
.icon-btn:hover {
  color: var(--text-primary);
  background: #273549;
  border-color: var(--border-hover);
}

.icon-btn.stop-btn {
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  border-color: rgba(239, 68, 68, 0.2);
}
.icon-btn.stop-btn:hover {
  background: rgba(239, 68, 68, 0.2);
}

.spin-icon {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

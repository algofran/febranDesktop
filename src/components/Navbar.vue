<template>
  <header class="navbar">
    <div class="brand">
      <img class="brand-logo" :src="logoUrl" alt="Febran" />
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
        <span>{{ t('navbar.tabs.' + tab.id) }}</span>
      </button>
    </nav>

    <!-- Node State & Controls -->
    <div class="status-panel">
      <div class="lang-switch" role="group" :aria-label="t('navbar.language')">
        <button
          v-for="l in locales"
          :key="l.code"
          :class="['lang-btn', { active: locale === l.code }]"
          :title="l.label"
          @click="setLocale(l.code)"
        >{{ l.code.toUpperCase() }}</button>
      </div>
      <div class="status-group">
        <!-- Daemon status -->
        <div :class="['node-status-chip', status.running ? 'online' : 'offline', { loading }]" :title="status.running ? t('navbar.daemonOn', { port: status.bind.split(':')[1] || '8989' }) : t('navbar.daemonStopped')">
          <span :class="['status-dot', status.running ? 'active' : 'inactive']"></span>
          <span class="status-text">{{ loading ? t('navbar.starting') : (status.running ? t('navbar.online') : t('navbar.offline')) }}</span>
          <span v-if="status.running" class="port-label">:{{ status.bind.split(':')[1] || '8989' }}</span>
        </div>

        <!-- Tenant WebSocket link -->
        <div
          v-if="status.running"
          :class="['ws-status-chip', status.ws_connected ? 'connected' : 'disconnected']"
          :title="status.ws_connected ? t('navbar.linkedTitle') : t('navbar.notLinkedTitle')"
        >
          <span :class="['ws-dot', status.ws_connected ? 'active' : 'inactive']"></span>
          <span>{{ status.ws_connected ? t('navbar.linked') : t('navbar.offline') }}</span>
        </div>
      </div>

      <!-- Action buttons -->
      <button
        v-if="!status.running"
        class="btn-primary start-btn"
        :disabled="loading || svc.installed"
        @click="$emit('start-gateway')"
        :title="t('navbar.startTitle')"
      >
        <svg v-if="loading" class="spin-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
        </svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <polygon points="5 3 19 12 5 21 5 3"/>
        </svg>
        <span>{{ loading ? t('navbar.starting') : t('navbar.start') }}</span>
      </button>

      <div v-else class="action-btn-group">
        <button
          class="icon-btn"
          :disabled="loading || svc.installed"
          @click="$emit('restart-gateway')"
          :title="t('navbar.restartTitle')"
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
          :title="t('navbar.stopTitle')"
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
import logoUrl from '../assets/febran-logo-dark.png'
import type { ProcessStatus } from "../types";
import { useServiceState } from "../useServiceState";
import { t, useI18n } from "../i18n";

const { locale, setLocale, locales } = useI18n();

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

.brand-logo {
  height: 38px;
  width: auto;
  display: block;
  flex-shrink: 0;
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
  color: #F2784F;
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
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.25);
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
  background-color: #10b981;
  box-shadow: 0 0 8px #10b981;
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
  background: #E15A32;
  color: #022c1b;
  box-shadow: 0 0 15px rgba(225, 90, 50, 0.3);
}
.start-btn:hover {
  background: #C94A25;
  box-shadow: 0 0 20px rgba(225, 90, 50, 0.45);
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

.lang-switch {
  display: inline-flex;
  padding: 2px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: var(--bg-input);
}

.lang-btn {
  padding: 0.2rem 0.5rem;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  background: transparent;
  border-radius: calc(var(--radius-md) - 2px);
  border: 0;
}

.lang-btn:hover {
  color: var(--text-primary);
}

.lang-btn.active {
  color: #ffffff;
  background: var(--accent-wa);
}
</style>

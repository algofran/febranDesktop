<template>
  <div class="glass-panel service-card">
    <div class="svc-header">
      <div>
        <h2 class="svc-title">{{ t('service.title') }}</h2>
        <p class="svc-subtitle">{{ t('service.subtitle') }}</p>
      </div>
      <span :class="['svc-badge', badge.cls]">{{ badge.text }}</span>
    </div>

    <div v-if="busy" class="svc-waiting">
      <svg class="spin-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
      </svg>
      <span>{{ t('service.waitingUac') }}</span>
    </div>

    <div v-if="notice" :class="['svc-notice', notice.type]">
      <span>{{ notice.text }}</span>
      <button class="svc-dismiss" :title="t('service.close')" @click="notice = null">×</button>
    </div>

    <!-- Not installed -->
    <template v-if="!info.installed">
      <p class="svc-text">
        {{ t('service.intro1') }}
        {{ t('service.intro2') }}
      </p>
      <p class="svc-warn">
        {{ dirty ? t('service.dirtyInstall') : t('service.savedConfigNote') }}
      </p>
      <div class="svc-actions">
        <button class="btn-primary" :disabled="busy || dirty" @click="run(() => api.serviceInstall())">
          {{ t('service.install') }}
        </button>
      </div>
    </template>

    <!-- Installed -->
    <template v-else>
      <div class="svc-actions">
        <button v-if="info.state === 'stopped'" class="btn-primary" :disabled="busy" @click="run(() => api.serviceControl('start'))">{{ t('service.start') }}</button>
        <button v-if="info.state === 'running'" class="btn-secondary" :disabled="busy" @click="run(() => api.serviceControl('stop'))">{{ t('service.stop') }}</button>
        <button class="btn-secondary" :disabled="busy" @click="run(() => api.serviceControl('restart'))">{{ t('service.restart') }}</button>
        <button class="btn-secondary" :disabled="busy || dirty" @click="run(() => api.serviceInstall())">{{ t('service.applyConfig') }}</button>
        <button class="btn-danger" :disabled="busy" @click="confirmOpen = true">{{ t('service.uninstall') }}</button>
      </div>
      <p class="svc-hint">{{ t('service.applyHint') }}</p>
      <p v-if="dirty" class="svc-warn">{{ t('service.dirtyApply') }}</p>
    </template>

    <div v-if="info.data_dir || info.log_file" class="svc-paths">
      <div v-if="info.data_dir" class="svc-path">
        <span class="svc-path-label">{{ t('service.data') }}</span>
        <code class="font-mono">{{ info.data_dir }}</code>
        <button class="btn-secondary btn-sm" @click="copy(info.data_dir)">{{ t('service.copy') }}</button>
      </div>
      <div v-if="info.log_file" class="svc-path">
        <span class="svc-path-label">{{ t('service.log') }}</span>
        <code class="font-mono">{{ info.log_file }}</code>
        <button class="btn-secondary btn-sm" @click="copy(info.log_file)">{{ t('service.copy') }}</button>
      </div>
    </div>

    <!-- Confirm dialog -->
    <div v-if="confirmOpen" class="svc-overlay" @click.self="confirmOpen = false">
      <div class="glass-panel svc-dialog">
        <h3 class="svc-title">{{ t('service.confirmTitle') }}</h3>
        <p class="svc-text">
          {{ t('service.confirmText') }}

        </p>
        <label class="svc-check">
          <input v-model="purge" type="checkbox" />
          <span>{{ t('service.purge') }}</span>
        </label>
        <div class="svc-actions">
          <button class="btn-secondary" @click="confirmOpen = false">{{ t('service.cancel') }}</button>
          <button class="btn-danger" @click="doUninstall">{{ t('service.uninstall') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { api } from "../api";
import type { ServiceInfo } from "../types";
import { useServiceState } from "../useServiceState";
import { t } from "../i18n";

defineProps<{ dirty?: boolean }>();

const { setServiceState } = useServiceState();

const info = ref<ServiceInfo>({ ok: true, message: "", installed: false, state: "not_installed", data_dir: "", log_file: "" });
const busy = ref(false);
const notice = ref<{ type: "success" | "error"; text: string } | null>(null);
const confirmOpen = ref(false);
const purge = ref(false);
let timer: ReturnType<typeof setInterval> | null = null;

const apply = (i: ServiceInfo) => {
  info.value = i;
  setServiceState(i.installed, i.state);
};

const refresh = async () => {
  if (busy.value) return;
  try {
    apply(await api.serviceStatus());
  } catch (err) {
    console.warn("service_status failed:", err);
  }
};

const run = async (action: () => Promise<ServiceInfo>) => {
  busy.value = true;
  notice.value = null;
  try {
    const res = await action();
    apply(res);
    if (res.message) notice.value = { type: res.ok ? "success" : "error", text: res.message };
  } catch (err: unknown) {
    notice.value = { type: "error", text: err instanceof Error ? err.message : String(err) };
  } finally {
    busy.value = false;
  }
  await refresh();
};

const doUninstall = async () => {
  confirmOpen.value = false;
  const p = purge.value;
  purge.value = false;
  await run(() => api.serviceUninstall(p));
};

const copy = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // clipboard unavailable
  }
};

const badge = computed(() => {
  switch (info.value.state) {
    case "running": return { text: t('service.state.running'), cls: "ok" };
    case "stopped":
    case "paused": return { text: t('service.state.stopped'), cls: "off" };
    case "start_pending":
    case "continue_pending": return { text: t('service.state.starting'), cls: "wait" };
    case "stop_pending":
    case "pause_pending": return { text: t('service.state.stopping'), cls: "wait" };
    case "not_installed": return { text: t('service.state.notInstalled'), cls: "none" };
    default: return { text: t('service.state.unknown'), cls: "none" };
  }
});

onMounted(() => {
  refresh();
  timer = setInterval(refresh, 5000);
});
onUnmounted(() => {
  if (timer) clearInterval(timer);
});
</script>

<style scoped>
.service-card {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 960px;
  margin: 0 auto;
  position: relative;
}
.svc-header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-subtle); padding-bottom: 1rem; }
.svc-title { font-size: 1.2rem; font-weight: 700; color: var(--text-primary); }
.svc-subtitle { font-size: 0.82rem; color: var(--text-muted); }
.svc-badge { font-size: 0.75rem; font-weight: 700; padding: 0.25rem 0.7rem; border-radius: 999px; }
.svc-badge.ok { background: rgba(16, 185, 129, 0.15); color: #34d399; }
.svc-badge.off { background: rgba(148, 163, 184, 0.15); color: #9ca3af; }
.svc-badge.wait { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.svc-badge.none { background: rgba(100, 116, 139, 0.2); color: #94a3b8; }
.svc-text { font-size: 0.85rem; color: var(--text-secondary); line-height: 1.5; }
.svc-hint { font-size: 0.75rem; color: var(--text-muted); }
.svc-warn { font-size: 0.78rem; color: #fbbf24; }
.svc-actions { display: flex; gap: 0.6rem; flex-wrap: wrap; justify-content: flex-start; }
.svc-waiting { display: flex; align-items: center; gap: 0.6rem; font-size: 0.82rem; color: #fbbf24; background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.35); border-radius: var(--radius-md); padding: 0.6rem 0.8rem; }
.svc-notice { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; font-size: 0.82rem; border-radius: var(--radius-md); padding: 0.55rem 0.8rem; }
.svc-notice.success { background: rgba(16, 185, 129, 0.12); color: #34d399; border: 1px solid #059669; }
.svc-notice.error { background: rgba(239, 68, 68, 0.12); color: #fca5a5; border: 1px solid #b91c1c; }
.svc-dismiss { background: none; border: none; color: inherit; font-size: 1.1rem; cursor: pointer; }
.svc-paths { display: flex; flex-direction: column; gap: 0.4rem; border-top: 1px solid var(--border-subtle); padding-top: 0.8rem; }
.svc-path { display: flex; align-items: center; gap: 0.5rem; font-size: 0.72rem; color: var(--text-muted); }
.svc-path code { color: var(--text-secondary); word-break: break-all; }
.svc-path-label { min-width: 2.5rem; }
.svc-overlay { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.svc-dialog { padding: 1.5rem; max-width: 460px; display: flex; flex-direction: column; gap: 1rem; background: var(--bg-surface, #111827); }
.svc-check { display: flex; gap: 0.5rem; align-items: flex-start; font-size: 0.82rem; color: var(--text-primary); }
.spin-icon { animation: spin 0.8s linear infinite; flex-shrink: 0; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>

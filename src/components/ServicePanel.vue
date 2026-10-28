<template>
  <div class="glass-panel service-card">
    <div class="svc-header">
      <div>
        <h2 class="svc-title">Mode Service</h2>
        <p class="svc-subtitle">Jalankan gateway sebagai service Windows</p>
      </div>
      <span :class="['svc-badge', badge.cls]">{{ badge.text }}</span>
    </div>

    <div v-if="busy" class="svc-waiting">
      <svg class="spin-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
      </svg>
      <span>Menunggu izin Administrator (UAC)… cek jendela yang mungkin muncul di belakang aplikasi.</span>
    </div>

    <div v-if="notice" :class="['svc-notice', notice.type]">
      <span>{{ notice.text }}</span>
      <button class="svc-dismiss" title="Tutup" @click="notice = null">×</button>
    </div>

    <!-- Not installed -->
    <template v-if="!info.installed">
      <p class="svc-text">
        Gateway tetap berjalan di latar belakang walau aplikasi ditutup dan menyala otomatis saat Windows start.
        Akan meminta izin Administrator (satu kali). Sesi WhatsApp disalin ke service, dan gateway milik aplikasi dihentikan.
      </p>
      <p class="svc-warn">
        {{ dirty ? 'Ada perubahan yang belum disimpan — simpan konfigurasi dulu.' : 'Service memakai konfigurasi yang sudah tersimpan, pastikan pengaturan sudah disimpan.' }}
      </p>
      <div class="svc-actions">
        <button class="btn-primary" :disabled="busy || dirty" @click="run(() => api.serviceInstall())">
          Jalankan sebagai Service
        </button>
      </div>
    </template>

    <!-- Installed -->
    <template v-else>
      <div class="svc-actions">
        <button v-if="info.state === 'stopped'" class="btn-primary" :disabled="busy" @click="run(() => api.serviceControl('start'))">Mulai</button>
        <button v-if="info.state === 'running'" class="btn-secondary" :disabled="busy" @click="run(() => api.serviceControl('stop'))">Hentikan</button>
        <button class="btn-secondary" :disabled="busy" @click="run(() => api.serviceControl('restart'))">Restart</button>
        <button class="btn-secondary" :disabled="busy || dirty" @click="run(() => api.serviceInstall())">Terapkan Konfigurasi</button>
        <button class="btn-danger" :disabled="busy" @click="confirmOpen = true">Hapus Service</button>
      </div>
      <p class="svc-hint">Service memakai salinan konfigurasi sendiri. Setelah mengubah pengaturan, klik ini agar berlaku.</p>
      <p v-if="dirty" class="svc-warn">Ada perubahan yang belum disimpan — simpan konfigurasi dulu sebelum menerapkan.</p>
    </template>

    <div v-if="info.data_dir || info.log_file" class="svc-paths">
      <div v-if="info.data_dir" class="svc-path">
        <span class="svc-path-label">Data:</span>
        <code class="font-mono">{{ info.data_dir }}</code>
        <button class="btn-secondary btn-sm" @click="copy(info.data_dir)">Salin</button>
      </div>
      <div v-if="info.log_file" class="svc-path">
        <span class="svc-path-label">Log:</span>
        <code class="font-mono">{{ info.log_file }}</code>
        <button class="btn-secondary btn-sm" @click="copy(info.log_file)">Salin</button>
      </div>
    </div>

    <!-- Confirm dialog -->
    <div v-if="confirmOpen" class="svc-overlay" @click.self="confirmOpen = false">
      <div class="glass-panel svc-dialog">
        <h3 class="svc-title">Hapus Service?</h3>
        <p class="svc-text">
          Service akan dihapus. Sesi WhatsApp terbaru disalin kembali ke aplikasi, jadi login tetap valid
          kecuali data service ikut dihapus.
        </p>
        <label class="svc-check">
          <input v-model="purge" type="checkbox" />
          <span>Hapus juga data service (sesi WhatsApp di service ikut terhapus)</span>
        </label>
        <div class="svc-actions">
          <button class="btn-secondary" @click="confirmOpen = false">Batal</button>
          <button class="btn-danger" @click="doUninstall">Hapus Service</button>
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
    case "running": return { text: "Berjalan", cls: "ok" };
    case "stopped":
    case "paused": return { text: "Berhenti", cls: "off" };
    case "start_pending":
    case "continue_pending": return { text: "Memulai…", cls: "wait" };
    case "stop_pending":
    case "pause_pending": return { text: "Menghentikan…", cls: "wait" };
    case "not_installed": return { text: "Belum terpasang", cls: "none" };
    default: return { text: "Tidak diketahui", cls: "none" };
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

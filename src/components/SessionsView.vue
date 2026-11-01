<template>
  <div class="sessions-view">
    <div class="sessions-layout">
      <!-- Left: Devices List Table -->
      <div class="glass-panel main-col">
        <div class="col-header">
          <div>
            <h2 class="col-title">Registered WhatsApp Devices</h2>
            <p class="col-subtitle">Multi-device SQLite sessions active on this daemon node</p>
          </div>
          <div class="header-actions">
            <button 
              class="btn-secondary btn-sm" 
              :disabled="syncingAll || !hasConnectedSessions" 
              @click="syncAllData"
              title="Tarik dan sinkronkan semua kontak dan grup dari seluruh nomor yang terhubung ke backend"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'spin-icon': syncingAll }">
                <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
              </svg>
              <span>{{ syncingAll ? 'Menyinkronkan...' : 'Tarik & Sync Semua' }}</span>
            </button>
            <button class="btn-primary btn-sm" @click="openPairModal">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="12" y1="5" x2="12" y2="19"/>
                <line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
              <span>Add Number</span>
            </button>
          </div>
        </div>

        <div v-if="sessions.length === 0" class="empty-box">
          <div class="empty-icon">📱</div>
          <div class="empty-title">No sessions registered</div>
          <div class="empty-desc">Click "Add Number" to scan a QR code with your WhatsApp mobile app.</div>
          <button class="btn-primary" @click="openPairModal">Start QR Pairing</button>
        </div>

        <div v-else class="sessions-list">
          <div 
            v-for="s in sessions" 
            :key="s.number_id" 
            :class="['session-card', { selected: activePairNumberId === s.number_id }]"
          >
            <div class="session-main">
              <div class="session-icon-box">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" :stroke="s.connected ? '#10b981' : '#f59e0b'" stroke-width="2">
                  <rect x="5" y="2" width="14" height="20" rx="2"/>
                  <line x1="12" y1="18" x2="12.01" y2="18"/>
                </svg>
              </div>
              <div>
                <div class="session-phone font-mono">{{ s.number_id }}</div>
                <div class="session-db">File: {{ s.number_id }}.db</div>

                <div v-if="s.pairing_key" class="pairing-key-row">
                  <span class="pairing-key-label">Pairing Key:</span>
                  <code class="pairing-key-code">{{ s.pairing_key }}</code>
                  <button
                    class="pairing-key-copy"
                    title="Salin Kode Pairing"
                    @click="copyKey(s.pairing_key)"
                  >
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                </div>
                <div v-if="s.pairing_key" class="pairing-key-helper">
                  Gunakan kode ini saat mendaftarkan nomor di Web Panel Tenant.
                </div>
              </div>
            </div>

            <div class="session-status">
              <span :class="['badge', s.connected ? 'badge-success' : 'badge-warning']">
                {{ s.connected ? 'Connected' : 'Disconnected' }}
              </span>
            </div>

            <div class="session-controls">
              <button 
                v-if="s.connected"
                class="btn-secondary btn-sm"
                :disabled="syncingDevice[s.number_id]"
                @click="syncDeviceData(s.number_id)"
                title="Tarik kontak & grup dari nomor ini dan sinkronkan ke backend"
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'spin-icon': syncingDevice[s.number_id] }">
                  <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
                </svg>
                <span>{{ syncingDevice[s.number_id] ? 'Syncing...' : 'Sync Data' }}</span>
              </button>
              <button 
                v-if="!s.connected" 
                class="btn-secondary btn-sm" 
                @click="startPairing()"
              >
                Scan QR
              </button>
              <button 
                class="btn-danger btn-sm" 
                @click="logoutDevice(s.number_id)"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: Live QR Code & Pairing Box -->
      <div class="glass-panel side-col">
        <div class="col-header">
          <div>
            <h3 class="col-title">QR Pairing Terminal</h3>
            <p class="col-subtitle">Scan via WhatsApp > Linked Devices</p>
          </div>
        </div>

        <div v-if="!pairingActive" class="pairing-idle-state">
          <div class="idle-graphic">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#64748b" stroke-width="1.5">
              <rect x="3" y="3" width="7" height="7"/>
              <rect x="14" y="3" width="7" height="7"/>
              <rect x="14" y="14" width="7" height="7"/>
              <rect x="3" y="14" width="7" height="7"/>
            </svg>
          </div>
          <div class="idle-title">No Active Pairing</div>
          <p class="idle-desc">Click the button below to generate a QR code. Then scan it with WhatsApp on your phone.</p>
          <button 
            class="btn-primary" 
            :disabled="loading" 
            @click="startPairing()"
          >
            Start QR Pairing
          </button>
        </div>

        <div v-else class="qr-active-box">
          <div v-if="activePairNumberId" class="qr-target-info">
            <span>Session:</span>
            <span class="font-mono font-bold">{{ activePairNumberId }}</span>
          </div>

          <div class="qr-frame">
            <div v-if="qrLoading" class="qr-spinner">
              <div class="loading-spinner"></div>
              <span>Generating QR Code...</span>
            </div>
            <img 
              v-else-if="qrCodeData" 
              :src="qrCodeData" 
              alt="WhatsApp Pairing QR" 
              class="qr-image"
            />
            <div v-else class="qr-error">
              <span>QR Expired or Device Already Paired</span>
              <button class="btn-secondary btn-sm" @click="fetchQR">Refresh QR</button>
            </div>
          </div>

          <div class="qr-timer-bar">
            <div class="qr-progress" :style="{ width: `${qrTimerPercent}%` }"></div>
          </div>

          <div class="qr-instructions">
            <ol>
              <li>Open WhatsApp on your mobile phone</li>
              <li>Tap <strong>Settings</strong> > <strong>Linked Devices</strong></li>
              <li>Tap <strong>Link a Device</strong> and point your camera at this QR code</li>
            </ol>
          </div>

          <div class="qr-bottom-actions">
            <button class="btn-secondary btn-sm" @click="fetchQR">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23 4 23 10 17 10"/>
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
              </svg>
              <span>Refresh</span>
            </button>
            <button class="btn-danger btn-sm" @click="cancelPairing">Cancel</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Sync Result Modal -->
    <div v-if="syncResultModal" class="sync-modal-backdrop" @click.self="syncResultModal = false">
      <div class="glass-panel sync-modal-content">
        <div class="sync-modal-header">
          <div>
            <h3 class="sync-modal-title">Hasil Penarikan & Sinkronisasi</h3>
            <p class="sync-modal-subtitle">Kontak dan grup WhatsApp berhasil ditarik dan disinkronkan ke backend pusat</p>
          </div>
          <button class="icon-btn-close" @click="syncResultModal = false">✕</button>
        </div>

        <div class="sync-summary-grid">
          <div class="sync-stat-card">
            <span class="sync-stat-label">Sesi / Nomor</span>
            <span class="sync-stat-num text-emerald">{{ syncStats.sessions }}</span>
          </div>
          <div class="sync-stat-card">
            <span class="sync-stat-label">Total Kontak</span>
            <span class="sync-stat-num text-emerald">{{ syncStats.contacts }}</span>
          </div>
          <div class="sync-stat-card">
            <span class="sync-stat-label">Total Grup</span>
            <span class="sync-stat-num text-emerald">{{ syncStats.groups }}</span>
          </div>
        </div>

        <!-- Details list -->
        <div class="sync-details-box">
          <div v-for="item in syncDetails" :key="item.number_id" class="sync-device-card">
            <div class="sync-device-head">
              <div class="device-title-row">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#E15A32" stroke-width="2">
                  <rect x="5" y="2" width="14" height="20" rx="2"/>
                  <line x1="12" y1="18" x2="12.01" y2="18"/>
                </svg>
                <span class="font-mono font-bold">{{ item.number_id }}</span>
              </div>
              <div class="sync-badges">
                <span class="badge badge-success">{{ item.contacts.length }} Kontak</span>
                <span class="badge badge-neutral">{{ item.groups.length }} Grup</span>
              </div>
            </div>
            
            <div v-if="item.error" class="sync-device-err">
              ⚠️ {{ item.error }}
            </div>

            <div class="sync-sample-lists">
              <div class="sync-sample-col" v-if="item.groups.length > 0">
                <div class="sample-label">Grup WhatsApp ({{ item.groups.length }})</div>
                <div class="sample-items">
                  <div v-for="(g, idx) in item.groups.slice(0, 5)" :key="idx" class="sample-row">
                    <span class="truncate">{{ g.name || g.Name || g.jid || g.JID }}</span>
                    <span class="font-mono text-xs">{{ g.participant_count || g.ParticipantCount || g.member_count || 0 }} member</span>
                  </div>
                  <div v-if="item.groups.length > 5" class="sample-more">
                    +{{ item.groups.length - 5 }} grup lainnya
                  </div>
                </div>
              </div>

              <div class="sync-sample-col" v-if="item.contacts.length > 0">
                <div class="sample-label">Kontak WhatsApp ({{ item.contacts.length }})</div>
                <div class="sample-items">
                  <div v-for="(c, idx) in item.contacts.slice(0, 5)" :key="idx" class="sample-row">
                    <span class="truncate">{{ c.name || c.phone }}</span>
                    <span class="font-mono text-xs">{{ c.phone }}</span>
                  </div>
                  <div v-if="item.contacts.length > 5" class="sample-more">
                    +{{ item.contacts.length - 5 }} kontak lainnya
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="sync-modal-footer">
          <button class="btn-primary btn-sm" @click="syncResultModal = false">Tutup</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from "vue";
import { api } from "../api";
import type { SessionInfo, AllSyncResult, SessionSyncData } from "../types";

const props = defineProps<{
  sessions: SessionInfo[];
  loading: boolean;
}>();

const emit = defineEmits<{
  (e: "refresh-sessions"): void;
  (e: "toast", msg: string, type?: "success" | "error"): void;
}>();

const syncingAll = ref(false);
const syncingDevice = ref<Record<string, boolean>>({});
const syncResultModal = ref(false);
const lastSyncResult = ref<AllSyncResult | SessionSyncData | null>(null);
const lastSyncType = ref<'all' | 'device'>('all');

const hasConnectedSessions = computed(() => props.sessions.some(s => s.connected));

const syncStats = computed(() => {
  if (!lastSyncResult.value) return { sessions: 0, contacts: 0, groups: 0 };
  if (lastSyncType.value === 'all') {
    const res = lastSyncResult.value as AllSyncResult;
    return {
      sessions: res.total_sessions || res.results?.length || 0,
      contacts: res.total_contacts || 0,
      groups: res.total_groups || 0,
    };
  } else {
    const res = lastSyncResult.value as SessionSyncData;
    return {
      sessions: 1,
      contacts: res.contacts?.length || 0,
      groups: res.groups?.length || 0,
    };
  }
});

const syncDetails = computed<SessionSyncData[]>(() => {
  if (!lastSyncResult.value) return [];
  if (lastSyncType.value === 'all') {
    const res = lastSyncResult.value as AllSyncResult;
    return res.results || [];
  } else {
    return [lastSyncResult.value as SessionSyncData];
  }
});

const syncAllData = async () => {
  if (syncingAll.value) return;
  syncingAll.value = true;
  try {
    const res = await api.syncAll();
    lastSyncResult.value = res.data;
    lastSyncType.value = 'all';
    syncResultModal.value = true;
    emit("toast", `Sinkronisasi berhasil: ${res.data.total_contacts} kontak & ${res.data.total_groups} grup dari ${res.data.total_sessions} nomor`, "success");
    emit("refresh-sessions");
  } catch (err: unknown) {
    emit("toast", `Gagal sinkronkan semua: ${err instanceof Error ? err.message : String(err)}`, "error");
  } finally {
    syncingAll.value = false;
  }
};

const syncDeviceData = async (numberId: string) => {
  if (syncingDevice.value[numberId]) return;
  syncingDevice.value[numberId] = true;
  try {
    const res = await api.syncDevice(numberId);
    lastSyncResult.value = res.data;
    lastSyncType.value = 'device';
    syncResultModal.value = true;
    emit("toast", `Sinkronisasi ${numberId} berhasil: ${res.data.contacts.length} kontak & ${res.data.groups.length} grup`, "success");
    emit("refresh-sessions");
  } catch (err: unknown) {
    emit("toast", `Gagal sinkronkan ${numberId}: ${err instanceof Error ? err.message : String(err)}`, "error");
  } finally {
    syncingDevice.value[numberId] = false;
  }
};


const pairingActive = ref(false);
const activePairNumberId = ref("");
const qrCodeData = ref("");
const qrLoading = ref(false);
const qrTimerPercent = ref(100);
const qrShownOnce = ref(false);

let qrPollInterval: ReturnType<typeof setInterval> | null = null;
let timerInterval: ReturnType<typeof setInterval> | null = null;

const openPairModal = () => {
  startPairing();
};

const startPairing = async () => {
  pairingActive.value = true;
  qrLoading.value = true;
  activePairNumberId.value = '';
  qrShownOnce.value = false;

  try {
    const res = await api.pairQR();
    activePairNumberId.value = res.session_id;
    emit("toast", "QR pairing started — scan with your phone");
    startPolling();
  } catch (err: unknown) {
    console.error("Pair error:", err);
    emit("toast", `Pairing failed: ${err instanceof Error ? err.message : String(err)}`, "error");
    cancelPairing();
  }
};

const fetchQR = async () => {
  if (!activePairNumberId.value) return;
  try {
    // Check if connected first
    const status = await api.getSessionStatus(activePairNumberId.value);
    if (status.connected) {
      emit("toast", `Number ${activePairNumberId.value} connected successfully!`, "success");
      cancelPairing();
      emit("refresh-sessions");
      return;
    }

    const res = await api.getQR(activePairNumberId.value);
    if (res && res.qr) {
      qrCodeData.value = res.qr;
      qrLoading.value = false;
      qrShownOnce.value = true;
      resetTimer();
    } else if (qrShownOnce.value) {
      // QR was shown but has since cleared (expired) — surface the error state.
      qrCodeData.value = "";
      qrLoading.value = false;
    }
    // Otherwise QR not ready yet — keep the spinner, poll will retry.
  } catch {
    // QR not ready yet. Keep spinner unless a QR was previously shown.
    if (qrShownOnce.value) {
      qrCodeData.value = "";
      qrLoading.value = false;
    }
  }
};

const resetTimer = () => {
  qrTimerPercent.value = 100;
};

const startPolling = () => {
  stopPolling();
  fetchQR(); // immediate first attempt
  qrPollInterval = setInterval(fetchQR, 1500);
  timerInterval = setInterval(() => {
    if (qrTimerPercent.value > 0) {
      qrTimerPercent.value -= 5;
    }
  }, 1000);
};

const stopPolling = () => {
  if (qrPollInterval) clearInterval(qrPollInterval);
  if (timerInterval) clearInterval(timerInterval);
  qrPollInterval = null;
  timerInterval = null;
};

const cancelPairing = () => {
  pairingActive.value = false;
  activePairNumberId.value = "";
  qrCodeData.value = "";
  qrShownOnce.value = false;
  stopPolling();
};

const copyKey = async (key?: string) => {
  if (!key) return;
  try {
    await navigator.clipboard.writeText(key);
    emit("toast", "Pairing key disalin", "success");
  } catch {
    emit("toast", "Gagal menyalin pairing key", "error");
  }
};

const logoutDevice = async (numberId: string) => {
  if (!confirm(`Are you sure you want to log out and delete session for ${numberId}?`)) return;
  try {
    await api.logout(numberId, false);
    emit("toast", `Logged out ${numberId}`, "success");
    emit("refresh-sessions");
  } catch (err: unknown) {
    emit("toast", `Logout failed: ${err instanceof Error ? err.message : String(err)}`, "error");
  }
};



onUnmounted(() => {
  stopPolling();
});
</script>

<style scoped>
.sessions-view {
  padding: 1.5rem;
  overflow-y: auto;
  height: calc(100vh - 65px);
}

.sessions-layout {
  display: grid;
  grid-template-columns: 1.3fr 0.9fr;
  gap: 1.25rem;
}

.glass-panel {
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.col-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.col-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}

.col-subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.empty-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1.5rem;
  text-align: center;
  gap: 0.75rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px dashed var(--border-subtle);
}

.empty-icon {
  font-size: 2.5rem;
}

.empty-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.empty-desc {
  font-size: 0.85rem;
  color: var(--text-muted);
  max-width: 320px;
  margin-bottom: 0.5rem;
}

.sessions-list {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}

.session-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.85rem 1rem;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all 0.15s ease;
}

.session-card.selected {
  border-color: #E15A32;
  background: rgba(225, 90, 50, 0.06);
}

.session-main {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.session-icon-box {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: var(--bg-surface-elevated);
  display: flex;
  align-items: center;
  justify-content: center;
}

.session-phone {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.session-db {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.pairing-key-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-top: 0.45rem;
}

.pairing-key-label {
  font-size: 0.68rem;
  color: var(--text-muted);
}

.pairing-key-code {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 700;
  color: #FDB000;
  background: #1e293b;
  padding: 0.2rem 0.45rem;
  border-radius: 4px;
  border: 1px solid #334155;
  user-select: all;
}

.pairing-key-copy {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.2rem;
  border-radius: 4px;
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.pairing-key-copy:hover {
  background: var(--bg-surface-elevated);
  color: var(--text-primary);
}

.pairing-key-helper {
  font-size: 0.65rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  line-height: 1.3;
}

.session-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

/* Right Pairing Side */
.pairing-idle-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1.5rem;
  text-align: center;
  gap: 0.75rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
}

.idle-graphic {
  padding: 1rem;
  border-radius: 50%;
  background: var(--bg-surface-elevated);
  margin-bottom: 0.25rem;
}

.idle-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.idle-desc {
  font-size: 0.8rem;
  color: var(--text-muted);
  max-width: 280px;
  line-height: 1.4;
}



.qr-active-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  background: var(--bg-input);
  padding: 1.25rem;
  border-radius: var(--radius-md);
}

.qr-target-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.qr-frame {
  width: 220px;
  height: 220px;
  background: #ffffff;
  border-radius: var(--radius-md);
  padding: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.4);
}

.qr-image {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.qr-spinner, .qr-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: #334155;
  font-size: 0.75rem;
  gap: 0.5rem;
}

.loading-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(225, 90, 50, 0.2);
  border-top-color: #E15A32;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.qr-timer-bar {
  width: 220px;
  height: 4px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
  overflow: hidden;
}

.qr-progress {
  height: 100%;
  background: #E15A32;
  transition: width 1s linear;
}

.qr-instructions {
  font-size: 0.78rem;
  color: var(--text-secondary);
  background: var(--bg-surface-elevated);
  padding: 0.75rem 1rem;
  border-radius: var(--radius-sm);
  width: 100%;
}

.qr-instructions ol {
  padding-left: 1.2rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.qr-bottom-actions {
  display: flex;
  gap: 0.5rem;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.spin-icon {
  animation: spin 0.8s linear infinite;
}

/* Sync Result Modal */
.sync-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 1.5rem;
}

.sync-modal-content {
  width: 100%;
  max-width: 650px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: var(--radius-lg);
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}

.sync-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.sync-modal-title {
  font-size: 1.15rem;
  font-weight: 600;
  color: #ffffff;
  margin: 0;
}

.sync-modal-subtitle {
  font-size: 0.8rem;
  color: #94a3b8;
  margin-top: 0.25rem;
}

.icon-btn-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 1.2rem;
  cursor: pointer;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
}

.icon-btn-close:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #ffffff;
}

.sync-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.sync-stat-card {
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.sync-stat-label {
  font-size: 0.75rem;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.sync-stat-num {
  font-size: 1.5rem;
  font-weight: 700;
  margin-top: 0.25rem;
}

.text-emerald {
  color: #10b981;
}

.sync-details-box {
  padding: 1.25rem 1.5rem;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.sync-device-card {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: var(--radius-md);
  padding: 1rem;
}

.sync-device-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}

.device-title-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.sync-badges {
  display: flex;
  gap: 0.5rem;
}

.sync-device-err {
  font-size: 0.8rem;
  color: #f87171;
  background: rgba(239, 68, 68, 0.1);
  padding: 0.5rem;
  border-radius: 4px;
  margin-bottom: 0.5rem;
}

.sync-sample-lists {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.sync-sample-col {
  background: rgba(30, 41, 59, 0.4);
  border-radius: var(--radius-sm);
  padding: 0.6rem 0.75rem;
}

.sample-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  color: #94a3b8;
  margin-bottom: 0.4rem;
}

.sample-items {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.sample-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.78rem;
  color: #e2e8f0;
  gap: 0.5rem;
}

.sample-more {
  font-size: 0.72rem;
  color: #64748b;
  font-style: italic;
  margin-top: 0.2rem;
}

.sync-modal-footer {
  padding: 1rem 1.5rem;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: flex-end;
}
</style>

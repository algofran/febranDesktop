<template>
  <div class="sandbox-view">
    <div class="sandbox-layout">
      <!-- Left: Message Composer -->
      <div class="glass-panel composer-card">
        <div class="card-header">
          <div>
            <h2 class="card-title">Message Dispatch Sandbox</h2>
            <p class="card-subtitle">Test sending direct messages or group chats through local WhatsApp sessions</p>
          </div>
          <div class="badge badge-success">POST /api/v1/send</div>
        </div>

        <div class="form-grid">
          <!-- Dispatch Mode -->
          <div class="form-group">
            <label class="form-label">Message Target Mode</label>
            <div class="mode-toggles">
              <button 
                :class="['toggle-btn', { active: mode === 'direct' }]" 
                @click="mode = 'direct'"
              >
                Direct User (Phone)
              </button>
              <button 
                :class="['toggle-btn', { active: mode === 'group' }]" 
                @click="mode = 'group'"
              >
                WhatsApp Group
              </button>
            </div>
          </div>

          <!-- Sender Phone -->
          <div class="form-group">
            <label class="form-label">Sender Device (Session)</label>
            <select v-model="selectedNumberId" class="font-mono">
              <option value="" disabled>Select a connected WhatsApp number</option>
              <option v-for="s in connectedSessions" :key="s.number_id" :value="s.number_id">
                {{ s.number_id }} (Connected)
              </option>
            </select>
            <div v-if="connectedSessions.length === 0" class="input-hint text-warning">
              No connected session available. Please pair a number first.
            </div>
          </div>

          <!-- Recipient -->
          <div class="form-group">
            <label class="form-label">
              {{ mode === 'direct' ? 'Recipient Phone Number' : 'Target Group JID' }}
            </label>
            <input 
              v-model="recipient" 
              type="text" 
              :placeholder="mode === 'direct' ? 'e.g. 6281234567890' : 'e.g. 120363041234567890@g.us'" 
              class="font-mono"
            />
            <div class="input-hint">
              {{ mode === 'direct' ? 'Include country code without + (e.g. 628...)' : 'Group JID format: ID@g.us' }}
            </div>
          </div>

          <!-- Quick Templates -->
          <div class="form-group">
            <label class="form-label">Quick Templates</label>
            <div class="template-chips">
              <button 
                v-for="tpl in templates" 
                :key="tpl.label" 
                class="chip-btn" 
                @click="applyTemplate(tpl.text)"
              >
                {{ tpl.label }}
              </button>
            </div>
          </div>

          <!-- Message Text -->
          <div class="form-group">
            <label class="form-label">Message Content</label>
            <textarea 
              v-model="messageText" 
              rows="4" 
              placeholder="Type your WhatsApp message here..."
            ></textarea>
          </div>

          <!-- Actions -->
          <div class="form-actions">
            <button 
              class="btn-primary send-btn" 
              :disabled="!canSend || sending" 
              @click="sendMessage"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="22" y1="2" x2="11" y2="13"/>
                <polygon points="22 2 15 22 11 13 2 9 22 2"/>
              </svg>
              <span>{{ sending ? 'Sending Message...' : 'Send WhatsApp Message' }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Right: Dispatch Receipts & Live Logs -->
      <div class="glass-panel receipts-card">
        <div class="card-header">
          <div>
            <h3 class="card-title">Dispatch Receipts</h3>
            <p class="card-subtitle">Real-time outcome of messages dispatched in this session</p>
          </div>
          <button v-if="receipts.length > 0" class="btn-secondary btn-sm" @click="receipts = []">
            Clear
          </button>
        </div>

        <div v-if="receipts.length === 0" class="empty-receipts">
          <div class="empty-icon">📨</div>
          <div class="empty-text">No messages dispatched yet in this session.</div>
        </div>

        <div v-else class="receipts-list">
          <div 
            v-for="(r, idx) in receipts" 
            :key="idx" 
            :class="['receipt-item', r.success ? 'success' : 'failed']"
          >
            <div class="receipt-top">
              <span :class="['badge', r.success ? 'badge-success' : 'badge-danger']">
                {{ r.success ? 'DISPATCHED' : 'FAILED' }}
              </span>
              <span class="receipt-time font-mono">{{ r.time }}</span>
            </div>
            <div class="receipt-target font-mono">
              To: <strong>{{ r.to }}</strong> (via {{ r.sender }})
            </div>
            <div class="receipt-body font-mono">{{ r.text }}</div>
            <div v-if="r.error" class="receipt-error font-mono">Error: {{ r.error }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { api } from "../api";
import type { SessionInfo } from "../types";

const props = defineProps<{
  sessions: SessionInfo[];
}>();

const emit = defineEmits<{
  (e: "toast", msg: string, type?: "success" | "error"): void;
}>();

const mode = ref<"direct" | "group">("direct");
const selectedNumberId = ref("");
const recipient = ref("");
const messageText = ref("");
const sending = ref(false);

interface Receipt {
  time: string;
  sender: string;
  to: string;
  text: string;
  success: boolean;
  error?: string;
}

const receipts = ref<Receipt[]>([]);

const connectedSessions = computed(() => {
  return props.sessions.filter((s) => s.connected);
});

const canSend = computed(() => {
  return (
    !!selectedNumberId.value &&
    !!recipient.value.trim() &&
    !!messageText.value.trim()
  );
});

const templates = [
  { label: "OTP Verification", text: "Your Febran verification code is: *839120*. Do not share this code with anyone." },
  { label: "Welcome Greeting", text: "Hello! Thank you for contacting our customer service. How can we help you today?" },
  { label: "Order Confirmed", text: "📦 Your order #FBR-9841 has been confirmed and is being prepared for shipment!" },
];

const applyTemplate = (text: string) => {
  messageText.value = text;
};

const sendMessage = async () => {
  if (!canSend.value || sending.value) return;

  sending.value = true;
  const now = new Date().toLocaleTimeString();
  const target = recipient.value.trim();
  const text = messageText.value.trim();
  const sender = selectedNumberId.value;

  try {
    if (mode.value === "direct") {
      await api.sendText(sender, target, text);
    } else {
      await api.sendGroup(sender, target, text);
    }

    receipts.value.unshift({
      time: now,
      sender,
      to: target,
      text,
      success: true,
    });
    emit("toast", "Message sent successfully!", "success");
    messageText.value = "";
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : String(err);
    receipts.value.unshift({
      time: now,
      sender,
      to: target,
      text,
      success: false,
      error: errorMsg,
    });
    emit("toast", `Failed to send: ${errorMsg}`, "error");
  } finally {
    sending.value = false;
  }
};
</script>

<style scoped>
.sandbox-view {
  padding: 1.5rem;
  overflow-y: auto;
  height: calc(100vh - 65px);
}

.sandbox-layout {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 1.25rem;
}

.composer-card, .receipts-card {
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}

.card-subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.form-grid {
  display: flex;
  flex-direction: column;
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
  color: var(--text-secondary);
}

.mode-toggles {
  display: flex;
  gap: 0.5rem;
}

.toggle-btn {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  padding: 0.5rem;
  font-size: 0.8rem;
}

.toggle-btn.active {
  background: var(--bg-surface-elevated);
  border-color: #10b981;
  color: #34d399;
  font-weight: 600;
}

.input-hint {
  font-size: 0.72rem;
  color: var(--text-muted);
}

.text-warning {
  color: #fbbf24;
}

.template-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.chip-btn {
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 0.72rem;
  padding: 0.3rem 0.6rem;
  border-radius: var(--radius-sm);
}

.chip-btn:hover {
  border-color: var(--border-hover);
  color: var(--text-primary);
}

.send-btn {
  width: 100%;
  padding: 0.75rem;
  font-size: 0.9rem;
}

.empty-receipts {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  text-align: center;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px dashed var(--border-subtle);
  gap: 0.5rem;
}

.empty-icon {
  font-size: 2rem;
}

.empty-text {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.receipts-list {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  max-height: 480px;
  overflow-y: auto;
}

.receipt-item {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.75rem;
  background: var(--bg-input);
  border-radius: var(--radius-md);
  border-left: 3px solid transparent;
}

.receipt-item.success {
  border-left-color: #10b981;
}

.receipt-item.failed {
  border-left-color: #ef4444;
}

.receipt-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.receipt-time {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.receipt-target {
  font-size: 0.8rem;
  color: var(--text-primary);
}

.receipt-body {
  font-size: 0.75rem;
  color: var(--text-secondary);
  background: var(--bg-surface-elevated);
  padding: 0.4rem 0.6rem;
  border-radius: var(--radius-sm);
  white-space: pre-wrap;
}

.receipt-error {
  font-size: 0.72rem;
  color: #f87171;
}
</style>

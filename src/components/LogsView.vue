<template>
  <div class="logs-view">
    <div class="glass-panel terminal-container">
      <div class="terminal-header">
        <div class="terminal-left">
          <div class="mac-dots">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
          </div>
          <span class="terminal-title">Daemon Live Console Output</span>
          <span class="badge badge-neutral font-mono">{{ filteredLogs.length }} lines</span>
        </div>

        <div class="terminal-controls">
          <!-- Search input -->
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="Filter logs..." 
            class="filter-input font-mono"
          />

          <!-- Auto-scroll toggle -->
          <button 
            :class="['btn-secondary btn-sm', { active: autoScroll }]" 
            @click="autoScroll = !autoScroll"
          >
            Auto-Scroll: {{ autoScroll ? 'ON' : 'OFF' }}
          </button>

          <!-- Clear -->
          <button class="btn-secondary btn-sm" @click="$emit('clear-logs')">
            Clear
          </button>

          <!-- Copy -->
          <button class="btn-secondary btn-sm" @click="copyLogs">
            Copy
          </button>
        </div>
      </div>

      <div ref="terminalBodyRef" class="terminal-body font-mono">
        <div v-if="filteredLogs.length === 0" class="empty-logs">
          <span>Waiting for gateway logs...</span>
        </div>
        <div 
          v-for="(l, idx) in filteredLogs" 
          :key="idx" 
          :class="['log-line', l.level]"
        >
          <span class="log-time">{{ l.timestamp }}</span>
          <span :class="['log-level', l.level]">[{{ l.level.toUpperCase() }}]</span>
          <span class="log-text">{{ l.message }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import type { LogMessage } from "../types";

const props = defineProps<{
  logs: LogMessage[];
}>();

const emit = defineEmits<{
  (e: "clear-logs"): void;
  (e: "toast", msg: string, type?: "success" | "error"): void;
}>();

const searchQuery = ref("");
const autoScroll = ref(true);
const terminalBodyRef = ref<HTMLElement | null>(null);

const filteredLogs = computed(() => {
  if (!searchQuery.value.trim()) return props.logs;
  const q = searchQuery.value.toLowerCase();
  return props.logs.filter((l) => l.message.toLowerCase().includes(q) || l.timestamp.includes(q));
});

watch(
  () => props.logs.length,
  () => {
    if (autoScroll.value) {
      nextTick(() => {
        if (terminalBodyRef.value) {
          terminalBodyRef.value.scrollTop = terminalBodyRef.value.scrollHeight;
        }
      });
    }
  }
);

const copyLogs = () => {
  const text = props.logs.map((l) => `[${l.timestamp}] [${l.level}] ${l.message}`).join("\n");
  navigator.clipboard.writeText(text);
  emit("toast", "Logs copied to clipboard", "success");
};
</script>

<style scoped>
.logs-view {
  padding: 1.5rem;
  height: calc(100vh - 65px);
  display: flex;
  flex-direction: column;
}

.terminal-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  background: #060911;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  background: #0b1120;
  border-bottom: 1px solid var(--border-subtle);
}

.terminal-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.mac-dots {
  display: flex;
  gap: 6px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.dot.red { background: #ef4444; }
.dot.yellow { background: #f59e0b; }
.dot.green { background: #10b981; }

.terminal-title {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.terminal-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.filter-input {
  width: 160px;
  padding: 0.3rem 0.6rem;
  font-size: 0.75rem;
}

.terminal-body {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
  font-size: 0.8rem;
  line-height: 1.6;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  background: #070b14;
}

.empty-logs {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
}

.log-line {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
  word-break: break-all;
}

.log-time {
  color: #64748b;
  font-size: 0.72rem;
  flex-shrink: 0;
}

.log-level {
  font-weight: 600;
  font-size: 0.72rem;
  flex-shrink: 0;
}

.log-level.stdout { color: #38bdf8; }
.log-level.stderr { color: #f87171; }
.log-level.error { color: #ef4444; }
.log-level.info { color: #34d399; }

.log-text {
  color: #e2e8f0;
}
</style>

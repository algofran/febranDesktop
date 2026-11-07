<template>
  <div class="settings-view">
    <div class="glass-panel settings-card">
      <div class="settings-header">
        <div>
          <h2 class="settings-title">{{ t('settings.title') }}</h2>
          <p class="settings-subtitle">{{ t('settings.subtitle') }}</p>
        </div>
        <button class="btn-primary" :disabled="saving" @click="save">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
            <polyline points="17 21 17 13 7 13 7 21"/>
            <polyline points="7 3 7 8 15 8"/>
          </svg>
          <span>{{ saving ? t('settings.saving') : t('settings.save') }}</span>
        </button>
      </div>

      <div class="settings-grid">
        <!-- Language -->
        <div class="section-group">
          <h3 class="group-title">{{ t('settings.languageTitle') }}</h3>
          <div class="form-row">
            <div class="form-group">
              <label class="form-label">{{ t('settings.language') }}</label>
              <select :value="locale" @change="setLocale(($event.target as HTMLSelectElement).value as Locale)">
                <option v-for="l in locales" :key="l.code" :value="l.code">{{ l.label }}</option>
              </select>
              <span class="form-hint">{{ t('settings.languageHint') }}</span>
            </div>
          </div>
        </div>

        <!-- Gateway Section -->
        <div class="section-group">
          <h3 class="group-title">{{ t('settings.gatewayTitle') }}</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">{{ t('settings.bindLabel') }} (GATEWAY_BIND)</label>
              <input v-model="form.gateway_bind" type="text" class="font-mono" />
              <span class="form-hint">{{ t('settings.bindHint') }}</span>
            </div>

            <div class="form-group">
              <label class="form-label">{{ t('settings.apiKeyLabel') }} (GATEWAY_API_KEY)</label>
              <div class="input-with-action">
                <input v-model="form.gateway_api_key" type="text" class="font-mono" />
                <button class="btn-secondary btn-sm" @click="generateKey">{{ t('settings.generate') }}</button>
              </div>
              <span class="form-hint">{{ t('settings.apiKeyHintBefore') }} <code>X-Gateway-Key</code> {{ t('settings.apiKeyHintAfter') }}</span>
            </div>
          </div>
        </div>

        <!-- Central Webhook Section -->
        <div class="section-group">
          <h3 class="group-title">{{ t('settings.centralTitle') }}</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">{{ t('settings.serverKeyLabel') }} (GATEWAY_SERVER_KEY)</label>
              <div class="input-with-action">
                <input v-model="form.gateway_server_key" type="text" class="font-mono" />
                <button class="btn-secondary btn-sm" @click="generateServerKey">{{ t('settings.generate') }}</button>
              </div>
              <span class="form-hint">{{ t('settings.serverKeyHintBefore') }} <code>server_key</code> {{ t('settings.serverKeyHintAfter') }}</span>
            </div>

            <div class="form-group">
              <label class="form-label">{{ t('settings.centralKeyLabel') }} (CENTRAL_API_KEY)</label>
              <input v-model="form.central_api_key" type="text" class="font-mono" />
              <span class="form-hint">{{ t('settings.centralKeyHint') }}</span>
            </div>
          </div>
        </div>

        <!-- Storage & System -->
        <div class="section-group">
          <h3 class="group-title">{{ t('settings.storageTitle') }}</h3>

          <div class="form-row">
            <div class="form-group">
              <label class="form-label">{{ t('settings.sessionDirLabel') }} (SESSION_DIR)</label>
              <input v-model="form.session_dir" type="text" class="font-mono" />
              <span class="form-hint">{{ t('settings.sessionDirHint') }}</span>
            </div>

            <div class="form-group">
              <label class="form-label">{{ t('settings.logLevelLabel') }} (LOG_LEVEL)</label>
              <select v-model="form.log_level">
                <option value="debug">{{ t('settings.logDebug') }}</option>
                <option value="info">{{ t('settings.logInfo') }}</option>
                <option value="warn">{{ t('settings.logWarn') }}</option>
                <option value="error">{{ t('settings.logError') }}</option>
              </select>
              <span class="form-hint">{{ t('settings.logLevelHint') }}</span>
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
import { t, useI18n } from "../i18n";
import type { Locale } from "../locales";

const { locale, setLocale, locales } = useI18n();

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
  emit("toast", t("settings.toastApiKey"));
};

const generateServerKey = () => {
  const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  let key = "node-";
  for (let i = 0; i < 16; i++) {
    key += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  form.value.gateway_server_key = key;
  emit("toast", t("settings.toastServerKey"));
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
  color: #FDB000;
}

.input-with-action {
  display: flex;
  gap: 0.5rem;
}
</style>

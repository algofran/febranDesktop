import { reactive } from "vue";
import { api } from "./api";
import type { ServiceState } from "./types";

// Shared (module-scope) service state, updated by ServicePanel and Dashboard.
const svc = reactive<{ installed: boolean; state: ServiceState }>({
  installed: false,
  state: "not_installed",
});

export function setServiceState(installed: boolean, state: ServiceState) {
  svc.installed = installed;
  svc.state = state;
}

async function refreshServiceState() {
  try {
    const info = await api.serviceStatus();
    setServiceState(info.installed, info.state);
  } catch {
    // keep last known state
  }
}

export function useServiceState() {
  return { svc, refreshServiceState, setServiceState };
}

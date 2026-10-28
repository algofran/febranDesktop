export interface GatewayConfig {
  gateway_bind: string;
  gateway_api_key: string;
  central_webhook_url: string;
  central_ws_url: string;
  gateway_server_key: string;
  central_api_key: string;
  session_dir: string;
  log_level: string;
}

export interface ProcessStatus {
  running: boolean;
  pid: number | null;
  bind: string;
  is_healthy: boolean;
  ws_connected: boolean;
}

export interface LogMessage {
  level: string;
  message: string;
  timestamp: string;
}

export interface GroupInfo {
  JID?: string;
  jid?: string;
  Name?: string;
  name?: string;
  Topic?: string;
  topic?: string;
  ParticipantCount?: number;
  participant_count?: number;
  member_count?: number;
}

export interface ContactInfo {
  jid: string;
  phone: string;
  name: string;
}

export interface SessionSyncData {
  number_id: string;
  phone?: string;
  contacts: ContactInfo[];
  groups: GroupInfo[];
  error?: string;
}

export interface AllSyncResult {
  total_sessions: number;
  total_contacts: number;
  total_groups: number;
  results: SessionSyncData[];
}

export interface SessionInfo {
  number_id: string;
  connected: boolean;
  paired: boolean;
  db_exists?: boolean;
  qr_code?: string;
  pairing_code?: string;
  pairing_key?: string;
  last_activity?: string;
}

export interface SendMessagePayload {
  number_id: string;
  to: string;
  text: string;
}

export interface SendGroupMessagePayload {
  number_id: string;
  group_jid: string;
  text: string;
}

export type ServiceState =
  | "running"
  | "stopped"
  | "start_pending"
  | "stop_pending"
  | "continue_pending"
  | "pause_pending"
  | "paused"
  | "not_installed"
  | "unknown";

export interface ServiceInfo {
  ok: boolean;
  message: string;
  installed: boolean;
  state: ServiceState;
  data_dir: string;
  log_file: string;
}

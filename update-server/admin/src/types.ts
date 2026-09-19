export interface PatchChannelSummary {
  name: string;
  asset: string;
  patch_size: number;
  full_size: number;
  saving_percent: number;
}

export interface ReleaseObjectSummary {
  version: string;
  uploaded_at: string;
  patches: Array<{
    source_version: string;
    channels: PatchChannelSummary[];
  }>;
}

export interface VersionSummary {
  version: string;
  update_available: number;
  download_started: number;
  download_verified: number;
  install_success: number;
  install_failed: number;
  download_requests: number;
  requested_bytes: number;
  devices: number;
  last_event_at: string;
}

export interface DashboardData {
  generated_at: string;
  active_version: string | null;
  totals: {
    events: number;
    successful_updates: number;
    failed_updates: number;
    updated_installations: number;
    download_requests: number;
    requested_bytes: number;
  };
  daily_updates: Array<{ date: string; count: number }>;
  versions: VersionSummary[];
  failures: Array<{ code: string; reason: string; count: number }>;
  releases: ReleaseObjectSummary[];
  invalid_manifests: string[];
}

export interface ReleaseEvent {
  event_id: string;
  transaction_id: string | null;
  installation_id: string | null;
  event_type: EventType;
  current_version: string | null;
  target_version: string;
  channel: string;
  architecture: string;
  build_mode: string;
  artifact: string | null;
  transferred_bytes: number | null;
  failure_code: string | null;
  failure_reason: string | null;
  client_time: string | null;
  created_at: string;
}

export type EventType = "update_available" | "download_started" | "download_verified" | "install_success" | "install_failed";

export interface ReleaseFilters {
  page?: number;
  page_size?: number;
  event_type?: EventType;
  channel?: string;
  architecture?: string;
  build_mode?: string;
  failure_code?: string;
  failure_reason?: string;
  from?: string;
  to?: string;
}

export interface ReleaseDetailData {
  generated_at: string;
  version: string;
  filters: ReleaseFilters;
  summary: {
    events: number;
    devices: number;
    update_available: number;
    download_started: number;
    download_verified: number;
    install_success: number;
    install_failed: number;
    transferred_bytes: number;
    first_event_at: string | null;
    last_event_at: string | null;
  };
  dimensions: {
    channels: Dimension[];
    architectures: Dimension[];
    build_modes: Dimension[];
    failure_codes: Dimension[];
  };
  failure_groups: Array<{
    code: string;
    reason: string;
    count: number;
    last_seen_at: string;
  }>;
  events: ReleaseEvent[];
  pagination: {
    page: number;
    page_size: number;
    total: number;
  };
  release: ReleaseObjectSummary | null;
}

export interface Dimension {
  value: string;
  count: number;
}

export type VersionStatus = "all" | "healthy" | "failed" | "active";

import { readReleaseManifest, type ReleaseObjectSummary } from "./dashboard";
import { isSafeVersion, UPDATE_EVENT_TYPES, type UpdateEventType } from "./validation";

const DEFAULT_PAGE_SIZE = 20;
const MAX_PAGE_SIZE = 100;
const FILTER_VALUE_PATTERN = /^[A-Za-z0-9._+-]{1,128}$/;

interface ReleaseEventRow {
  event_id: string;
  transaction_id: string | null;
  installation_id: string | null;
  event_type: string;
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

interface ReleaseSummaryRow {
  events?: number;
  devices?: number;
  update_available?: number;
  download_started?: number;
  download_verified?: number;
  install_success?: number;
  install_failed?: number;
  transferred_bytes?: number;
  first_event_at?: string | null;
  last_event_at?: string | null;
}

interface FailureGroupRow {
  failure_code?: string;
  failure_reason?: string;
  count?: number;
  last_seen_at?: string;
}

interface DimensionRow {
  value?: string;
  count?: number;
}

export interface ReleaseFilters {
  page: number;
  page_size: number;
  event_type?: UpdateEventType;
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
    channels: Array<{ value: string; count: number }>;
    architectures: Array<{ value: string; count: number }>;
    build_modes: Array<{ value: string; count: number }>;
    failure_codes: Array<{ value: string; count: number }>;
  };
  failure_groups: Array<{
    code: string;
    reason: string;
    count: number;
    last_seen_at: string;
  }>;
  events: ReleaseEventRow[];
  pagination: {
    page: number;
    page_size: number;
    total: number;
  };
  release: ReleaseObjectSummary | null;
}

export function parseReleaseFilters(url: URL): ReleaseFilters {
  const page = positiveInteger(url.searchParams.get("page"), 1);
  const pageSize = Math.min(positiveInteger(url.searchParams.get("page_size"), DEFAULT_PAGE_SIZE), MAX_PAGE_SIZE);
  const eventTypeValue = optionalFilter(url.searchParams.get("event_type"));
  const eventType = eventTypeValue && UPDATE_EVENT_TYPES.includes(eventTypeValue as UpdateEventType)
    ? eventTypeValue as UpdateEventType
    : undefined;

  return {
    page,
    page_size: pageSize,
    event_type: eventType,
    channel: optionalFilter(url.searchParams.get("channel")),
    architecture: optionalFilter(url.searchParams.get("architecture")),
    build_mode: optionalFilter(url.searchParams.get("build_mode")),
    failure_code: optionalFilter(url.searchParams.get("failure_code")),
    failure_reason: optionalFilter(url.searchParams.get("failure_reason")),
    from: optionalTimestamp(url.searchParams.get("from")),
    to: optionalTimestamp(url.searchParams.get("to")),
  };
}

export async function loadReleaseDetails(
  db: D1Database,
  bucket: R2Bucket,
  version: string,
  filters: ReleaseFilters,
): Promise<ReleaseDetailData> {
  if (!isSafeVersion(version))
    throw new Error("invalid_version");

  const filtered = buildWhere(version, filters, true);
  const failureFiltered = buildWhere(version, filters, false);
  const offset = (filters.page - 1) * filters.page_size;

  const [summaryResult, eventsResult, failureResult, channelResult, architectureResult, buildModeResult, failureCodeResult, release] = await Promise.all([
    db.prepare(`
      SELECT
        COUNT(*) AS events,
        COUNT(DISTINCT COALESCE(NULLIF(installation_id, ''), NULLIF(transaction_id, ''))) AS devices,
        COALESCE(SUM(event_type = 'update_available'), 0) AS update_available,
        COALESCE(SUM(event_type = 'download_started'), 0) AS download_started,
        COALESCE(SUM(event_type = 'download_verified'), 0) AS download_verified,
        COALESCE(SUM(event_type = 'install_success'), 0) AS install_success,
        COALESCE(SUM(event_type = 'install_failed'), 0) AS install_failed,
        COALESCE(SUM(transferred_bytes), 0) AS transferred_bytes,
        MIN(created_at) AS first_event_at,
        MAX(created_at) AS last_event_at
      FROM update_events
      WHERE ${filtered.sql}
    `).bind(...filtered.bindings).first<ReleaseSummaryRow>(),
    db.prepare(`
      SELECT event_id, transaction_id, installation_id, event_type, current_version,
        target_version, channel, architecture, build_mode, artifact, transferred_bytes,
        failure_code, failure_reason, client_time, created_at
      FROM update_events
      WHERE ${filtered.sql}
      ORDER BY created_at DESC, event_id DESC
      LIMIT ? OFFSET ?
    `).bind(...filtered.bindings, filters.page_size, offset).all<ReleaseEventRow>(),
    db.prepare(`
      SELECT COALESCE(failure_code, 'unknown') AS failure_code,
        COALESCE(failure_reason, 'unknown') AS failure_reason,
        COUNT(*) AS count,
        MAX(created_at) AS last_seen_at
      FROM update_events
      WHERE ${failureFiltered.sql} AND event_type = 'install_failed'
      GROUP BY COALESCE(failure_code, 'unknown'), COALESCE(failure_reason, 'unknown')
      ORDER BY count DESC, last_seen_at DESC
      LIMIT 50
    `).bind(...failureFiltered.bindings).all<FailureGroupRow>(),
    loadDimension(db, version, "channel"),
    loadDimension(db, version, "architecture"),
    loadDimension(db, version, "build_mode"),
    loadDimension(db, version, "failure_code", "event_type = 'install_failed'"),
    readReleaseManifest(bucket, version),
  ]);

  const summary = summaryResult ?? {};
  const total = numberValue(summary.events);
  return {
    generated_at: new Date().toISOString(),
    version,
    filters,
    summary: {
      events: total,
      devices: numberValue(summary.devices),
      update_available: numberValue(summary.update_available),
      download_started: numberValue(summary.download_started),
      download_verified: numberValue(summary.download_verified),
      install_success: numberValue(summary.install_success),
      install_failed: numberValue(summary.install_failed),
      transferred_bytes: numberValue(summary.transferred_bytes),
      first_event_at: stringOrNull(summary.first_event_at),
      last_event_at: stringOrNull(summary.last_event_at),
    },
    dimensions: {
      channels: normalizeDimensions(channelResult.results),
      architectures: normalizeDimensions(architectureResult.results),
      build_modes: normalizeDimensions(buildModeResult.results),
      failure_codes: normalizeDimensions(failureCodeResult.results),
    },
    failure_groups: failureResult.results.map(row => ({
      code: stringValue(row.failure_code) || "unknown",
      reason: stringValue(row.failure_reason) || "unknown",
      count: numberValue(row.count),
      last_seen_at: stringValue(row.last_seen_at),
    })),
    events: eventsResult.results,
    pagination: {
      page: filters.page,
      page_size: filters.page_size,
      total,
    },
    release,
  };
}

function buildWhere(version: string, filters: ReleaseFilters, includeEventType: boolean): {
  sql: string;
  bindings: Array<string | number>;
} {
  const clauses = ["target_version = ?"];
  const bindings: Array<string | number> = [version];
  const equalsFilters: Array<[string, string | undefined]> = [
    ["channel", filters.channel],
    ["architecture", filters.architecture],
    ["build_mode", filters.build_mode],
    ["COALESCE(failure_code, 'unknown')", filters.failure_code],
    ["COALESCE(failure_reason, 'unknown')", filters.failure_reason],
  ];
  if (includeEventType)
    equalsFilters.unshift(["event_type", filters.event_type]);
  for (const [column, value] of equalsFilters) {
    if (!value)
      continue;
    clauses.push(`${column} = ?`);
    bindings.push(value);
  }
  if (filters.from) {
    clauses.push("created_at >= ?");
    bindings.push(filters.from);
  }
  if (filters.to) {
    clauses.push("created_at <= ?");
    bindings.push(filters.to);
  }
  return { sql: clauses.join(" AND "), bindings };
}

async function loadDimension(
  db: D1Database,
  version: string,
  column: "channel" | "architecture" | "build_mode" | "failure_code",
  extraWhere?: string,
): Promise<D1Result<DimensionRow>> {
  const condition = extraWhere ? ` AND ${extraWhere}` : "";
  return db.prepare(`
    SELECT COALESCE(${column}, 'unknown') AS value, COUNT(*) AS count
    FROM update_events
    WHERE target_version = ?${condition}
    GROUP BY COALESCE(${column}, 'unknown')
    ORDER BY count DESC, value
  `).bind(version).all<DimensionRow>();
}

function normalizeDimensions(rows: DimensionRow[]): Array<{ value: string; count: number }> {
  return rows.map(row => ({ value: stringValue(row.value) || "unknown", count: numberValue(row.count) }));
}

function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function optionalFilter(value: string | null): string | undefined {
  const normalized = value?.trim();
  return normalized && FILTER_VALUE_PATTERN.test(normalized) ? normalized : undefined;
}

function optionalTimestamp(value: string | null): string | undefined {
  const normalized = value?.trim();
  if (!normalized || normalized.length > 64 || Number.isNaN(Date.parse(normalized)))
    return undefined;
  return new Date(normalized).toISOString();
}

function numberValue(value: unknown): number {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function stringOrNull(value: unknown): string | null {
  return typeof value === "string" && value ? value : null;
}

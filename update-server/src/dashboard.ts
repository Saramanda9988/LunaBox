interface DashboardSQLRow {
  row_kind: string;
  total_events?: number;
  successful_updates?: number;
  failed_updates?: number;
  updated_installations?: number;
  requests?: number;
  requested_bytes?: number;
  date?: string;
  version?: string;
  update_available?: number;
  download_started?: number;
  download_verified?: number;
  install_success?: number;
  install_failed?: number;
  devices?: number;
  last_event_at?: string;
  failure_code?: string;
  failure_reason?: string;
  count?: number;
}

interface ManifestArtifact {
  size: number;
}

interface ManifestPatch extends ManifestArtifact {
  sourceVersion: string;
}

interface ManifestFile {
  path: string;
  full: ManifestArtifact;
  patch?: ManifestPatch;
}

interface ParsedManifest {
  version: string;
  channels: Array<{
    name: string;
    files: ManifestFile[];
  }>;
}

export interface PatchChannelSummary {
  name: string;
  asset: string;
  patch_size: number;
  full_size: number;
  saving_percent: number;
}

export interface PatchRelationSummary {
  source_version: string;
  channels: PatchChannelSummary[];
}

export interface ReleaseObjectSummary {
  version: string;
  uploaded_at: string;
  patches: PatchRelationSummary[];
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
  versions: Array<{
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
  }>;
  failures: Array<{ code: string; reason: string; count: number }>;
  releases: ReleaseObjectSummary[];
  invalid_manifests: string[];
}

export async function loadDashboard(db: D1Database, bucket: R2Bucket): Promise<DashboardData> {
  const [databaseResults, releaseScan, activeVersion] = await Promise.all([
    loadDatabaseStats(db),
    scanReleaseManifests(bucket),
    readActiveVersion(bucket),
  ]);
  const [summaryResult, dailyResult, versionResult, downloadResult, failureResult] = databaseResults;
  const summary = summaryResult.results[0];
  const downloads = downloadResult.results[0];
  const downloadByVersion = new Map(
    downloadResult.results.slice(1).map(row => [stringValue(row.version), row]),
  );
  const versions: DashboardData["versions"] = versionResult.results.map(row => {
    const version = stringValue(row.version);
    const versionDownloads = downloadByVersion.get(version);
    return {
      version,
      update_available: numberValue(row.update_available),
      download_started: numberValue(row.download_started),
      download_verified: numberValue(row.download_verified),
      install_success: numberValue(row.install_success),
      install_failed: numberValue(row.install_failed),
      download_requests: numberValue(versionDownloads?.requests),
      requested_bytes: numberValue(versionDownloads?.requested_bytes),
      devices: numberValue(row.devices),
      last_event_at: stringValue(row.last_event_at),
    };
  });
  const versionsWithEvents = new Set(versions.map(row => row.version));
  for (const release of releaseScan.releases) {
    if (versionsWithEvents.has(release.version))
      continue;
    const versionDownloads = downloadByVersion.get(release.version);
    versions.push({
      version: release.version,
      update_available: 0,
      download_started: 0,
      download_verified: 0,
      install_success: 0,
      install_failed: 0,
      download_requests: numberValue(versionDownloads?.requests),
      requested_bytes: numberValue(versionDownloads?.requested_bytes),
      devices: 0,
      last_event_at: release.uploaded_at,
    });
  }
  versions.sort((left, right) => right.last_event_at.localeCompare(left.last_event_at));

  return {
    generated_at: new Date().toISOString(),
    active_version: activeVersion,
    totals: {
      events: numberValue(summary?.total_events),
      successful_updates: numberValue(summary?.successful_updates),
      failed_updates: numberValue(summary?.failed_updates),
      updated_installations: numberValue(summary?.updated_installations),
      download_requests: numberValue(downloads?.requests),
      requested_bytes: numberValue(downloads?.requested_bytes),
    },
    daily_updates: dailyResult.results.map(row => ({
      date: stringValue(row.date),
      count: numberValue(row.count),
    })),
    versions,
    failures: failureResult.results.map(row => ({
      code: stringValue(row.failure_code) || "unknown",
      reason: stringValue(row.failure_reason),
      count: numberValue(row.count),
    })),
    releases: releaseScan.releases,
    invalid_manifests: releaseScan.invalidManifests,
  };
}

async function loadDatabaseStats(db: D1Database): Promise<D1Result<DashboardSQLRow>[]> {
  return db.batch<DashboardSQLRow>([
    db.prepare(`
      SELECT
        'summary' AS row_kind,
        COUNT(*) AS total_events,
        COALESCE(SUM(event_type = 'install_success'), 0) AS successful_updates,
        COALESCE(SUM(event_type = 'install_failed'), 0) AS failed_updates,
        COUNT(DISTINCT COALESCE(NULLIF(installation_id, ''), NULLIF(transaction_id, ''))) AS updated_installations
      FROM update_events
    `),
    db.prepare(`
      SELECT 'daily' AS row_kind, DATE(created_at) AS date, COUNT(*) AS count
      FROM update_events
      WHERE event_type = 'install_success'
        AND created_at >= DATETIME('now', '-29 days')
      GROUP BY DATE(created_at)
      ORDER BY date
    `),
    db.prepare(`
      SELECT
        'version' AS row_kind,
        target_version AS version,
        COALESCE(SUM(event_type = 'update_available'), 0) AS update_available,
        COALESCE(SUM(event_type = 'download_started'), 0) AS download_started,
        COALESCE(SUM(event_type = 'download_verified'), 0) AS download_verified,
        COALESCE(SUM(event_type = 'install_success'), 0) AS install_success,
        COALESCE(SUM(event_type = 'install_failed'), 0) AS install_failed,
        COUNT(DISTINCT COALESCE(NULLIF(installation_id, ''), NULLIF(transaction_id, ''))) AS devices,
        MAX(created_at) AS last_event_at
      FROM update_events
      GROUP BY target_version
      ORDER BY MAX(created_at) DESC
    `),
    db.prepare(`
      SELECT 'downloads_total' AS row_kind, NULL AS version,
        COALESCE(SUM(request_count), 0) AS requests,
        COALESCE(SUM(requested_bytes), 0) AS requested_bytes
      FROM download_requests
      UNION ALL
      SELECT 'downloads_version' AS row_kind, version,
        COALESCE(SUM(request_count), 0) AS requests,
        COALESCE(SUM(requested_bytes), 0) AS requested_bytes
      FROM download_requests
      GROUP BY version
    `),
    db.prepare(`
      SELECT 'failure' AS row_kind, COALESCE(failure_code, 'unknown') AS failure_code,
        COALESCE(failure_reason, '') AS failure_reason, COUNT(*) AS count
      FROM update_events
      WHERE event_type = 'install_failed'
      GROUP BY COALESCE(failure_code, 'unknown'), COALESCE(failure_reason, '')
      ORDER BY count DESC
      LIMIT 10
    `),
  ]);
}

async function scanReleaseManifests(bucket: R2Bucket): Promise<{
  releases: ReleaseObjectSummary[];
  invalidManifests: string[];
}> {
  const versions = await listReleaseVersions(bucket);
  const releases: ReleaseObjectSummary[] = [];
  const invalidManifests: string[] = [];

  for (let offset = 0; offset < versions.length; offset += 10) {
    const batch = versions.slice(offset, offset + 10);
    const results = await Promise.all(batch.map(version => readReleaseManifest(bucket, version)));
    for (let index = 0; index < results.length; index++) {
      const result = results[index];
      if (result)
        releases.push(result);
      else
        invalidManifests.push(batch[index]);
    }
  }

  releases.sort((left, right) => right.uploaded_at.localeCompare(left.uploaded_at));
  return { releases, invalidManifests };
}

async function listReleaseVersions(bucket: R2Bucket): Promise<string[]> {
  const versions = new Set<string>();
  let cursor: string | undefined;

  do {
    const listed = await bucket.list({
      prefix: "releases/",
      delimiter: "/",
      limit: 1000,
      cursor,
    });
    for (const prefix of listed.delimitedPrefixes) {
      const match = prefix.match(/^releases\/([^/]+)\/$/);
      if (match)
        versions.add(decodeURIComponent(match[1]));
    }
    cursor = listed.truncated ? listed.cursor : undefined;
  } while (cursor);

  return [...versions];
}

export async function readReleaseManifest(bucket: R2Bucket, version: string): Promise<ReleaseObjectSummary | null> {
  const object = await bucket.get(`releases/${version}/manifest.json`);
  if (!object)
    return null;

  try {
    const manifest = parseReleaseManifest(await object.json<unknown>());
    if (!manifest || manifest.version !== version)
      return null;
    return {
      version,
      uploaded_at: object.uploaded.toISOString(),
      patches: summarizePatches(manifest),
    };
  }
  catch {
    return null;
  }
}

async function readActiveVersion(bucket: R2Bucket): Promise<string | null> {
  const object = await bucket.get("channels/stable/version.json");
  if (!object)
    return null;
  try {
    const value = await object.json<unknown>();
    if (!isRecord(value) || typeof value.version !== "string")
      return null;
    return value.version;
  }
  catch {
    return null;
  }
}

export function parseReleaseManifest(value: unknown): ParsedManifest | null {
  if (!isRecord(value) || typeof value.version !== "string" || !isRecord(value.channels))
    return null;

  const channels: ParsedManifest["channels"] = [];
  for (const [name, channelValue] of Object.entries(value.channels)) {
    if (!isRecord(channelValue) || !Array.isArray(channelValue.files))
      return null;
    const files: ManifestFile[] = [];
    for (const fileValue of channelValue.files) {
      if (!isRecord(fileValue) || typeof fileValue.path !== "string" || !isArtifact(fileValue.full))
        return null;
      const file: ManifestFile = {
        path: fileValue.path,
        full: { size: fileValue.full.size },
      };
      if (fileValue.patch !== undefined) {
        if (!isArtifact(fileValue.patch) || typeof fileValue.patch.source_version !== "string")
          return null;
        file.patch = {
          size: fileValue.patch.size,
          sourceVersion: fileValue.patch.source_version,
        };
      }
      files.push(file);
    }
    channels.push({ name, files });
  }

  return { version: value.version, channels };
}

export function summarizePatches(manifest: ParsedManifest): PatchRelationSummary[] {
  const relations = new Map<string, PatchChannelSummary[]>();
  for (const channel of manifest.channels) {
    for (const file of channel.files) {
      if (!file.patch)
        continue;
      const items = relations.get(file.patch.sourceVersion) ?? [];
      items.push({
        name: channel.name,
        asset: file.path,
        patch_size: file.patch.size,
        full_size: file.full.size,
        saving_percent: file.full.size > 0
          ? Math.max(0, Math.round((1 - file.patch.size / file.full.size) * 1000) / 10)
          : 0,
      });
      relations.set(file.patch.sourceVersion, items);
    }
  }

  return [...relations.entries()]
    .map(([sourceVersion, channels]) => ({ source_version: sourceVersion, channels }))
    .sort((left, right) => right.source_version.localeCompare(left.source_version, undefined, { numeric: true }));
}

function isArtifact(value: unknown): value is Record<string, unknown> & { size: number } {
  return isRecord(value) && typeof value.size === "number" && Number.isFinite(value.size) && value.size > 0;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function numberValue(value: unknown): number {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

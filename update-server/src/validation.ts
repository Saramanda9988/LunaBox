export const UPDATE_EVENT_TYPES = [
  "update_available",
  "download_started",
  "download_verified",
  "install_success",
  "install_failed",
] as const;

export type UpdateEventType = (typeof UPDATE_EVENT_TYPES)[number];

export interface UpdateEvent {
  event_id: string;
  transaction_id?: string;
  installation_id?: string;
  event_type: UpdateEventType;
  current_version?: string;
  target_version: string;
  channel: string;
  architecture: string;
  build_mode: string;
  artifact?: string;
  transferred_bytes?: number;
  failure_code?: string;
  client_time?: string;
}

const identifierPattern = /^[A-Za-z0-9._+-]{1,128}$/;
const eventIDPattern = /^[A-Za-z0-9_-]{8,128}$/;
const assetPattern = /^[A-Za-z0-9][A-Za-z0-9._+-]{0,254}$/;

export function isSafeVersion(value: string): boolean {
  return identifierPattern.test(value);
}

export function isSafeChannel(value: string): boolean {
  return identifierPattern.test(value);
}

export function isSafeAssetName(value: string): boolean {
  return assetPattern.test(value) && !value.includes("..");
}

export function channelObjectKey(channel: string): string {
  if (!isSafeChannel(channel))
    throw new Error("invalid channel");
  return `channels/${channel}/version.json`;
}

export function manifestObjectKey(version: string): string {
  if (!isSafeVersion(version))
    throw new Error("invalid version");
  return `releases/${version}/manifest.json`;
}

export function versionObjectKey(version: string): string {
  if (!isSafeVersion(version))
    throw new Error("invalid version");
  return `releases/${version}/version.json`;
}

export function assetObjectKey(version: string, asset: string): string {
  if (!isSafeVersion(version) || !isSafeAssetName(asset))
    throw new Error("invalid asset key");
  return `releases/${version}/${asset}`;
}

export function parseUpdateEvent(value: unknown): UpdateEvent {
  if (!value || typeof value !== "object")
    throw new Error("event body must be an object");

  const event = value as Record<string, unknown>;
  const eventType = String(event.event_type ?? "");
  const targetVersion = String(event.target_version ?? "");
  const channel = String(event.channel ?? "");
  const architecture = String(event.architecture ?? "");
  const buildMode = String(event.build_mode ?? "");
  const eventID = String(event.event_id ?? "");

  if (!eventIDPattern.test(eventID))
    throw new Error("invalid event_id");
  if (!UPDATE_EVENT_TYPES.includes(eventType as UpdateEventType))
    throw new Error("invalid event_type");
  if (!isSafeVersion(targetVersion))
    throw new Error("invalid target_version");
  if (!isSafeChannel(channel))
    throw new Error("invalid channel");
  if (!identifierPattern.test(architecture) || !identifierPattern.test(buildMode))
    throw new Error("invalid platform fields");

  const transferredBytes = event.transferred_bytes === undefined
    ? undefined
    : Number(event.transferred_bytes);
  if (transferredBytes !== undefined && (!Number.isSafeInteger(transferredBytes) || transferredBytes < 0))
    throw new Error("invalid transferred_bytes");

  return {
    event_id: eventID,
    transaction_id: optionalIdentifier(event.transaction_id),
    installation_id: optionalIdentifier(event.installation_id),
    event_type: eventType as UpdateEventType,
    current_version: optionalVersion(event.current_version),
    target_version: targetVersion,
    channel,
    architecture,
    build_mode: buildMode,
    artifact: optionalAsset(event.artifact),
    transferred_bytes: transferredBytes,
    failure_code: optionalIdentifier(event.failure_code),
    client_time: optionalTimestamp(event.client_time),
  };
}

function optionalIdentifier(value: unknown): string | undefined {
  if (value === undefined || value === null || value === "")
    return undefined;
  const normalized = String(value);
  if (!identifierPattern.test(normalized))
    throw new Error("invalid identifier field");
  return normalized;
}

function optionalVersion(value: unknown): string | undefined {
  if (value === undefined || value === null || value === "")
    return undefined;
  const normalized = String(value);
  if (!isSafeVersion(normalized))
    throw new Error("invalid current_version");
  return normalized;
}

function optionalAsset(value: unknown): string | undefined {
  if (value === undefined || value === null || value === "")
    return undefined;
  const normalized = String(value);
  if (!isSafeAssetName(normalized))
    throw new Error("invalid artifact");
  return normalized;
}

function optionalTimestamp(value: unknown): string | undefined {
  if (value === undefined || value === null || value === "")
    return undefined;
  const normalized = String(value);
  if (normalized.length > 64 || Number.isNaN(Date.parse(normalized)))
    throw new Error("invalid client_time");
  return normalized;
}

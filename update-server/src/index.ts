import {
  assetObjectKey,
  channelObjectKey,
  isSafeAssetName,
  isSafeVersion,
  manifestObjectKey,
  parseUpdateEvent,
  type UpdateEvent,
  versionObjectKey,
} from "./validation";

interface Env {
  UPDATE_BUCKET: R2Bucket;
  UPDATE_DB: D1Database;
  ADMIN_TOKEN: string;
}

interface RouteContext {
  request: Request;
  env: Env;
  execution: ExecutionContext;
  url: URL;
}

export default {
  async fetch(request: Request, env: Env, execution: ExecutionContext): Promise<Response> {
    try {
      return await route({ request, env, execution, url: new URL(request.url) });
    }
    catch (error) {
      console.error(error);
      return json({ error: "internal_error" }, 500);
    }
  },
} satisfies ExportedHandler<Env>;

async function route(context: RouteContext): Promise<Response> {
  const { request, url } = context;
  if (request.method === "GET" && url.pathname === "/health")
    return json({ status: "ok" });

  if (request.method === "GET" && url.pathname === "/version.json")
    return serveObject(context, channelObjectKey("stable"), "public, max-age=60");

  const channelMatch = url.pathname.match(/^\/v1\/channels\/([^/]+)$/);
  if (request.method === "GET" && channelMatch)
    return serveObject(context, channelObjectKey(decodeURIComponent(channelMatch[1])), "public, max-age=60");

  const manifestMatch = url.pathname.match(/^\/v1\/releases\/([^/]+)\/manifest$/);
  if (request.method === "GET" && manifestMatch)
    return serveObject(context, manifestObjectKey(decodeURIComponent(manifestMatch[1])), "public, max-age=31536000, immutable");

  const versionMatch = url.pathname.match(/^\/v1\/releases\/([^/]+)\/version$/);
  if (request.method === "GET" && versionMatch)
    return serveObject(context, versionObjectKey(decodeURIComponent(versionMatch[1])), "public, max-age=31536000, immutable");

  const assetMatch = url.pathname.match(/^\/v1\/releases\/([^/]+)\/assets\/([^/]+)$/);
  if ((request.method === "GET" || request.method === "HEAD") && assetMatch) {
    const version = decodeURIComponent(assetMatch[1]);
    const asset = decodeURIComponent(assetMatch[2]);
    if (!isSafeVersion(version) || !isSafeAssetName(asset))
      return json({ error: "invalid_asset" }, 400);
    return serveAsset(context, version, asset);
  }

  if (request.method === "POST" && url.pathname === "/v1/events")
    return acceptEvent(context);

  const statsMatch = url.pathname.match(/^\/v1\/stats\/releases\/([^/]+)$/);
  if (request.method === "GET" && statsMatch)
    return releaseStats(context, decodeURIComponent(statsMatch[1]));

  return json({ error: "not_found" }, 404);
}

async function serveObject(context: RouteContext, key: string, cacheControl: string): Promise<Response> {
  const object = await context.env.UPDATE_BUCKET.get(key);
  if (!object)
    return json({ error: "not_found" }, 404);
  if (!("body" in object))
    return new Response(null, { status: 304, headers: objectHeaders(object, cacheControl) });

  const headers = objectHeaders(object, cacheControl);
  return new Response(object.body, { headers });
}

async function serveAsset(context: RouteContext, version: string, asset: string): Promise<Response> {
  const key = assetObjectKey(version, asset);
  const requestHeaders = context.request.headers;
  const object = await context.env.UPDATE_BUCKET.get(key, {
    range: requestHeaders,
    onlyIf: requestHeaders,
  });
  if (!object)
    return json({ error: "not_found" }, 404);
  if (!("body" in object))
    return new Response(null, { status: 304, headers: objectHeaders(object, "public, max-age=31536000, immutable") });

  const headers = objectHeaders(object, "public, max-age=31536000, immutable");
  headers.set("accept-ranges", "bytes");
  let status = 200;
  let requestedBytes = object.size;
  if (object.range) {
    const range = resolveRange(object.range, object.size);
    status = 206;
    requestedBytes = range.length;
    headers.set("content-range", `bytes ${range.offset}-${range.offset + range.length - 1}/${object.size}`);
    headers.set("content-length", String(range.length));
  }

  context.execution.waitUntil(recordDownloadRequest(context.env.UPDATE_DB, version, asset, requestedBytes));
  return new Response(context.request.method === "HEAD" ? null : object.body, { status, headers });
}

function resolveRange(range: R2Range, objectSize: number): { offset: number; length: number } {
  if ("suffix" in range) {
    const length = Math.min(range.suffix, objectSize);
    return { offset: objectSize - length, length };
  }
  const offset = range.offset ?? 0;
  return {
    offset,
    length: Math.min(range.length ?? objectSize - offset, objectSize - offset),
  };
}

function objectHeaders(object: R2Object, cacheControl: string): Headers {
  const headers = new Headers();
  object.writeHttpMetadata(headers);
  headers.set("etag", object.httpEtag);
  headers.set("cache-control", cacheControl);
  headers.set("x-content-type-options", "nosniff");
  return headers;
}

async function acceptEvent(context: RouteContext): Promise<Response> {
  if (!isJSONRequest(context.request))
    return json({ error: "content_type_required" }, 415);

  let event: UpdateEvent;
  try {
    event = parseUpdateEvent(await context.request.json());
  }
  catch (error) {
    return json({ error: "invalid_event", message: errorMessage(error) }, 400);
  }

  const result = await context.env.UPDATE_DB.prepare(`
    INSERT OR IGNORE INTO update_events (
      event_id, transaction_id, installation_id, event_type,
      current_version, target_version, channel, architecture, build_mode,
      artifact, transferred_bytes, failure_code, client_time
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `).bind(
    event.event_id,
    event.transaction_id ?? null,
    event.installation_id ?? null,
    event.event_type,
    event.current_version ?? null,
    event.target_version,
    event.channel,
    event.architecture,
    event.build_mode,
    event.artifact ?? null,
    event.transferred_bytes ?? null,
    event.failure_code ?? null,
    event.client_time ?? null,
  ).run();

  return json({ accepted: true, duplicate: (result.meta.changes ?? 0) === 0 }, 202);
}

async function releaseStats(context: RouteContext, version: string): Promise<Response> {
  if (!isSafeVersion(version))
    return json({ error: "invalid_version" }, 400);
  if (!await hasValidAdminToken(context.request, context.env.ADMIN_TOKEN))
    return json({ error: "unauthorized" }, 401);

  const [events, downloads, failures] = await context.env.UPDATE_DB.batch([
    context.env.UPDATE_DB.prepare(`
      SELECT event_type, COUNT(*) AS count
      FROM update_events
      WHERE target_version = ?
      GROUP BY event_type
      ORDER BY event_type
    `).bind(version),
    context.env.UPDATE_DB.prepare(`
      SELECT asset, SUM(request_count) AS requests, SUM(requested_bytes) AS requested_bytes
      FROM download_requests
      WHERE version = ?
      GROUP BY asset
      ORDER BY asset
    `).bind(version),
    context.env.UPDATE_DB.prepare(`
      SELECT COALESCE(failure_code, 'unknown') AS failure_code, COUNT(*) AS count
      FROM update_events
      WHERE target_version = ? AND event_type = 'install_failed'
      GROUP BY failure_code
      ORDER BY count DESC
      LIMIT 20
    `).bind(version),
  ]);

  return json({
    version,
    events: events.results,
    downloads: downloads.results,
    failures: failures.results,
  });
}

async function recordDownloadRequest(db: D1Database, version: string, asset: string, requestedBytes: number): Promise<void> {
  const date = new Date().toISOString().slice(0, 10);
  await db.prepare(`
    INSERT INTO download_requests (request_date, version, asset, request_count, requested_bytes)
    VALUES (?, ?, ?, 1, ?)
    ON CONFLICT(request_date, version, asset) DO UPDATE SET
      request_count = request_count + 1,
      requested_bytes = requested_bytes + excluded.requested_bytes
  `).bind(date, version, asset, requestedBytes).run();
}

async function hasValidAdminToken(request: Request, expected: string): Promise<boolean> {
  const authorization = request.headers.get("authorization") ?? "";
  const actual = authorization.startsWith("Bearer ") ? authorization.slice(7) : "";
  if (!expected || actual.length !== expected.length)
    return false;

  const [actualDigest, expectedDigest] = await Promise.all([
    crypto.subtle.digest("SHA-256", new TextEncoder().encode(actual)),
    crypto.subtle.digest("SHA-256", new TextEncoder().encode(expected)),
  ]);
  const left = new Uint8Array(actualDigest);
  const right = new Uint8Array(expectedDigest);
  let difference = 0;
  for (let index = 0; index < left.length; index++)
    difference |= left[index] ^ right[index];
  return difference === 0;
}

function isJSONRequest(request: Request): boolean {
  return (request.headers.get("content-type") ?? "").toLowerCase().startsWith("application/json");
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "invalid value";
}

function json(value: unknown, status = 200): Response {
  return Response.json(value, {
    status,
    headers: {
      "cache-control": "no-store",
      "x-content-type-options": "nosniff",
    },
  });
}

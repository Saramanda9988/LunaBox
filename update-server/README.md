# LunaBox Update Server

Cloudflare Worker update service backed by R2 and D1.

## Object layout

```text
channels/<channel>/version.json
releases/<version>/version.json
releases/<version>/manifest.json
releases/<version>/<asset>
```

Versioned release objects are immutable. A channel document is published last
and identifies the release whose manifest URL clients derive. `/version.json`
is an alias for the stable channel document.

## API

```text
GET  /health
GET  /version.json
GET  /v1/channels/<channel>
GET  /v1/releases/<version>/manifest
GET  /v1/releases/<version>/version
GET  /v1/releases/<version>/assets/<asset>
POST /v1/events
GET  /v1/stats/releases/<version>
GET  /v1/admin/dashboard
GET  /v1/admin/releases/<version>
GET  /admin
```

The statistics and dashboard API endpoints require
`Authorization: Bearer <ADMIN_TOKEN>`. The `/admin` page asks for the token and
keeps it in the current browser tab's session storage.

The React and Ant Design dashboard shows successful update events, anonymous
installation counts, failures, asset request volume, a 30-day update chart,
per-version telemetry, and patch source-to-target relationships read from each
R2 release manifest. A release detail view supports filtering events by status,
channel, architecture, build mode, failure code, and failure reason.

Clients persist a random installation UUID under the LunaBox local cache and
include it in update telemetry. No machine identifier or local path is sent.
Older events without this field are counted by update transaction.

## Local development

Build the administration assets and start the Worker in one terminal:

```powershell
pnpm install
if (!(Test-Path .dev.vars)) { Copy-Item .dev.vars.example .dev.vars }
pnpm build
pnpm exec wrangler d1 migrations apply lunabox-updates-test --local --env test
pnpm dev:worker
```

Start Vite in another terminal for hot module replacement:

```powershell
pnpm dev
```

Open `http://localhost:5173/admin/`. Vite proxies `/v1` requests to the local
Worker on port 8787. The administration token is read from `.dev.vars`.

## Setup

```powershell
pnpm install
Copy-Item .dev.vars.example .dev.vars
pnpm build
pnpm exec wrangler d1 create lunabox-updates
pnpm exec wrangler r2 bucket create lunabox-updates
pnpm exec wrangler secret put ADMIN_TOKEN --env production
pnpm exec wrangler d1 migrations apply lunabox-updates --remote --env production
pnpm deploy
```

Set a development-only token in `.dev.vars`, which is ignored by Git. Run
`pnpm types` after changing bindings or variables in `wrangler.jsonc`.

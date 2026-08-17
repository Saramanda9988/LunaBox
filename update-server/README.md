# LunaBox Update Server

Cloudflare Worker update service backed by R2 and D1.

## Object layout

```text
channels/<channel>/version.json
releases/<version>/manifest.json
releases/<version>/<asset>
```

Versioned release objects are immutable. A channel document is published last
and points clients to the selected version manifest.

## API

```text
GET  /health
GET  /v1/channels/<channel>
GET  /v1/releases/<version>/manifest
GET  /v1/releases/<version>/version
GET  /v1/releases/<version>/assets/<asset>
POST /v1/events
GET  /v1/stats/releases/<version>
```

The statistics endpoint requires `Authorization: Bearer <ADMIN_TOKEN>`.

## Setup

```powershell
pnpm install
pnpm exec wrangler d1 create lunabox-updates
pnpm exec wrangler r2 bucket create lunabox-updates
pnpm exec wrangler secret put ADMIN_TOKEN
pnpm exec wrangler d1 migrations apply lunabox-updates --remote
pnpm deploy
```

Replace `REPLACE_WITH_D1_DATABASE_ID` in `wrangler.jsonc` with the ID returned
by the D1 create command.

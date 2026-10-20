# Live Streaming Service Architecture

## Boundary

```text
OBS (creator machine)
  ↓  RTMP publish + stream key
MediaMTX (:1935 RTMP, :8888 HLS, :9997 control — internal only)
  ↓  HTTP auth + lifecycle hooks
live-streaming-service (:8096)
  ↓  owns sessions, keys (hashed), status, playback URLs
PostgreSQL (edvance_live)
  ↓
creator-web Live Studio (create/OBS panel/hls.js preview)
student-app Live Classes (expo-video plays HLS)
```

MediaMTX is dumb transport: ingest, remux to HLS, and callbacks.
The Go service owns all product state. The control API on 9997 is
never published outside the compose network; only the service talks
to it (path listing, publisher kick).

## Why the stream key is `path?token=`

Design constraint: the playback URL must never contain the stream key.
MediaMTX extracts the `token` query parameter on the RTMP URL and hands
it to the HTTP authorizer, so:

- **Publish URL (secret, owner only):** `rtmp://<host>:1935/ls_<16hex>?token=<64hex>`
- **Playback URL (public):** `http://<host>:8888/ls_<16hex>/index.m3u8`
- **Database:** SHA-256 of the token + a 4-char hint. The raw key is
  returned once at create/regenerate and never stored or re-readable.

Revoking a key = flipping status to `ended` (the authorizer rejects any
future publish) + best-effort kick of the live RTMP connection.

## Status lifecycle (hook-confirmed only)

```text
        create                publish hook (runOnOnline)
   waiting      →  (OBS connects, auth ok)  →  live
      ↑                                          │
      │         done hook (runOnOffline)         │
      └──────────── (OBS disconnects) ───────────┘
                         end session
                            ↓  (terminal)
                          ended
```

- `live` is never set by the create/start API — only by MediaMTX's
  `runOnOnline` hook, so the UI can't fake a broadcast.
- OBS dropping the connection returns the session to `waiting`
  (re-runnable with the same key); `ended` is terminal and revokes
  the key.
- A 15s reconciler compares DB state against
  `GET :9997/v3/paths/list` and repairs any missed hook.

## Identity flow (creator API)

```text
creator-web
  ↓  Authorization: Bearer <JWT>
API Gateway (strips /api/v1/live or /api/v1/streams prefix)
  ↓
live-streaming-service
  ↓  HS256-only JWT validation (internal/token), sub → creator_id
  ↓  ownership enforced in service layer (403 across creators)
PostgreSQL (edvance_live)
```

## Media hooks (internal only)

```text
MediaMTX  →  POST /internal/hooks/authorize   (RTMP connect; body has path+token query)
MediaMTX  →  POST /internal/hooks/publish     (path online → status live)
MediaMTX  →  POST /internal/hooks/done        (path offline → status waiting)
```

Three independent guards keep hooks off the public internet:

1. Route mounted only under `/internal`, never under a gateway prefix.
2. `INTERNAL_SERVICE_TOKEN` bearer check (constant-time) — MediaMTX
   sends it via `MEDIAMTX_HOOK_TOKEN`.
3. Requests carrying the gateway's `X-Edvance-Gateway` header are
   rejected, so a misconfigured gateway route still fails closed.

## Public surface (no auth, no secrets)

```text
GET /streams/active                 discovery for student-app
GET /streams/{streamID}/playback    {id, title, isLive, playbackUrl, ...}
```

Creator surface (JWT): `POST /streams`, `GET /streams/mine`,
`GET /streams/{id}`, `POST /streams/{id}/end`,
`POST /streams/{id}/regenerate-key`. Only create/regenerate responses
carry the raw key, and only to the authenticated owner.

Errors everywhere: `{"error": {"code", "message"}}`.

## Related

- Ops + E2E walkthrough: `services/live-streaming-service/README.md`
- Media transport config: `infrastructure/docker/mediamtx/mediamtx.yml`

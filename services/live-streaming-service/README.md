# Live Streaming Service

Owns **live session orchestration**: stream sessions, hashed stream
keys, status (`waiting → live → ended`), playback URLs, and the
MediaMTX control/hooks boundary. Media ingest/remux is delegated to
MediaMTX (RTMP + HLS); this service never touches bytes.

Port: `8096` · Database: `edvance_live` · Module:
`github.com/Anshul563/edvance-project/services/live-streaming-service`

Architecture deep-dive: `docs/architecture/live-streaming-service.md`

## Responsibilities

- creator stream sessions (create / list / get / end / regenerate key)
- stream key issuing: raw key returned once, SHA-256 + hint stored
- HTTP authorization of RTMP publishers (MediaMTX `auth: http`)
- lifecycle hooks: publish → `live`, done → `waiting` (hook-confirmed only)
- 15s reconciler against the MediaMTX control API (`:9997`, internal)
- student-facing discovery + playback endpoints (no auth, no secrets)
- best-effort publisher kick on session end

## Non-responsibilities

RTMP/HLS serving, transcoding, recording, chat, reactions, viewer
counts, DRM, CDN, OBS itself. MediaMTX is transport only: the control
API (9997) is never published, and the raw stream key never appears in
any playback URL or the database.

## Environment

```env
APP_ENV=development
LIVE_STREAMING_SERVICE_PORT=8096
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_live?sslmode=disable
JWT_ACCESS_SECRET=      # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
INTERNAL_SERVICE_TOKEN= # shared ONLY with MediaMTX (MEDIAMTX_HOOK_TOKEN)
MEDIAMTX_API_URL=http://mediamtx:9997
RTMP_PUBLIC_URL=rtmp://<LAN-IP>:1935   # handed to OBS
HLS_PUBLIC_BASE_URL=http://<LAN-IP>:8888    # handed to students
```

`RTMP_PUBLIC_URL` / `HLS_PUBLIC_BASE_URL` must use an address the
creator's OBS and the students' devices can reach (your LAN IP, not
`localhost`) — set them in the root `.env`.

## Endpoints

Student-facing (public, never leak publisher secrets):

```text
GET  /streams/active                   list sessions with status=live
GET  /streams/{streamID}/playback      {id, title, isLive, playbackUrl, ...}
```

Creator-facing (JWT, ownership enforced):

```text
POST /streams                          {title, description?} → + raw streamKey (once)
GET  /streams/mine
GET  /streams/{streamID}
POST /streams/{streamID}/end           terminal; revokes key, kicks publisher
POST /streams/{streamID}/regenerate-key→ new raw streamKey (once)
```

Internal (MediaMTX only; bearer `INTERNAL_SERVICE_TOKEN`, gateway
header rejected):

```text
POST /internal/hooks/authorize | publish | done
```

Health: `GET /health` (alive), `GET /ready` (PostgreSQL).
Errors: `{"error": {"code", "message"}}`.

## Local end-to-end (OBS → HLS → phone)

### 1. Start the stack and migrate

```bash
cp .env.example .env          # once; set RTMP_PUBLIC_URL/HLS_PUBLIC_BASE_URL to your LAN IP
make migrate                  # creates edvance_live + applies migrations
make live-up                  # postgres, api-gateway, auth-service, live-streaming-service, mediamtx
docker compose ps             # all should be healthy/up
```

Published ports: `8080` gateway, `8096` live service, `1935` RTMP,
`8888` HLS. Control API `9997` stays internal.

### 2. Create a stream (creator-web Live Studio, or curl)

UI: `http://localhost:3000/creator/live` (creator-web) → **Create Live
Stream**. Copy the **Server** URL and **Stream Key** shown in the OBS
panel — the key is displayed once.

curl equivalent (JWT from auth-service login/register):

```bash
TOKEN='<jwt>'
curl -sS -X POST http://localhost:8096/streams \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"Algebra live class","description":"Chapter 3"}'
# → {"id":"…","streamKey":"ls_…?token=…","rtmpUrl":"rtmp://…","playbackUrl":"http://…","status":"waiting",...}
```

### 3. Publish with OBS

Settings → Stream:

- Service: **Custom…**
- Server: the copied `rtmpUrl` (no app path — MediaMTX uses the whole
  URL path as the stream path)
- Stream Key: the copied stream key (`ls_…?token=…`). The token in the
  URL is what MediaMTX forwards to the authorizer.

Start Streaming. Within a few seconds the Live Studio card flips
`Waiting for OBS → Live` (hook-confirmed) and the built-in hls.js
preview starts playing `playbackUrl`.

### 4. Publish without OBS (ffmpeg smoke test)

```bash
STREAM_KEY='ls_xxx?token=yyy'   # from step 2
docker run --rm --network edvance_default jrottenberg/ffmpeg:6.1-alpine \
  -re -f lavfi -i testsrc=size=1280x720:rate=25 \
  -f lavfi -i sine=frequency=440 \
  -c:v libx264 -preset veryfast -tune zerolatency -c:a aac -f flv \
  "rtmp://mediamtx:1935/${STREAM_KEY}"
```

Verify HLS is being served:

```bash
curl -sS http://localhost:8888/<pathName>/index.m3u8   # pathName without ?token=
```

### 5. Watch on the student app

```bash
cd apps/student-app && npx expo start
```

The **Live Classes** screen polls `GET :8096/streams/active` (base URL
derived from the Expo dev-server host, or `EXPO_API_URL`) and plays the
selected stream with expo-video (HLS). Physical device and laptop must
share the LAN; the stream appears only while step 3/4 is actually
publishing.

### 6. Stop / revoke

- OBS: Stop Streaming → session returns to `waiting`, replayable with
  the same key.
- Live Studio: **End Stream** → terminal `ended`; key revoked, any
  connected publisher kicked; the student app drops it from the list.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| OBS: "Failed to connect" | `rtmpUrl` copied fully (path + `?token=`); port 1935 reachable; `docker logs edvance-mediamtx` |
| OBS connects, UI stays `Waiting` | `docker logs edvance-live-streaming-service` — authorize/publish hook errors; `INTERNAL_SERVICE_TOKEN` matches `MEDIAMTX_HOOK_TOKEN` |
| Status `live` but no preview/playback | HLS port 8888 reachable from the viewer; `curl http://localhost:8888/<path>/index.m3u8`; `docker logs edvance-mediamtx` |
| Student app: "Couldn't load live classes" | `API:` line at the bottom of the screen must be your machine's LAN IP; port 8096 reachable from the phone |
| Stream never leaves `waiting` after OBS start | Authorizer rejected: token hash mismatch (key regenerated?) or session `ended`; recreate the stream |

## Tests

```bash
cd services/live-streaming-service
go test ./...
```

`internal/router/flow_test.go` covers the full HTTP surface: auth
required/ownership 403s, hook token + gateway-header rejection, the
`waiting → live → waiting → ended` lifecycle, and that no response ever
contains the raw stream key outside create/regenerate.

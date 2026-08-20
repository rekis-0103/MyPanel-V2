# MyPanel V1 API

Semua respons API menggunakan JSON kecuali WebSocket console dan respons tanpa
body. Error berbentuk `{ "error": "...", "code": "...", "requestId": "..." }`.

## Authentication

- `POST /api/v1/auth/login` — body `username` dan `password`; menetapkan cookie
  HttpOnly dan mengembalikan user beserta `csrfToken`.
- `GET /api/v1/auth/me` — session dan CSRF token saat ini.
- `POST /api/v1/auth/logout` — menghapus session.

Semua endpoint lain selain health memerlukan session. Request mutasi juga harus
mengirim `X-CSRF-Token`, dan origin browser harus sama dengan `TRUSTED_ORIGIN`.

## Health dan catalog

- `GET /api/v1/health/live` — liveness process.
- `GET /api/v1/health/ready` — status PostgreSQL, Redis, dan agent.
- `GET /api/v1/catalog` — runtime Minecraft yang didukung.

## Servers dan jobs

- `GET|POST /api/v1/servers`
- `GET|DELETE /api/v1/servers/{id}`
- `POST /api/v1/servers/{id}/actions` — `start`, `stop`, atau `restart`.
- `GET|PUT /api/v1/servers/{id}/config`
- `GET /api/v1/jobs/{id}`

Create, lifecycle, config, backup, restore, dan delete adalah operasi
asynchronous. Respons `202 Accepted` berisi objek `job`; UI dapat memantau status
`queued`, `running`, `completed`, atau `failed` melalui endpoint job/server.

## Runtime features

- `GET /api/v1/servers/{id}/metrics`
- `GET /api/v1/servers/{id}/logs`
- `GET|PUT|DELETE /api/v1/servers/{id}/files?path=...`
- `GET|POST /api/v1/servers/{id}/backups`
- `POST /api/v1/servers/{id}/backups/{backupId}/restore`
- `DELETE /api/v1/servers/{id}/backups/{backupId}`
- `GET|POST /api/v1/servers/{id}/schedules`
- `DELETE /api/v1/servers/{id}/schedules/{scheduleId}`
- `GET /api/v1/audit`

`/api/v1/servers/{id}/console` di-upgrade menjadi WebSocket. Client mengirim
`{"type":"command","command":"say hi","csrfToken":"..."}` dan menerima event
`log`, `status`, serta `command-result`.

File dibatasi 10 MiB per operasi dan path absolut, traversal, serta symlink yang
keluar dari root server ditolak agent. Pesan error node sengaja disanitasi pada
boundary controller.

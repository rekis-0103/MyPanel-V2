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
- `GET /api/v1/catalog` — runtime Minecraft dan pilihan `javaVersions` yang
  didukung.

## Servers dan jobs

- `GET|POST /api/v1/servers`
- `GET|DELETE /api/v1/servers/{id}`
- `POST /api/v1/servers/{id}/actions` — `start`, `stop`, atau `restart`.
- `GET|PUT /api/v1/servers/{id}/config`
- `GET /api/v1/jobs/{id}`

Create, lifecycle, config, backup, restore, dan delete adalah operasi
asynchronous. Respons `202 Accepted` berisi objek `job`; UI dapat memantau status
`queued`, `running`, `completed`, atau `failed` melalui endpoint job/server.

Create server menerima `javaVersion` bernilai `21` atau `25`. Nilai kosong dari
client lama dianggap `21`. Respons server selalu menyertakan `javaVersion`.
Nama/tag image tidak pernah diterima dari browser; agent memetakan versi yang
sudah divalidasi ke image yang dikonfigurasi operator.

Config menerima properti server yang sudah ada serta `jvmOpts` dan `extraArgs`.
Kedua startup field dibatasi 512 karakter dan hanya menerima karakter allowlist;
shell operator, quote, newline, command substitution, dan Java argument file
ditolak. Update config membuat ulang container melalui job durable dan tidak
menghapus direktori data.

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
`history`, `log`, `status`, `lifecycle`, serta `command-result`. Event
`history` adalah snapshot terurut yang menggabungkan log bertimestamp Docker
dan lifecycle tersimpan saat koneksi dibuka. Event `log` berikutnya hanya
membawa baris setelah cursor RFC3339Nano terakhir. Event `status` cepat dapat
hanya membawa `state`; pembaruan periodik juga membawa objek `metrics` berisi
`state`, `cpuPercent`, `memoryBytes`, `diskBytes`, dan `players`.
`lifecycle` membawa pesan control-plane yang disanitasi dan dipertahankan
sebanyak 200 event terbaru per server untuk replay setelah reload.
`cpuPercent` memakai semantik Docker: 100 berarti satu vCPU terpakai penuh dan
nilai maksimum praktis mengikuti jumlah vCPU server dikali 100.
Selama health Minecraft belum `healthy`, event status membawa state `starting`
dan command WebSocket ditolak dengan hasil `server is still starting`.

File dibatasi 10 MiB per operasi dan path absolut, traversal, serta symlink yang
keluar dari root server ditolak agent. Pesan error node sengaja disanitasi pada
boundary controller.

# MyPanel V2 API

Semua respons API menggunakan JSON kecuali WebSocket console dan respons tanpa
body. Error berbentuk `{ "error": "...", "code": "...", "requestId": "..." }`.

## Authentication

Selain login owner yang sudah ada, `POST /api/v1/auth/register` membuat akun
role `user` ketika `REGISTRATION_ENABLED=true`. Registrasi dibatasi per IP.
`POST /api/v1/auth/change-password` mengganti password sendiri. Akun yang
di-reset owner hanya dapat memakai endpoint auth sampai password sementara
diganti. Suspend, reset, dan perubahan password menginvalidasi session lama.

- `POST /api/v1/auth/login` — body `username` dan `password`; menetapkan cookie
  HttpOnly dan mengembalikan user beserta `csrfToken`.
- `GET /api/v1/auth/me` — session dan CSRF token saat ini.
- `POST /api/v1/auth/logout` — menghapus session.

Semua endpoint lain selain health memerlukan session. Request mutasi juga harus
mengirim `X-CSRF-Token`, dan origin browser harus sama dengan `TRUSTED_ORIGIN`.

## Health dan catalog

- `GET /api/v1/health/live` — liveness process.
- `GET /api/v1/health/ready` — status PostgreSQL, Redis, dan agent.
- `GET /api/v1/catalog` — runtime Minecraft, identifier icon, pilihan
  `javaVersions`, dan daftar `versions` berbentuk `{id, java}` untuk checkout.

## Servers dan jobs

`GET /servers` dibatasi oleh ownership untuk role `user`; owner melihat semua
server beserta `ownerUserId` dan `ownerUsername`. Create/delete server mentah
hanya tersedia untuk owner. Semua endpoint turunan server dan job melakukan
cek ownership lagi di controller, termasuk upgrade WebSocket.

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

## Hosting simulasi dan administrasi

- `GET /api/v1/packages` — paket aktif; owner dapat memakai `?all=1`.
- `POST /api/v1/packages`, `PUT|DELETE /api/v1/packages/{id}` — kelola paket,
  khusus owner. Payload create/update juga menerima `themeColor` berupa warna
  hex `#RRGGBB`, `icon` dari katalog `grass|anvil|gold|diamond|feather|crystal`,
  serta boolean `isPopular` dan `isRecommended`. Nilai ini dikembalikan pada
  respons paket publik untuk membentuk tampilan dan label marketplace.
- `GET /api/v1/capacity` — user menerima sisa kapasitas jual dan availability
  paket; owner juga menerima total, reserved, dan telemetri host.
- `POST /api/v1/checkout` — khusus user; membutuhkan `packageId`, UUID
  `idempotencyKey`, nama server, runtime, dan versi Minecraft. `javaVersion`
  diterima untuk kompatibilitas client, tetapi controller selalu menurunkannya
  kembali dari versi Minecraft (`26.x` ke Java 25, versi sebelumnya ke Java 21).
- `GET /api/v1/orders` dan `GET /api/v1/subscriptions` — milik user; owner
  melihat seluruh data.
- `POST /api/v1/subscriptions/{id}/renew` — perpanjangan simulasi 30 hari.
- `POST /api/v1/subscriptions/{id}/retry` — ulang provisioning berstatus
  `action_required` tanpa pembayaran baru.
- `GET /api/v1/admin/users` — daftar/search user, khusus owner.
- `PUT /api/v1/admin/users/{id}/status` — suspend/aktifkan dan invalidasi session.
- `POST /api/v1/admin/users/{id}/reset-password` — tetapkan password sementara.
- `PUT /api/v1/admin/servers/{id}/owner` — alihkan server berlangganan ke user
  aktif lain.

Checkout dan reaktivasi memakai advisory lock PostgreSQL. Pengecekan kapasitas,
alokasi port, order, subscription, server, dan job berada dalam transaksi. UUID
idempotency unik per user mencegah order ganda saat request diulang.

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
dan lifecycle tersimpan saat koneksi dibuka. Event `log` berikutnya diteruskan
per baris dari stream Docker persisten setelah cursor RFC3339Nano terakhir,
tanpa interval polling atau buffer waktu browser. Event `status` cepat dapat
hanya membawa `state`; pembaruan periodik juga membawa objek `metrics` berisi
`state`, `cpuPercent`, `memoryBytes`, `diskBytes`, dan `players`.
`lifecycle` membawa pesan control-plane yang disanitasi dan dipertahankan
sebanyak 200 event terbaru per server untuk replay setelah reload.
`cpuPercent` memakai semantik Docker: 100 berarti satu vCPU terpakai penuh dan
dibatasi pada jumlah vCPU server dikali 100.
Sebelum marker ready Paper dari boot saat ini terdeteksi (atau health Minecraft
menjadi `healthy`), event status membawa state `starting` dan command WebSocket
ditolak dengan hasil `server is still starting`.

File dibatasi 10 MiB per operasi dan path absolut, traversal, serta symlink yang
keluar dari root server ditolak agent. Pesan error node sengaja disanitasi pada
boundary controller.

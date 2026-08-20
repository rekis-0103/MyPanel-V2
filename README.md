# MyPanel V2

MyPanel adalah control plane Minecraft Java untuk satu owner dan satu Linux VM.
Arsitekturnya terinspirasi panel hosting modern, tetapi implementasinya berdiri
sendiri: React untuk UI, Go untuk controller dan node agent, PostgreSQL untuk
state durable, Redis untuk session/rate limit, serta Docker untuk runtime server.

Status proyek: **v0.1.0 / v1 single-node**. Fitur inti sudah dapat dijalankan,
tetapi proyek belum ditujukan sebagai layanan multi-tenant atau pengganti
Pterodactyl yang kompatibel langsung.

## Fitur v1

- Login owner dengan Argon2id, session opaque di Redis, CSRF, origin check, dan
  rate limit login.
- Provisioning asynchronous dan lifecycle start/stop/restart/delete dengan job
  durable serta reconciliation desired/observed state.
- Runtime Vanilla, Paper, Purpur, Fabric, Forge, dan NeoForge dengan versi yang
  dapat dipilih.
- Alokasi port otomatis; limit CPU, total memory, JVM heap headroom, PID, dan
  kuota disk yang dipantau agent.
- Console WebSocket interaktif, metrics, file manager terkurung pada root server,
  backup/restore, schedule, settings, dan audit activity.
- Agent privat dengan mTLS. Controller dan web tidak menerima Docker socket.
- Compose dengan secret files, network terpisah, filesystem read-only, capability
  drop, log rotation, serta health/readiness check.

Detail boundary dan data flow ada di [docs/architecture.md](docs/architecture.md),
sedangkan kontrak HTTP ada di [docs/api.md](docs/api.md).

## Persyaratan

- Linux VM dengan Docker Engine dan Docker Compose v2
- Sedikitnya 4 GiB RAM; sesuaikan `NODE_MEMORY_MB` agar menyisakan RAM untuk OS,
  database, Redis, controller, dan agent
- Port panel dan rentang port game yang dapat dijangkau oleh klien yang sesuai

## Instalasi

```sh
cp .env.example .env
sh scripts/init-secrets.sh
docker compose config --quiet
docker compose up --build -d
docker compose ps
```

Password owner pertama tersimpan di `secrets/admin_password`. File tersebut dan
secret lain tidak masuk Git. Buka panel pada `http://127.0.0.1:8080` dari VM,
atau ubah `PANEL_BIND_IP` ke IP host-only/VPN yang memang ingin dilayani.

Sebelum panel diakses melalui HTTPS, set `TRUSTED_ORIGIN` ke origin HTTPS dan
`COOKIE_SECURE=true`. Jangan expose panel HTTP langsung ke internet. Panduan
SSH, firewall, TLS/VPN, backup, dan update ada di
[docs/runbooks/vm-hardening.md](docs/runbooks/vm-hardening.md).

### PowerShell

```powershell
Copy-Item .env.example .env
./scripts/init-secrets.ps1
docker compose config --quiet
docker compose up --build -d
```

## Operasi

```sh
# Status dan readiness
docker compose ps
curl --fail http://127.0.0.1:8080/api/v1/health/ready

# Log terfokus
docker compose logs --tail=200 controller agent

# Terapkan image/source baru tanpa menghapus data
docker compose up --build -d

# Hentikan stack; named volume dan world tetap dipertahankan
docker compose down
```

World dan backup berada di `/var/lib/mypanel`. PostgreSQL dan Redis memakai named
volume Docker. Jangan menjalankan `docker compose down -v` kecuali memang ingin
menghapus state database, session, dan sertifikat internal.

## Development dan quality gate

```sh
cd controller
go test -race ./...
go vet ./...

cd ../web
corepack enable
pnpm install --frozen-lockfile
pnpm lint
pnpm test
pnpm build
```

Frontend dev server memakai `pnpm dev` dan mem-proxy `/api` ke controller pada
`127.0.0.1:8080`. Untuk validasi deployment, jalankan
`docker compose config --quiet` sebelum `docker compose up`.

## Dokumentasi proyek

- [Arsitektur dan trust boundary](docs/architecture.md)
- [Kontrak API](docs/api.md)
- [Runbook deployment dan hardening VM](docs/runbooks/vm-hardening.md)
- [Rencana dan catatan verifikasi v1](docs/plans/mypanel-v1.md)
- [Panduan kontribusi](CONTRIBUTING.md)
- [Kebijakan keamanan](SECURITY.md)
- [Riwayat perubahan](CHANGELOG.md)

Kontribusi melalui issue atau pull request dipersilakan. Jangan melaporkan
kerentanan atau membagikan kredensial melalui issue publik; ikuti
[SECURITY.md](SECURITY.md).

## Batasan v1

- Satu owner dan satu node; belum ada multi-tenant, sub-user, billing, atau
  cluster scheduling.
- Kuota disk ditegakkan oleh file manager, pemeriksaan sebelum start/restart,
  dan reconciliation berkala. Hard filesystem project quota bergantung pada
  filesystem host dan belum dikonfigurasi otomatis.
- Jumlah pemain belum diambil dari query Minecraft dan sementara dilaporkan `0`.
- Backup tersimpan lokal pada node; salinan off-site harus ditambahkan oleh
  operator.
- TLS publik/VPN dan hardening SSH sengaja menjadi tindakan operator agar proses
  instalasi tidak memutus akses VM.

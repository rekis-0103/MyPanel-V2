# MyPanel V2

MyPanel adalah control plane dan simulasi layanan hosting Minecraft Java
multi-user untuk satu Linux VM.
Arsitekturnya terinspirasi panel hosting modern, tetapi implementasinya berdiri
sendiri: React untuk UI, Go untuk controller dan node agent, PostgreSQL untuk
state durable, Redis untuk session/rate limit, serta Docker untuk runtime server.

Status proyek: **v0.2.0-dev / single-node multi-tenant**. Transaksi di panel
bersifat simulasi dan tidak memproses uang nyata. Proyek belum ditujukan sebagai
pengganti Pterodactyl yang kompatibel langsung.

## Fitur v1

- Login owner dan user dengan Argon2id, session opaque di Redis, CSRF, origin
  check, rate limit, registrasi opsional, suspend akun, serta reset password
  sementara yang wajib diganti saat login berikutnya.
- Isolasi ownership: user hanya dapat melihat dan mengelola resource server
  miliknya; owner dapat mengelola semua server melalui ID serta melihat pemilik.
- Marketplace paket, checkout instan simulasi, order, perpanjangan 30 hari,
  retry provisioning, masa tenggang 7 hari, dan pelepasan CPU/RAM/port tanpa
  menghapus world.
- Dashboard kapasitas admin untuk total, reservasi, sisa vCPU/RAM/disk/port, dan
  telemetri host. Checkout ditolak atomik bila resource tidak cukup.
- Provisioning asynchronous dan lifecycle start/stop/restart/delete dengan job
  durable serta reconciliation desired/observed state.
- Runtime Vanilla, Paper, Purpur, Fabric, Forge, dan NeoForge dengan versi
  Minecraft serta Java 21/25 yang dapat dipilih per server.
- Alokasi port otomatis; limit CPU, total memory, JVM heap headroom, PID, dan
  kuota disk yang dipantau agent.
- UI responsif dengan navigasi server kontekstual, mode terang/gelap, navigasi
  bawah mobile, deep-link per server, serta pilihan bahasa Indonesia/English.
- Console xterm.js berwarna melalui WebSocket incremental, grafik CPU/RAM/disk,
  file manager tabel dengan upload
  drag-and-drop dan download, backup/restore, schedule, settings, dan audit log.
- Agent privat dengan mTLS. Controller dan web tidak menerima Docker socket.
- Compose dengan secret files, network terpisah, filesystem read-only, capability
  minimum, log rotation, serta health/readiness check. Agent hanya mempertahankan
  `DAC_OVERRIDE` agar dapat mengelola direktori data milik UID Minecraft.

Detail boundary dan data flow ada di [docs/architecture.md](docs/architecture.md),
sedangkan kontrak HTTP ada di [docs/api.md](docs/api.md).

## Persyaratan

- Linux VM dengan Docker Engine dan Docker Compose v2
- Sedikitnya 4 GiB RAM; sesuaikan `NODE_MEMORY_MB` agar menyisakan RAM untuk OS,
  database, Redis, controller, dan agent
- Port panel dan rentang port game yang dapat dijangkau oleh klien yang sesuai

Tetapkan `NODE_CPUS`, `NODE_MEMORY_MB`, dan `NODE_DISK_MB` sebagai kapasitas yang
boleh dijual. Sisakan headroom untuk OS dan service panel. Registrasi publik
dapat dimatikan dengan `REGISTRATION_ENABLED=false`.

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

# Terapkan source lokal baru dan migrasinya tanpa menghapus data
docker compose build
docker compose run --rm migrate
docker compose up -d

# Hentikan stack; named volume dan world tetap dipertahankan
docker compose down
```

World dan backup berada di `/var/lib/mypanel`. PostgreSQL dan Redis memakai named
volume Docker. Jangan menjalankan `docker compose down -v` kecuali memang ingin
menghapus state database, session, dan sertifikat internal.

Alamat server yang bind ke `0.0.0.0` ditampilkan menggunakan hostname panel,
sehingga panel pada `192.168.56.101` mengiklankan `192.168.56.101:<port>` tanpa
mengubah bind Docker yang tetap menerima koneksi pada seluruh interface.

## Alur akun dan hosting simulasi

Owner tetap login dengan akun bootstrap lama. User mendaftar dari halaman login,
memilih paket bertema Starter, Iron, Gold, atau Diamond, lalu checkout simulasi
membuat server, order, subscription, alokasi port, dan job provisioning dalam
satu transaksi. Tidak ada payment gateway atau uang nyata.

Saat checkout, user memilih runtime dan versi Minecraft dari katalog. Java
ditetapkan otomatis: Minecraft `26.x` memakai Java 25, sedangkan versi yang lebih
lama pada katalog memakai Java 21. Controller menghitung ulang pilihan ini agar
nilai Java tidak bergantung pada data yang dikirim browser.

Subscription berlaku 30 hari. Setelah kedaluwarsa, server dihentikan dan masuk
masa tenggang 7 hari. Jika belum diperpanjang, container, CPU, RAM, dan port
dilepas; world tetap disimpan dan disk masih dihitung sebagai kapasitas
terpakai. Reaktivasi mengambil port baru dan memprovision ulang container dengan
data lama. Schedule tidak berjalan untuk subscription yang tidak aktif.

Saat migrasi multi-user pertama, session Redis lama akan ditolak karena belum
memiliki versi session. Login ulang sekali dengan credential yang sama; password
owner tidak berubah.

Saat owner membuat server secara manual, pilih Java 21 untuk kompatibilitas luas
atau Java 25 untuk server/plugin modern yang sudah mendukungnya. Image dapat
dipin melalui
`MINECRAFT_IMAGE_JAVA_21` dan `MINECRAFT_IMAGE_JAVA_25` di `.env`; browser tidak
dapat memasukkan image arbitrary. Server yang sudah ada dimigrasikan ke Java 21
agar perilakunya tidak berubah.

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
`127.0.0.1:8080`. Buka `http://127.0.0.1:5173`; route seperti
`/servers/<id>/console` dapat dibuka langsung dan akan tetap bekerja di image
Caddy produksi. Untuk validasi deployment, jalankan
`docker compose config --quiet` sebelum `docker compose up`.

Telemetri CPU, RAM, disk, dan uptime host tersedia untuk owner melalui endpoint
kapasitas; user hanya menerima sisa kapasitas jual dan ketersediaan paket.
Operasi file Rename dan New Folder tetap dinonaktifkan sampai
kontrak backend khusus tersedia; upload, download, edit, dan delete tetap aktif.
Metric server tetap tersedia: CPU mengikuti angka Docker (100% setara satu
vCPU penuh dan dapat mencapai `jumlah vCPU × 100%`), RAM mengurangi cache
cgroup, dan disk menghitung file di direktori data server. Console memakai
timestamp Docker sebagai cursor internal, menghilangkannya dari tampilan,
mempertahankan timestamp Minecraft, serta memberi
warna berbeda pada level INFO/WARN/ERROR dan nama plugin. Setelah container
dinyalakan, status tetap `starting` dan command console terkunci sampai marker
`Done (...)! For help, type ...` dari boot saat ini terdeteksi atau healthcheck
Minecraft menyatakan server siap; light mode juga memakai terminal berlatar
terang dengan palette ANSI berkontras tinggi. Warna keluaran plugin didukung
melalui ANSI SGR, kode Minecraft `§`, legacy `&`, RGB `&x&…`, serta tag
MiniMessage bernama/hex dan dekorasi. Container baru atau yang diperbarui juga
meminta logger Adventure menghasilkan ANSI true-color secara eksplisit.
Event lifecycle control plane ditampilkan oranye di console, termasuk proses
start/restart, status running/stopped, restart berhasil, dan alasan kegagalan
yang aman seperti OOM, exit code, disk limit, atau timeout healthcheck. Sebanyak
200 event terbaru per server disimpan agar tetap tersedia setelah halaman
console dimuat ulang. Snapshot log dan lifecycle digabungkan berdasarkan waktu;
setelah itu agent mempertahankan satu stream Docker dan meneruskan setiap baris
baru tanpa polling atau buffer waktu di browser. xterm tetap memakai write queue
ber-backpressure agar burst besar tidak membekukan halaman. Command diproses
oleh antrean worker terpisah dan ditulis langsung ke named pipe persisten pada
data server, sehingga tidak perlu membuat proses `docker exec` per command.
Container lama tetap memakai fallback sampai konfigurasi berikutnya diterapkan.
Nilai CPU runtime juga dibatasi pada kapasitas container (`jumlah vCPU × 100%`),
sehingga server 2 vCPU ditampilkan dalam rentang 0–200%. Saat startup/restart,
agent sementara memberi burst 25% (2 vCPU menjadi 2,5 core) dan otomatis
mengembalikan hard limit segera setelah Minecraft siap.
Server Paper dan Purpur juga mengaktifkan `optimize-explosions` melalui patch
startup internal. Optimasi ini mengurangi pekerjaan pencarian entity saat TNT
atau ledakan lain diproses, tetapi tidak menjamin 20 TPS untuk puluhan ribu TNT
karena simulasi ledakan tetap berjalan pada tick thread Minecraft.

## Update dari GitHub

Setelah versi yang diinginkan sudah tersedia pada branch GitHub yang dilacak,
jalankan dari VM:

```sh
cd ~/MyPanel-V2
sh scripts/update.sh
```

Script hanya menerima update fast-forward pada working tree bersih, memvalidasi
Compose, membangun image baru, menjalankan migrasi secara eksplisit, mengganti
container, lalu menunggu readiness. `.env`, secret, world, backup, serta named
volume tidak diubah. Untuk update otomatis setiap lima menit, ikuti
[runbook update VM](docs/runbooks/vm-updates.md).

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

## Batasan saat ini

- Satu owner dan satu node; belum ada cluster scheduling, invoice pajak,
  payment gateway nyata, refund, kupon, atau email transactional.
- Pembelian dan pembayaran sepenuhnya simulasi. Status order bukan bukti
  pembayaran nyata.
- Kuota disk ditegakkan oleh file manager, pemeriksaan sebelum start/restart,
  dan reconciliation berkala. Hard filesystem project quota bergantung pada
  filesystem host dan belum dikonfigurasi otomatis.
- Jumlah pemain belum diambil dari query Minecraft dan sementara dilaporkan `0`.
- Backup tersimpan lokal pada node; salinan off-site harus ditambahkan oleh
  operator.
- TLS publik/VPN dan hardening SSH sengaja menjadi tindakan operator agar proses
  instalasi tidak memutus akses VM.

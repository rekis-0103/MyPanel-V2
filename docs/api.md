# 🔌 Spesifikasi REST API & WebSocket MyPanel V2

Dokumen ini mendefinisikan kontrak HTTP, format payload JSON, dan protokol WebSocket untuk **MyPanel V2**.

Seluruh endpoint berada di bawah namespace `/api/v1` kecuali didefinisikan secara khusus. Respons error selalu mengembalikan format terstandarisasi:

```json
{
  "error": "Pesan deskriptif kesalahan",
  "code": "error_code_identifier",
  "requestId": "req_01j7abc123..."
}
```

---

## 🔐 1. Autentikasi & Akun (`/api/v1/auth`)

Request mutasi (`POST`, `PUT`, `DELETE`) wajib menyertakan cookie session `mypanel_session`, header `X-CSRF-Token`, dan origin yang sesuai dengan `TRUSTED_ORIGIN`.

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/auth/login` | Publik | Autentikasi dengan `username` dan `password`. Mengembalikan data session & CSRF token. |
| `POST` | `/api/v1/auth/register` | Publik | Pendaftaran user baru (aktif jika `REGISTRATION_ENABLED=true`). Dibatasi rate limit per IP. |
| `GET` | `/api/v1/auth/me` | User / Owner | Mengambil info session pengguna aktif, role, dan token CSRF. |
| `POST` | `/api/v1/auth/change-password` | User / Owner | Mengubah password mandiri. Menginvalidasi seluruh session aktif lama. |
| `POST` | `/api/v1/auth/logout` | User / Owner | Menghapus session aktif di Redis dan membersihkan cookie browser. |

---

## 🩺 2. Health Check & Katalog (`/api/v1/health`, `/api/v1/catalog`)

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/health/live` | Publik | Status liveness proses controller. |
| `GET` | `/api/v1/health/ready` | Publik | Memeriksa kesiapan PostgreSQL, Redis, dan konektivitas mTLS ke Node Agent. |
| `GET` | `/api/v1/catalog` | Publik | Daftar runtime yang didukung (Paper, Purpur, Fabric, Vanilla), icon SVG, dan pemetaan Java. |

---

## 🎮 3. Manajemen & Lifecycle Server (`/api/v1/servers`)

Operasi mutasi lifecycle mengembalikan status `202 Accepted` bersama objek `job` yang berjalan secara asinkronus di latar belakang.

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/servers` | User / Owner | Daftar server. User hanya melihat miliknya; Owner melihat seluruh armada server. |
| `POST` | `/api/v1/servers` | Owner | Membuat server manual dengan alokasi resource dan pilihan Java 21/25. |
| `GET` | `/api/v1/servers/{id}` | Terkait | Detail spesifikasi server, status aktual (`running`, `stopped`, dsb.), dan telemetri. |
| `POST` | `/api/v1/servers/{id}/actions` | Terkait | Menjalankan aksi lifecycle: `start`, `stop`, atau `restart`. |
| `GET` | `/api/v1/servers/{id}/config` | Terkait | Mengambil konfigurasi startup, flags JVM (`jvmOpts`), dan argumen game (`extraArgs`). |
| `PUT` | `/api/v1/servers/{id}/config` | Terkait | Memperbarui startup flags (tervalidasi regex ketat untuk mencegah shell injection). |
| `DELETE`| `/api/v1/servers/{id}` | Owner | Menghapus server. Menerima payload `{ "purgeData": boolean }`. |
| `GET` | `/api/v1/jobs/{id}` | Terkait | Memantau progress job asinkronus (`queued`, `running`, `completed`, `failed`). |

---

## 🛒 4. Marketplace & Billing Simulasi (`/api/v1/packages`, `/api/v1/checkout`)

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/packages` | User / Owner | Daftar paket kapasitas aktif. Owner dapat menyertakan `?all=1`. |
| `POST` | `/api/v1/packages` | Owner | Membuat paket baru dengan warna hex `themeColor` dan ikon SVG tematik. |
| `PUT` | `/api/v1/packages/{id}` | Owner | Mengubah kuota paket, harga simulasi, penanda `isPopular`, atau `isRecommended`. |
| `DELETE`| `/api/v1/packages/{id}` | Owner | Menonaktifkan atau menghapus paket. |
| `GET` | `/api/v1/capacity` | User / Owner | Telemetri kapasitas. User melihat sisa kapasitas jual; Owner melihat utilisasi host. |
| `POST` | `/api/v1/checkout` | User | Checkout paket simulasi ber-advisory lock atomik dengan kunci idempotensi UUID. |
| `GET` | `/api/v1/orders` | User / Owner | Riwayat pemesanan paket simulasi. |
| `GET` | `/api/v1/subscriptions` | User / Owner | Status langganan 30 hari dan masa tenggang 7 hari. |
| `POST` | `/api/v1/subscriptions/{id}/renew` | User | Memperpanjang masa aktif server 30 hari ke depan. |
| `POST` | `/api/v1/subscriptions/{id}/retry` | User | Mengulang provisioning instance yang berstatus `action_required`. |

---

## 📁 5. File Manager & Backup

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/servers/{id}/files` | Terkait | Menjelajahi file & folder (`?path=/`). Terproteksi dari path traversal & symlink. |
| `PUT` | `/api/v1/servers/{id}/files` | Terkait | Menyimpan isi file konfigurasi (maksimal 10 MiB per request). |
| `DELETE`| `/api/v1/servers/{id}/files` | Terkait | Menghapus file atau folder secara aman. |
| `POST` | `/api/v1/servers/{id}/files/folders` | Terkait | Membuat folder baru di server. |
| `POST` | `/api/v1/servers/{id}/files/move` | Terkait | Memindahkan atau mengubah nama (*rename*) file/direktori. |
| `GET` | `/api/v1/servers/{id}/backups` | Terkait | Daftar arsip backup snapshot server yang tersimpan di host. |
| `POST` | `/api/v1/servers/{id}/backups` | Terkait | Membuat backup manual baru (kompresi `.tar.gz` dengan SHA-256). |
| `GET` | `/api/v1/servers/{id}/backups/{id}/download` | Terkait | Mengunduh file arsip backup. |
| `POST` | `/api/v1/servers/{id}/backups/{id}/restore`| Terkait | Memulihkan server dari backup dengan verifikasi SHA-256 & auto snapshot. |
| `DELETE`| `/api/v1/servers/{id}/backups/{id}` | Terkait | Menghapus file arsip backup. |

---

## 🧩 6. Addons & Modpacks

| Method | Endpoint | Hak Akses | Deskripsi |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/servers/{id}/addons/search` | Terkait | Mencari plugin/mod kompatibel via Modrinth atau CurseForge (`?q=...`). |
| `GET` | `/api/v1/servers/{id}/addons` | Terkait | Inventori plugin/mod yang terpasang di server. |
| `POST` | `/api/v1/servers/{id}/addons` | Terkait | Memasang atau memperbarui plugin dengan verifikasi dependensi & SHA-512. |
| `DELETE`| `/api/v1/servers/{id}/addons/{addonId}`| Terkait | Menghapus plugin/mod dari server. |
| `GET` | `/api/v1/servers/{id}/modpacks` | Terkait | Mengambil modpack CurseForge yang sedang aktif pada server. |
| `GET` | `/api/v1/servers/{id}/modpacks/search` | Terkait | Mencari modpack resmi di katalog CurseForge. |
| `GET` | `/api/v1/servers/{id}/modpacks/{projectId}/versions` | Terkait | Daftar versi file modpack yang kompatibel dengan Forge/NeoForge & Java. |
| `POST` | `/api/v1/servers/{id}/modpacks` | Terkait | Memasang modpack dengan konfirmasi nama server, auto backup, dan rollback. |

---

## 🖥️ 7. Live Console WebSocket Protocol

**Endpoint URL**: `ws://<host>:<port>/api/v1/servers/{id}/console`

### Client Message (Kirim Perintah):
```json
{
  "type": "command",
  "command": "op Steve",
  "csrfToken": "csrf_token_string_here"
}
```

### Server Events (Menerima Output Streaming):
- **`history`**: Snapshot histori log gabungan saat koneksi pertama kali dibuka.
- **`log`**: Baris log baru dari Docker stream (dilengkapi ANSI / Minecraft color code).
- **`status`**: Pembaruan status berkala (state, CPU %, RAM bytes, Disk bytes, Players).
- **`lifecycle`**: Event penting control plane (proses start, shutdown, OOM alert, crash notice).
- **`command-result`**: Konfirmasi hasil penerimaan perintah.

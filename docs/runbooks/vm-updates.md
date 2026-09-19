# 🔄 Runbook Pembaruan VM (Continuous Deployment)

Runbook ini menjelaskan tata cara memperbarui instalasi **MyPanel V2** pada VM Linux langsung dari commit GitHub terbaru tanpa menghapus file konfigurasi `.env`, secret lokal, world server Minecraft, arsip backup, maupun data volume PostgreSQL/Redis.

---

## ⚡ 1. Pembaruan Manual Cepat

Pastikan perubahan telah di-push ke remote branch GitHub (`feat/mypanel-v1` atau `main`), kemudian jalankan perintah berikut pada terminal VM:

```sh
cd ~/MyPanel-V2
sh scripts/update.sh
```

### 🛡️ Validasi Otomatis `update.sh`:
Skrip secara proaktif menolak eksekusi jika menemukan kondisi berisiko:
1. Terdapat perubahan lokal yang belum di-commit (`untracked` atau `dirty working tree`).
2. Branch remote tidak ditemukan atau bukan merupakan *fast-forward merge*.
3. Sedang ada proses update lain yang berjalan (dilindungi *atomic directory lock*).

### 🚀 Alur Kerja Pembaruan:
```text
Fetch Origin ➔ Fast-Forward Merge ➔ Docker Build (--pull) ➔ Migrasi PostgreSQL ➔ Recreate Containers ➔ Healthcheck Verification (Ready)
```

> [!TIP]
> Jika Anda ingin memaksa pembaruan ulang tanpa ada commit baru (misalnya untuk memperbarui dependensi base image Docker), gunakan:
> ```sh
> MYPANEL_UPDATE_FORCE=1 sh scripts/update.sh
> ```

---

## 📊 2. Memeriksa Status & Log Pasca-Update

```sh
# Melihat status kontainer dan port yang aktif
docker compose ps

# Memantau log gabungan controller dan agent
docker compose logs --tail=100 -f controller agent web

# Memeriksa endpoint kesiapan internal
curl -sS http://127.0.0.1:8080/api/v1/health/ready
```

---

## ⏰ 3. Pembaruan Otomatis dengan systemd User Timer

Anda dapat mengonfigurasi VM agar memeriksa dan menerapkan pembaruan dari GitHub secara otomatis setiap interval waktu tertentu menggunakan `systemd` user timer:

```sh
# 1. Salin unit service dan timer
mkdir -p ~/.config/systemd/user
cp deploy/systemd/mypanel-update.service ~/.config/systemd/user/
cp deploy/systemd/mypanel-update.timer ~/.config/systemd/user/

# 2. Reload daemon dan aktifkan timer
systemctl --user daemon-reload
systemctl --user enable --now mypanel-update.timer

# 3. Pastikan timer tetap berjalan saat session SSH ditutup
sudo loginctl enable-linger "$USER"

# 4. Verifikasi status timer aktif
systemctl --user list-timers mypanel-update.timer
```

### Memeriksa Eksekusi Otomatis:
```sh
journalctl --user -u mypanel-update.service -n 100 --no-pager
```

---

## 🔐 4. Kebijakan Keamanan & Rollback

> [!WARNING]
> Siapa pun yang memiliki hak akses push ke branch deployment dapat mengeksekusi kode di Docker host saat update otomatis berjalan. Pastikan:
> - Mengaktifkan autentikasi dua faktor (2FA / MFA) pada akun GitHub Anda.
> - Membatasi hak akses branch (`Branch protection rules`).
> - Meninjau seluruh Pull Request sebelum di-merge ke branch deployment.

### Prosedur Rollback:
Skrip `update.sh` sengaja **tidak** melakukan rollback otomatis bila terjadi error. Jika Anda perlu kembali ke commit sebelumnya:
1. Lakukan `git checkout <commit_sha_stabil>`.
2. Jalankan `docker compose up --build -d`.
3. Validasi status container dengan `docker compose ps`.

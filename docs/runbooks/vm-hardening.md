# 🛡️ Runbook Pengamanan & Hardening VM Produksi

Runbook ini memandu operator dalam mengonfigurasi dan mengamankan Linux VM single-node sebelum menjalankan **MyPanel V2** di lingkungan produksi.

> [!CAUTION]
> Buat snapshot VM atau backup disk yang dapat dipulihkan sebelum melakukan perubahan konfigurasi firewall atau SSH guna menghindari risiko terkunci (*lockout*).

---

## 📁 1. Penyiapan Direktori & Secrets

```sh
# Buat direktori aplikasi dengan izin terbatas
sudo install -d -m 0750 -o "$USER" -g "$USER" /srv/mypanel
cd /srv/mypanel

# Salin konfigurasi environment & generate secret acak
cp .env.example .env
sh scripts/init-secrets.sh

# Amankan izin file kredensial (hanya user pemilik yang dapat membaca)
chmod 0600 .env secrets/*

# Validasi & jalankan stack
docker compose config --quiet
docker compose up --build -d
docker compose ps
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

> [!IMPORTANT]
> Jangan pernah menyalin file `*.example` sebagai secret produksi. Skrip `init-secrets.sh` menghasilkan string acak berkekuatan kriptografi tinggi dan melindungi secret yang sudah ada.

---

## 🌐 2. Konfigurasi Domain & TLS Reverse Proxy

Untuk akses publik melalui internet, letakkan reverse proxy bertoken TLS (misalnya Cloudflare Tunnel, Caddy eksternal, atau Nginx) di depan MyPanel, kemudian konfigurasikan `.env`:

```dotenv
COOKIE_SECURE=true
TRUSTED_ORIGIN=https://panel.domainanda.com
```

Terapkan perubahan dengan:
```sh
docker compose up -d
```

---

## 🔑 3. Pengamanan Akses SSH (Mencegah Brute Force)

1. Pastikan Anda telah memiliki SSH Key pair di komputer operator (`ssh-keygen -t ed25519`).
2. Pasang public key ke `~/.ssh/authorized_keys` user VM.
3. **Penting**: Buka terminal kedua untuk memverifikasi login via SSH key berhasil sebelum menutup session pertama.
4. Uji konfigurasi SSH dengan `sudo sshd -t`.
5. Nonaktifkan autentikasi password dan root login pada konfigurasi SSH (`/etc/ssh/sshd_config.d/99-hardening.conf`):
   ```text
   PasswordAuthentication no
   PermitRootLogin no
   KbdInteractiveAuthentication no
   ```
6. Terapkan konfigurasi dengan `sudo systemctl restart sshd`.

---

## 🧱 4. Konfigurasi Firewall (UFW)

Terapkan aturan firewall ketat untuk hanya membuka port yang diperlukan:

```sh
# Kebijakan default
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Izinkan port SSH hanya untuk operator
sudo ufw allow from <IP_OPERATOR> to any port 22 proto tcp

# Izinkan akses panel web
sudo ufw allow 8080/tcp

# Rentang alokasi port game server Minecraft
sudo ufw allow 25565:25749/tcp

# Aktifkan firewall
sudo ufw enable
sudo ufw status numbered
```

> [!NOTE]
> Jangan membuka port database PostgreSQL (`5432`), Redis (`6379`), atau Node Agent (`8081`). Komponen ini berada di jaringan internal Docker Compose dan tidak boleh dapat diakses dari luar.

---

## 💾 5. Strategi Pencadangan Off-Site (Disaster Recovery)

Untuk memitigasi risiko kegagalan hardware atau kerusakan storage host:

```sh
# 1. Dump database PostgreSQL
docker compose exec -T postgres pg_dump -U mypanel -d mypanel -Fc > mypanel_$(date +%F).dump

# 2. Arsip data world & konfigurasi
sudo tar -C /var/lib -czf mypanel-data_$(date +%F).tar.gz mypanel

# 3. Unggah arsip ke penyimpanan terpisah (S3 / remote backup server)
# rclone copy mypanel-data_$(date +%F).tar.gz remote:my-backups/
```

---

## 🔄 6. Pembaruan Rutin Sertifikat Internal (mTLS CA)

Sertifikat leaf mTLS internal antara Controller dan Agent berlaku selama 1 tahun. Sebelum masa berlaku habis:
1. Hentikan stack: `docker compose down`.
2. Hapus volume sertifikat internal (CA akan di-generate ulang otomatis saat startup):
   ```sh
   docker volume rm mypanel-v2_certs
   ```
3. Nyalakan kembali stack: `docker compose up -d`.
4. Browser client tidak akan terpengaruh karena sertifikat ini khusus untuk jaringan internal *controller-to-agent*.

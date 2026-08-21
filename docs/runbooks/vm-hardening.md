# VM Deployment and Hardening Runbook

Runbook ini ditujukan untuk VM single-node. Ambil snapshot VM atau backup yang
dapat dipulihkan sebelum perubahan SSH/firewall.

## 1. Siapkan direktori dan secrets

```sh
sudo install -d -m 0750 -o "$USER" -g "$USER" /srv/mypanel
cd /srv/mypanel
cp .env.example .env
sh scripts/init-secrets.sh
chmod 0600 .env secrets/*
docker compose config --quiet
docker compose up --build -d
docker compose ps
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

Jangan salin file `*.example` menjadi secret produksi. Generator menghasilkan
nilai acak dan mempertahankan file yang sudah ada.

## 2. Akses panel dengan aman

Pilihan sederhana untuk host-only development adalah mengikat `PANEL_BIND_IP`
ke alamat host-only VM dan membatasi firewall ke alamat host. Untuk penggunaan
di internet, taruh reverse proxy TLS atau VPN di depan panel, lalu set:

```dotenv
COOKIE_SECURE=true
TRUSTED_ORIGIN=https://panel.example.net
```

Reload dengan `docker compose up -d`. Sertifikat mTLS di named volume hanya untuk
jalur controller-agent; ia bukan pengganti TLS browser.

## 3. SSH tanpa risiko lockout

1. Buat SSH key pada komputer operator bila belum ada.
2. Tambahkan public key ke `~/.ssh/authorized_keys` user VM.
3. Buka terminal kedua dan pastikan login key berhasil. Biarkan session pertama
   tetap terbuka.
4. Jalankan `sudo sshd -t` sebelum reload SSH.
5. Baru setelah verifikasi, nonaktifkan password login dan root login melalui
   file drop-in distro, lalu reload service SSH.

Jangan menonaktifkan password sebelum terminal kedua berhasil login memakai key.
Ganti password VM yang pernah dibagikan dan jangan menyimpannya dalam repository.

## 4. Firewall

Terapkan aturan sesuai interface/alamat nyata; contoh berikut harus disesuaikan:

```sh
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow from <OPERATOR_IP> to any port 22 proto tcp
sudo ufw allow from <PANEL_CLIENT_CIDR> to any port 8080 proto tcp
sudo ufw allow 25565:25749/tcp
sudo ufw enable
sudo ufw status numbered
```

Pastikan akses SSH dari terminal kedua masih aktif sebelum menutup session lama.
Jangan membuka port PostgreSQL, Redis, controller, atau agent; Compose menaruhnya
pada network internal dan hanya web serta alokasi game yang perlu keluar.

## 5. Backup dan restore operasional

Backup in-panel berada di `/var/lib/mypanel/backups`. Salin secara berkala ke
storage lain bersama dump database:

```sh
docker compose exec -T postgres pg_dump -U mypanel -d mypanel -Fc > mypanel.dump
sudo tar -C /var/lib -czf mypanel-data.tar.gz mypanel
```

Uji restore di VM terpisah. Backup pada disk VM yang sama tidak melindungi dari
kerusakan disk atau kehilangan VM.

## 6. Update dan rollback

```sh
docker compose config --quiet
docker compose build
docker compose run --rm migrate
docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

Migrasi bersifat additive dan dijalankan sebelum controller. Simpan snapshot
database/data sebelum update. Jika health gagal, periksa
`docker compose logs --tail=200 migrate bootstrap controller agent` sebelum
mengubah state atau menghapus volume.

Sertifikat leaf mTLS berlaku satu tahun. Jadwalkan maintenance sebelum masa
berlaku habis: hentikan stack, backup state, hapus hanya volume sertifikat
MyPanel, lalu jalankan stack kembali agar CA dan leaf certificate dibuat ulang.
Browser tidak terpengaruh karena sertifikat ini hanya digunakan pada network
controller-agent.

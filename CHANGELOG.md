# 📜 Catatan Rilis (Changelog)

Semua perubahan penting pada proyek **MyPanel V2** dicatat dalam dokumen ini.

Format pencatatan mengikuti panduan [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) dan penomoran versi mengacu pada [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## 🚀 [v1.0.0] — 2026-09-19

### 🎨 Desain & Tampilan (Minecraft Themed UI)
- **Minecraft Themed Landing Page**: Desain landing page publik baru dengan kartu showcase server interaktif, preview status `🟢 ONLINE`, logo Minecraft Grass Cube, live meter TPS 20.0, format MOTD Minecraft asli, dan tombol 1-klik copy IP.
- **Cinematic Login Page**: Halaman login bertema senja portal nether dengan aurora borealis, kartu obsidian glassmorphic, switch instan masuk/daftar, dan tombol intip sandi (*password toggle*).
- **Aset Grafis Lokal & Kepatuhan CSP**: Integrasi visual resolusi tinggi (floating island castle, nether portal, obsidian datacenter diorama) yang dibundel langsung via Vite untuk memenuhi `Content-Security-Policy`.

### ✨ Fitur Baru (Added)
- **CurseForge Modpack Discovery**: Pencarian dan instalasi modpack CurseForge dengan penyesuaian otomatis Forge/NeoForge, Minecraft, dan Java; konfirmasi nama server, backup pra-instalasi otomatis, dan rollback saat kegagalan boot.
- **Telemetri & Metrik Batch**: Pengambilan metrik server batch, Minecraft Server List Ping (pemain online dan latency), riwayat 24 jam resolusi 1 menit, rollup 7 hari, dan sistem notifikasi alert in-panel.
- **Manajemen Add-on Modrinth Terkelola**: Pencarian plugin/mod terverifikasi dari Modrinth, dependency resolution, SHA-512 checksum verification, dan penanda restart-required.
- **File Manager & Backup Terkelola**: Pembuatan folder aman, rename/move terproteksi dari path traversal, backup terkompresi yang dapat diunduh, snapshot pra-restore, dan penjadwalan otomatis.
- **Multi-Tenant Hosting & Billing Simulasi**: Registrasi multi-user, RBAC Owner vs User, paket hosting tematik (Starter, Iron, Gold, Diamond), alokasi atomik ber-advisory lock, langganan 30 hari, dan masa tenggang 7 hari.
- **Pembaruan Satu Perintah (`scripts/update.sh`)**: Skrip fast-forward update otomatis dengan build image, migrasi eksplisit, rollout container, dan verifikasi kesiapan health check.

### 🐛 Perbaikan & Optimasi (Fixed & Optimized)
- **Zero Buffering Console**: Live console kini menggunakan satu persistent Docker stream dengan translasi kode warna Minecraft (`§`), ANSI true-color, dan MiniMessage tanpa latency polling 100ms.
- **Named Pipe Execution**: Eksekusi perintah console dilakukan melalui FIFO worker persisten tanpa overhead spawning proses `docker exec`.
- **Entity Explosion Cache**: Server Paper dan Purpur menerima patch startup internal untuk mengoptimalkan pemrosesan ledakan TNT tanpa memakan kuota disk pengguna.
- **Resource Limits & Burst**: CPU dibatasi sesuai kapasitas vCPU aktual (0–100% per core) dengan burst 25% sementara saat fase booting untuk mempercepat kesiapan server.

---

## 📦 [v0.1.0] — 2026-08-20

### ✨ Fitur Awal (Added)
- **Arsitektur Control Plane Terisolasi**: Go Controller dengan autentikasi Argon2id, session opaque di Redis, validasi CSRF, dan audit log persisten di PostgreSQL.
- **Node Agent via mTLS**: Komunikasi controller-to-agent privat berbasis sertifikat x509 yang dikeluarkan oleh internal CA.
- **Runtime Sandboxing**: Dukungan container Docker terisolasi untuk Vanilla, Paper, Purpur, Fabric, Forge, dan NeoForge dengan alokasi Java 21/25.
- **Web Console xterm.js**: Streaming terminal interaktif melalui WebSocket terotentikasi.
- **Dashboard Awal**: Tampilan manajemen server tunggal dengan pengaturan startup dan file manager.
- **Compose Hardening**: Deployment dengan secret terisolasi, jaringan terpisah, dan kapabilitas Linux minimum.

---

[v1.0.0]: https://github.com/rekis-0103/MyPanel-V2/releases/tag/v1.0.0
[v0.1.0]: https://github.com/rekis-0103/MyPanel-V2/releases/tag/v0.1.0

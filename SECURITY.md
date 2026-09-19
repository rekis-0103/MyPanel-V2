# 🔐 Kebijakan Keamanan (Security Policy)

Keamanan adalah prioritas utama dalam perancangan arsitektur **MyPanel V2**. Kami menerapkan pemisahan hak akses berprinsip *least-privilege* dan isolasi antar-komponen untuk melindungi host dan workload server Minecraft.

---

## 📦 Versi yang Didukung

Pembaruan keamanan dan perbaikan celah kritis diterapkan pada branch default (`main` / `feat/mypanel-v1`) dan rilis terbaru:

| Versi | Didukung | Keterangan |
| :--- | :---: | :--- |
| **v0.2.x (Latest)** | ✅ | Didukung penuh untuk perbaikan bug dan keamanan. |
| **v0.1.x** | ⚠️ | Disarankan segera memperbarui ke versi terbaru. |
| **< v0.1.0** | ❌ | Tidak lagi menerima patch. |

---

## 🚨 Melaporkan Kerentanan Keamanan

> [!CAUTION]
> **JANGAN PERNAH** mempublikasikan dugaan kerentanan keamanan melalui issue publik, pull request, forum publik, atau media sosial.

Gunakan fasilitas **GitHub Security Advisories** resmi pada repository ini:
1. Buka tab **Security** di bagian atas repository GitHub.
2. Klik **Advisories** pada panel sebelah kiri.
3. Klik tombol **Report a vulnerability**.

### Informasi yang Perlu Disertakan:
- **Komponen Terdampak**: Controller, Node Agent, Web Frontend, atau Docker runtime.
- **Versi / Commit SHA**: Commit spesifik tempat kerentanan ditemukan.
- **Langkah Reproduksi (PoC)**: Langkah minimal dan bukti konsep (*Proof of Concept*) yang aman dan sudah disanitasi.
- **Dampak Potensial**: Risiko privilege escalation, data leak, atau denial-of-service.
- **Saran Mitigasi**: Solusi atau patch sementara bila Anda telah menemukannya.

> [!IMPORTANT]
> Jangan menyertakan kredensial nyata, private key TLS, file `.env`, world pemain aktual, data pribadi, atau dump basis data produksi dalam laporan Anda.

---

## 🛡️ Batas Tanggung Jawab Operasional (Shared Responsibility)

MyPanel dirancang dengan pertahanan berlapis, namun operator host tetap memiliki peran krusial dalam mengamankan lingkungan produksi:

| Area | Tanggung Jawab MyPanel | Tanggung Jawab Operator Host |
| :--- | :--- | :--- |
| **Isolasi Docker** | mTLS agent privat, alokasi cgroups, drop capabilities, non-root user. | Hardening kernel Linux, patch keamanan OS, pembatasan akses daemon Docker. |
| **Akses Jaringan** | Binding port otomatis, validasi origin, session opaque, CSRF token. | Konfigurasi firewall (UFW/iptables), isolasi subnet VM, sertifikat HTTPS publik. |
| **Kredensial** | Argon2id hashing, secrets disimpan di file terisolasi tanpa akses Git. | Pengamanan kunci SSH, rotasi password admin, perlindungan file `secrets/`. |
| **Data & Backup** | Backup terkompresi lokal, verifikasi checksum pra-restore, snapshot. | Pencadangan berkala ke storage off-site (S3/NAS), pengujian disaster recovery. |

Panduan lengkap konfigurasi pengamanan VM produksi dapat dilihat di **[docs/runbooks/vm-hardening.md](docs/runbooks/vm-hardening.md)**.

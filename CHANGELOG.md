# Changelog

Perubahan penting proyek ini dicatat di sini. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) dan versi mengikuti
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Pilihan Java 21 atau Java 25 per server, dengan image allowlist di agent dan
  migrasi kompatibel untuk server lama.
- Script update VM berbasis fast-forward dengan build, migrasi eksplisit,
  rollout Compose, readiness check, serta systemd timer opsional.
- Navigasi server kontekstual, mode terang/gelap, grafik metric pada console,
  alamat server yang mengikuti hostname panel, dan pengaturan startup tervalidasi.

### Fixed

- Live console memakai satu stream Docker persisten dan langsung merender tiap
  pesan tanpa polling 100 ms atau buffer browser 120 ms; antrean xterm tetap
  memberi backpressure saat burst besar. Command berjalan pada worker terpisah
  dan masuk melalui named pipe bind-mounted tanpa membuat Docker exec baru;
  pipe internal dilindungi dari file manager. Sampel CPU dibatasi pada kapasitas
  vCPU container, dan pengecekan startup yang duplikatif dikurangi.
- Riwayat console sekarang mengurutkan log Minecraft dan event MyPanel memakai
  timestamp yang sama, log burst dibaca dengan cursor tanpa tail-reset, dan
  readiness tidak lagi menunggu pengambilan sampel CPU atau interval healthcheck
  setelah marker ready Paper dari boot saat ini muncul.
- Update yang gagal setelah fast-forward sekarang dicoba kembali sampai commit
  tersebut berhasil melewati readiness check.
- Container Minecraft dapat menyiapkan ownership data dan berpindah ke user
  non-root dengan capability bootstrap minimum.
- CPU dan working-memory server dihitung sesuai limit/cgroup, console dikirim
  incremental dengan dukungan warna aman, dan command tidak lagi membuat noise
  koneksi RCON per perintah.
- Console tidak lagi menampilkan timestamp Docker ganda, level log dan nama
  plugin diberi warna semantik, dan CPU mengikuti persentase core Docker tanpa
  normalisasi kedua yang mengecilkan nilainya.
- Status server tetap memulai sampai health Minecraft siap, command diblokir
  selama startup, dan terminal mengikuti background serta palette light mode.
- Console menerjemahkan warna plugin dari ANSI, kode Minecraft/legacy, RGB,
  serta tag MiniMessage, dan meminta output Adventure true-color tanpa
  mengizinkan kontrol terminal non-warna.
- File yang diunggah panel memakai group Minecraft (`GID 1000`), sehingga JAR
  plugin dapat dibaca dan ditemukan Paper saat server dimulai.

## [0.1.0] - 2026-08-20

### Added

- Control plane single-owner dengan autentikasi Argon2id, session Redis, CSRF,
  origin validation, rate limit, dan audit activity.
- Job durable untuk provisioning dan lifecycle server, rekonsiliasi state,
  alokasi port, serta limit resource.
- Node agent mTLS dengan operasi Docker yang dibatasi untuk server terkelola.
- Runtime Vanilla, Paper, Purpur, Fabric, Forge, dan NeoForge.
- Console WebSocket, file manager aman, backup/restore, schedule, settings,
  metrics, dan katalog runtime.
- Dashboard React responsif yang terhubung ke API nyata.
- Deployment Compose ter-harden dengan secret files, network terpisah,
  filesystem read-only, capability drop, health check, dan log rotation.

[Unreleased]: https://github.com/rekis-0103/MyPanel-V2/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/rekis-0103/MyPanel-V2/releases/tag/v0.1.0

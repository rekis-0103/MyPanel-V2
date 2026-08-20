# Changelog

Perubahan penting proyek ini dicatat di sini. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) dan versi mengikuti
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

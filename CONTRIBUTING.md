# Contributing to MyPanel V2

Terima kasih telah membantu MyPanel. Perubahan sebaiknya kecil, dapat ditinjau,
dan tetap mempertahankan boundary keamanan antara browser, controller, agent,
dan Docker host.

## Sebelum mulai

1. Cari issue yang sudah ada sebelum membuka issue baru.
2. Untuk perubahan besar, jelaskan masalah, rancangan, migrasi, dan dampak
   kompatibilitas terlebih dahulu di issue.
3. Jangan memasukkan password, token, private key, sertifikat, `.env`, dump
   database, world, atau backup server ke repository.

## Menjalankan proyek

Persyaratan dan instalasi utama ada di [README.md](README.md). Untuk konfigurasi
lokal, salin `.env.example` menjadi `.env` dan hasilkan secret dengan script di
`scripts/`. Gunakan nilai lokal yang unik; file aktual diabaikan Git.

## Alur perubahan

1. Buat branch dari branch default dengan nama yang menjelaskan tujuan,
   misalnya `feat/sftp-access` atau `fix/restore-validation`.
2. Ikuti pola dan dependency yang sudah dipakai proyek. Hindari menambahkan
   dependency bila standard library atau dependency yang ada sudah memadai.
3. Tambahkan atau perbarui test untuk perilaku yang berubah.
4. Perbarui dokumentasi ketika API, konfigurasi, deployment, migrasi, atau
   perilaku operator berubah.
5. Buat commit terfokus dengan pesan imperatif, misalnya
   `feat(agent): validate backup destination`.

## Quality gate

Backend:

```sh
cd controller
go test -race ./...
go vet ./...
```

Frontend:

```sh
cd web
corepack enable
pnpm install --frozen-lockfile
pnpm lint
pnpm test
pnpm build
```

Deployment:

```sh
docker compose config --quiet
```

Pull request harus menjelaskan tujuan, perubahan perilaku, risiko keamanan atau
migrasi, dan hasil command validasi. Sertakan screenshot untuk perubahan UI yang
terlihat.

## Pedoman keamanan

- Controller publik tidak boleh menerima Docker socket.
- Operasi Docker baru harus melalui agent, allowlist operasi, validasi UUID,
  dan boundary mTLS yang sudah ada.
- Endpoint mutasi browser harus mempertahankan session, CSRF, origin check,
  validasi input, ownership, dan audit event yang relevan.
- Path file harus tetap berada di root server dan tidak boleh lolos melalui
  traversal atau symlink.
- Pesan error dari node atau dependency tidak boleh membocorkan secret atau
  detail host sensitif ke browser.

Kerentanan harus dilaporkan mengikuti [SECURITY.md](SECURITY.md), bukan melalui
issue publik.

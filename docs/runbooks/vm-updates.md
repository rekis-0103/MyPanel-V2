# VM Update Runbook

Runbook ini memperbarui checkout `~/MyPanel-V2` dari branch GitHub yang sedang
dipakai tanpa menghapus `.env`, secret, world, backup, atau named volume.

## Update manual

Pastikan perubahan yang ingin dipasang sudah di-push ke branch GitHub yang
dilacak VM, lalu jalankan:

```sh
cd ~/MyPanel-V2
sh scripts/update.sh
```

Script menolak kondisi berikut agar update tidak menimpa pekerjaan lokal:

- file tracked atau untracked yang belum di-commit;
- branch remote yang hilang;
- histori remote yang memerlukan merge atau force update;
- update lain yang masih berjalan.

Urutan rollout adalah fetch, fast-forward, validasi Compose, build image,
migrasi additive/idempotent, penggantian container, dan readiness check selama
maksimal 60 detik. Jika build gagal, container lama tetap berjalan. Jika rollout
atau readiness gagal, periksa output dan log yang dicetak script sebelum mencoba
perubahan lain.

Status dan log dapat diperiksa dengan:

```sh
docker compose ps
docker compose logs --tail=200 controller agent web migrate
```

Jalankan ulang rollout tanpa commit Git baru, misalnya untuk menarik base image
baru:

```sh
MYPANEL_UPDATE_FORCE=1 sh scripts/update.sh
```

## Update otomatis dengan systemd user timer

Template bawaan mengasumsikan repository berada di `~/MyPanel-V2` dan mengikuti
branch yang sedang di-checkout. Jika lokasi repository berbeda, edit file
service sebelum menyalinnya. Untuk memaksa branch tertentu, tambahkan
`Environment=MYPANEL_UPDATE_BRANCH=<branch>` pada bagian `[Service]`.

```sh
mkdir -p ~/.config/systemd/user
cp deploy/systemd/mypanel-update.service ~/.config/systemd/user/
cp deploy/systemd/mypanel-update.timer ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now mypanel-update.timer
systemctl --user list-timers mypanel-update.timer
```

Agar timer user tetap berjalan setelah logout SSH, aktifkan linger satu kali:

```sh
sudo loginctl enable-linger "$USER"
```

Pantau eksekusi:

```sh
systemctl --user status mypanel-update.timer
journalctl --user -u mypanel-update.service -n 100 --no-pager
```

Jalankan update segera tanpa menunggu timer:

```sh
systemctl --user start mypanel-update.service
journalctl --user -u mypanel-update.service -n 100 --no-pager
```

Nonaktifkan otomatisasi:

```sh
systemctl --user disable --now mypanel-update.timer
```

## Keamanan dan rollback

Siapa pun yang dapat menulis ke branch deployment secara efektif dapat
menjalankan kode pada Docker host saat timer menerapkan update. Lindungi akun
GitHub dengan MFA, batasi maintainer, aktifkan branch protection, dan review
perubahan sebelum merge. Jangan menyimpan credential push atau password SSH di
unit systemd.

Script sengaja tidak melakukan rollback Git otomatis. Migrasi bersifat additive
dan container lama tetap kompatibel dengan kolom baru, tetapi rollback harus
menggunakan commit yang sudah ditinjau dan diuji. Ambil backup database/world
sebelum rilis yang mengubah schema atau format data.

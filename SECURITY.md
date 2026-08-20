# Security Policy

## Versi yang didukung

MyPanel masih berada pada tahap awal. Perbaikan keamanan diterapkan pada branch
default dan rilis terbaru; versi lama tidak menerima backport terjadwal.

## Melaporkan kerentanan

Jangan membuka issue publik untuk dugaan kerentanan. Gunakan **GitHub Security
Advisories** pada menu `Security` repository ini dan pilih `Report a
vulnerability`. Sertakan:

- versi atau commit yang terdampak;
- prasyarat dan langkah reproduksi minimal;
- dampak yang diamati;
- bukti konsep yang aman dan sudah disanitasi;
- saran mitigasi bila tersedia.

Jangan menyertakan kredensial nyata, private key, world pemain, data pribadi,
atau dump database. Pemilik proyek akan mengonfirmasi penerimaan dan menilai
laporan secepat yang memungkinkan, tetapi belum menjanjikan SLA respons.

## Batas keamanan operasional

MyPanel mengelola workload Minecraft melalui agent yang memiliki akses Docker.
Operator tetap bertanggung jawab atas hardening VM, patch host, firewall,
backup off-site, TLS/VPN untuk akses browser, rotasi secret, dan pembatasan SSH.
Ikuti [runbook hardening](docs/runbooks/vm-hardening.md) sebelum mengekspos panel.

Credential atau private key yang pernah masuk commit harus dianggap bocor:
hapus dari penggunaan, rotasi segera, lalu bersihkan riwayat dengan koordinasi
maintainer. Menambahkan file tersebut ke `.gitignore` saja tidak mencabut
credential yang sudah dipublikasikan.

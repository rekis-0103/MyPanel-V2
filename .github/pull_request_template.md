## 📝 Deskripsi Perubahan

<!-- Jelaskan secara singkat masalah yang diselesaikan dan fitur/perilaku yang ditambahkan atau diubah. -->

---

## 🎯 Jenis Perubahan

- [ ] 🚀 Fitur Baru (`feat`)
- [ ] 🐛 Perbaikan Bug (`fix`)
- [ ] 🎨 Perubahan Tampilan / UI (`style`)
- [ ] ♻️ Refaktor Kode (`refactor`)
- [ ] ⚡ Optimasi Performa (`perf`)
- [ ] 📚 Dokumentasi (`docs`)
- [ ] 🔧 Konfigurasi & CI/CD (`chore`)

---

## 🖼️ Tangkapan Layar / Preview (Bila Ada Perubahan UI)

<details>
<summary>📸 Klik untuk melihat tangkapan layar</summary>

<!-- Sisipkan gambar tangkapan layar atau GIF demo di sini -->

</details>

---

## ✅ Daftar Cek Validasi (Quality Gates)

- [ ] Test unit backend lulus (`cd controller && go test -race ./...`)
- [ ] Static check backend lulus (`cd controller && go vet ./...`)
- [ ] Lint & build frontend lulus (`cd web && pnpm lint && pnpm test && pnpm build`)
- [ ] Validasi konfigurasi Compose lulus (`docker compose config --quiet`)
- [ ] Migrasi database teruji bersifat aditif & idempotent (bila ada skema baru)
- [ ] **TIDAK ADA** secret, password, token, dump database, atau private key yang ter-commit

---

## ⚠️ Risiko & Dampak Operasional

<!-- Jelaskan potensi dampak terhadap kompatibilitas mundur, migrasi data, downtime, atau konfigurasi lingkungan host. Tulis "Tidak ada" bila tidak ada dampak khusus. -->

# CurseForge modpack management

## Goal

Allow an authenticated server owner to search CurseForge modpacks, choose an
actually published Forge or NeoForge file, review the runtime/version change,
and replace the existing server through a confirmed durable operation.

## Acceptance criteria

- [x] Search is restricted to CurseForge Minecraft modpacks.
- [x] Version choices come from CurseForge file metadata and expose loader,
      Minecraft version, release channel, and required Java version.
- [x] Installation requires the exact server name as confirmation.
- [x] Runtime, Minecraft version, and Java are derived by the controller.
- [x] A pre-change backup is created before the runtime is replaced.
- [x] The selected CurseForge file is pinned in the Minecraft container.
- [x] World data and the existing resource allocation are retained.
- [x] Ownership, subscription state, active-job, and provider availability
      checks remain enforced server-side.
- [x] UI covers loading, empty, incompatible, confirmation, queued, installed,
      and failed states.
- [!] Backend/frontend tests, static checks, and production build pass. Compose
      validation and race tests require the Linux VM because Docker and CGO are
      unavailable in the current Windows environment.

## Implementation notes

- Store one managed modpack row per server. The row is additive and removed
  automatically with its server.
- CurseForge project and file IDs are re-resolved during installation; the
  browser cannot submit a runtime or Minecraft version.
- Only Forge and NeoForge files with a panel-supported Minecraft version are
  offered. Client/server-pack files are excluded from the selectable manifest
  list because AUTO_CURSEFORGE requires the regular manifest file.
- Use `TYPE=AUTO_CURSEFORGE`, `CF_SLUG`, and `CF_FILE_ID` in the managed
  Minecraft container. No CurseForge credential is returned to the browser or
  placed in a job payload.
- If backup or provisioning fails, restore the previous server definition in
  PostgreSQL and attempt to recreate the previous container definition.

## Verification record

- `go test ./...` passed.
- `go vet ./...` passed.
- `pnpm lint` passed.
- `pnpm test` passed: 13 files, 49 tests.
- `pnpm build` passed with Vite 8.2.1.
- `go test -race ./...` was not run: the local Go toolchain has CGO disabled.
- `docker compose config --quiet` and a live migration/install smoke test were
  not run: Docker CLI is unavailable in the current environment.

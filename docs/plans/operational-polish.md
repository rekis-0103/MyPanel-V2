# Operational Polish Milestones

## Goal

Deliver the P0 stability/UI fixes and the P1 operational features agreed for
MyPanel's serious single-node hosting simulation.

## Constraints and decisions

- Existing authentication, ownership, CSRF, job, and `/api/v1` compatibility
  boundaries remain enforced.
- Modrinth is enabled by default. CurseForge is optional and file-secret based.
- Spigot, email/webhook alerts, real payments, and multi-node placement are out
  of scope.
- Metrics retain one-minute samples for 24 hours and 15-minute samples for seven
  days. Alerts are in-panel only.
- Changes are not committed, pushed, merged, or deployed unless separately
  requested.

## Milestone status

| Milestone | Status | Dependencies | Verification |
|---|---|---|---|
| P0 stability and UI | complete | None | Go/frontend gates and browser smoke passed |
| P1 metrics and notifications | complete | P0 | Unit tests, vet, typecheck, and build passed |
| P1 add-on manager | complete | P0 | Provider/checksum/path regression tests passed |
| P1 files, backups, schedules | complete | P0 | Agent and frontend regression tests passed |
| CI, documentation, final review | complete | P0/P1 | Local gates passed; VM Compose check remains operational |

## P0 stability and UI

- [x] Keep valid sessions when non-auth bootstrap data fails and expose retry.
- [x] Scope lifecycle pending state to the affected server/action.
- [x] Add batch server metrics and accurate host CPU/RAM telemetry.
- [x] Drive admin server creation from catalog and available capacity.
- [x] Repair mobile contextual navigation, accessibility, readable typography,
      stale file state, dirty settings, and destructive confirmations.

## P1 metrics and notifications

- [x] Add Minecraft status ping and compatible player/latency fields.
- [x] Persist and query 24-hour/7-day metric history.
- [x] Persist deduplicated in-panel operational notifications and expose read
      APIs/UI.

## P1 add-on manager

- [x] Add provider adapters for Modrinth and optional CurseForge.
- [x] Add secure durable install/update/remove jobs and managed add-on state.
- [x] Add runtime-aware search, inventory, dependency confirmation, and restart
      required UI.

## P1 files, backups, schedules

- [x] Add safe create-folder and move/rename operations.
- [x] Verify restore checksums, create pre-restore backups, stream downloads,
      and enforce scheduled-backup retention.
- [x] Add schedule update, enable/disable, run-now, and last-job visibility.

## Finalization

- [x] Add GitHub Actions checks.
- [x] Update API, architecture, README, changelog, and this status document.
- [x] Run targeted tests, full Go/frontend gates, migration/Compose checks where
      available, and review the complete diff for secrets and regressions.

## Baseline evidence

- `go test ./...`: passed.
- `go vet ./...`: passed.
- `pnpm lint`, `pnpm test` (47 tests), and `pnpm build`: passed.
- `docker compose config --quiet`: unavailable on the Windows workstation
  because Docker CLI is not installed; must be verified on the Linux VM.

## Final evidence

- `go test ./...`: passed.
- `go vet ./...`: passed.
- `pnpm lint`: passed.
- `pnpm test`: 12 files and 48 tests passed.
- `pnpm build`: passed with Vite 8.2.1.
- Browser smoke: landing page rendered, semantic navigation was present, and
  the primary sign-in link opened `/login` with username focus.
- `git diff --check`: passed; only the workstation's expected LF-to-CRLF
  conversion notices were reported.
- `go test -race ./...`: not runnable locally because the Windows environment
  has neither an enabled CGO toolchain nor `gcc`; the GitHub Actions gate runs
  the race suite on Ubuntu.
- Migration execution and `docker compose config --quiet`: not runnable locally
  because PostgreSQL and Docker CLI are unavailable. Both must run during the
  VM update; Compose syntax is also covered by the added GitHub Actions job.

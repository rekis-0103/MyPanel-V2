# MyPanel V1 Execution Plan

Target: a secure, single-owner Minecraft Java panel on one Linux VM, using a
separate controller and privileged node agent so the public web/API process
never receives the Docker socket.

## Milestones

- [x] M1 — Versioned schema, owner authentication, server-side sessions,
  CSRF/rate limiting, audit events, and live/ready health checks.
- [x] M2 — mTLS node agent, durable asynchronous jobs, reconciliation,
  allocations, persistent server directories, and enforced resource limits.
- [x] M3 — WebSocket console, commands, safe files, backup/restore, schedules,
  server configuration, metrics, and runtime catalog.
- [x] M4 — Responsive state-complete React UI integrated with the real APIs.
- [x] M5 — Hardened Compose/Caddy deployment, secret-file support, separate
  networks, bounded logs, and a lockout-safe VM hardening runbook.
- [x] M6 — Unit/integration/frontend coverage, production builds, Compose and
  migration validation, diff/security review, and current documentation.

## Acceptance criteria

- The controller runs non-root and has no Docker socket; only the node agent
  can perform allowlisted operations against managed server UUIDs.
- Every mutating browser request requires an authenticated owner session and
  CSRF token. Login is rate-limited and logout invalidates the server session.
- Provision/start/stop/restart/delete are durable jobs, survive browser
  disconnects, and converge the database state to actual agent state.
- Every Minecraft server mounts an agent-managed persistent `/data` directory,
  has an explicit allocation, and receives CPU, total-memory, heap-headroom,
  PID, and disk limits.
- Console, files, backup/restore, schedules, settings, metrics, and audit views
  have usable loading, empty, pending, success, and error behavior.
- Existing database rows are preserved and backfilled onto a default node.
- All documented quality-gate commands pass, or any genuine environment-only
  limitation is recorded with evidence.

## Operational boundary

Rotating real credentials, changing SSH authentication, and enabling firewall
rules remain explicit runbook actions. They must only be applied after a second
verified SSH-key session is open, so implementation cannot lock the owner out.

## Verification record

- `go test ./...` and `go vet ./...` passed locally.
- `go test -race ./...` passed in the Linux Go 1.26 container.
- `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm test`, and `pnpm build`
  passed; eight test files contain 26 passing frontend tests.
- PostgreSQL 17 applied migrations 001 and 002 to version 2.
- A temporary full Compose stack passed readiness, owner login/session/CSRF
  logout, mTLS agent connectivity, internal-network, and Docker-socket boundary
  checks. The temporary stack and its volumes were removed after validation.
- Browser smoke checks passed at desktop and 390×844 without console errors or
  horizontal overflow.

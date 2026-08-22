# MyPanel V1 Architecture

## Trust boundaries

- **Web** serves the React application and proxies `/api` to the controller.
- **Controller** is the public application boundary. It authenticates the owner,
  validates every request, stores durable state in PostgreSQL, stores ephemeral
  sessions/rate limits in Redis, and never receives the Docker socket.
- **Agent** is the privileged node boundary. It is reachable only on the
  private control network, requires a certificate issued by the MyPanel CA, and
  accepts only typed operations for MyPanel-managed server UUIDs.
- **Minecraft containers** are untrusted workloads with explicit CPU, memory,
  PID, port, and data-directory constraints.

## Data flow

1. A browser mutation is authenticated by an opaque server-side session and a
   matching CSRF token.
2. The controller validates ownership, capacity, allocation, and state, then
   inserts a PostgreSQL job and returns `202 Accepted`.
3. A background worker atomically claims the job, calls the node agent over
   mTLS, and records completion or a bounded error.
4. A reconciler independently inspects the actual agent state and updates
   `observed_state`; desired state is never treated as observed state.
5. The UI receives job/state changes through normal polling and console events
   over an authenticated WebSocket.

## Frontend structure

- React Router exposes stable deep links for dashboard, server list, console,
  files, backups, schedules, settings, and activity. Server cards enter the
  console directly; server-specific navigation is only rendered on a server
  route. Caddy's SPA
  fallback serves `index.html` for direct route access.
- `App` retains authentication, catalog/server loading, lifecycle actions, and
  polling. Page and layout components consume those operations without
  duplicating controller business rules.
- Shared UI primitives and CSS design tokens provide status, metrics, actions,
  modal confirmation, toast feedback, loading, empty, and error states.
- The interface defaults to Indonesian and can switch to English. Only the
  locale and last selected server ID are stored in browser local storage; no
  credential or session token is persisted there.
- Console rendering uses xterm.js with an authenticated incremental WebSocket
  stream. Safe SGR and Minecraft color codes are rendered, while cursor/title
  control sequences are removed. Host-wide telemetry is intentionally presented as
  unavailable until the controller exposes an authoritative endpoint.

## Public contracts

- Existing `/api/v1` paths remain the public namespace.
- Mutating lifecycle responses use `{ "server": Server, "job": Job }` and HTTP
  202. `GET /api/v1/jobs/{id}` exposes durable progress.
- `Server.state` is the observed state. New fields include `desiredState`,
  `nodeId`, `diskMb`, `bindIp`, `currentJob`, and `lastError`.
- `/api/v1/servers/{id}/console` is a WebSocket carrying `log`, `status`, and
  `command-result` messages. Browser commands are sent as `command` messages.
- Agent routes are under `/v1/servers/{uuid}` and are not browser-accessible.

## Storage and lifecycle

- Server data lives at `/var/lib/mypanel/servers/<uuid>` and is mounted at
  `/data`; backups live under `/var/lib/mypanel/backups/<uuid>`.
- The root agent runs with primary GID `1000`, matching the Minecraft runtime,
  so uploaded files and newly created directories remain readable by the
  server while retaining restrictive group-based modes.
- Total container memory is the user allocation. JVM maximum heap defaults to
  80% of that allocation to leave native-memory headroom.
- Server CPU follows Docker's core-relative percentage: 100% represents one
  fully used vCPU, so a two-vCPU server can reach 200%. Working RAM subtracts
  `inactive_file` on cgroup v2 (falling back to `cache`), and disk usage includes
  regular files only within the managed server root.
- Docker timestamps are disabled at the log source because Minecraft already
  emits its own timestamp. The browser applies safe semantic ANSI colors to
  Minecraft levels and plugin tags while preserving validated ANSI SGR colors
  and translating Minecraft `§`, plugin legacy `&`/`&x`, and supported
  MiniMessage color/decorations. Managed JVM defaults request Adventure
  true-color output while preserving user JVM options and advertise an
  `xterm-256color`/true-color terminal; non-SGR terminal controls are stripped.
- The web Content Security Policy keeps scripts restricted to same-origin
  files. Inline style elements are allowed because xterm.js generates a scoped
  runtime stylesheet for its ANSI palette; inline script execution remains
  disallowed.
- Console commands use the image's named console pipe as UID/GID 1000 instead
  of opening one RCON connection per command. Runtime updates enable stdin and
  create that pipe; existing containers receive it when their config is next
  applied.
- A running Docker process remains `starting` while its Minecraft healthcheck is
  not healthy. Start and restart jobs wait for readiness before completing, and
  both the browser and WebSocket command boundary reject commands until the
  observed state is `running`.
- Lifecycle messages are persisted in `server_console_events` and streamed as
  separate WebSocket events. The browser renders them orange, retains the 200
  latest events per server, and strips embedded terminal controls. Failure
  reasons are limited to safe categories such as OOM, exit code, disk limit,
  health timeout, and node availability instead of exposing raw host errors.
- Startup settings persist validated `jvmOpts` and `extraArgs` in the existing
  JSON config. The agent maps them only to the image-supported `JVM_OPTS` and
  `EXTRA_ARGS` variables; arbitrary shell commands and images remain disallowed.
- Java version is persisted per server and restricted to the controller/agent
  allowlist. The agent maps Java 21 and 25 to operator-configured image names;
  arbitrary container images never cross the browser trust boundary.
- Minecraft containers drop every Linux capability, then add only `CHOWN`,
  `SETGID`, and `SETUID` so the image entrypoint can prepare `/data` and switch
  to its unprivileged `minecraft` user. `no-new-privileges` remains enabled.
- The node agent drops every capability except `DAC_OVERRIDE`. Minecraft changes
  its data directory to UID/GID 1000 with restrictive modes, so this single
  capability lets the agent perform the authenticated file, quota, backup, and
  purge operations without granting capabilities to the public controller or
  web containers.
- Delete is a durable job. It stops and removes the managed container, and data
  is removed only when the request explicitly sets `purgeData: true`.
- Hanya satu job aktif diizinkan per server. Operasi yang bersaing ditolak dan
  browser disconnect tidak membatalkan job yang sedang berjalan.

## Deployment

The default Compose topology has separate edge, control, and data networks.
Certificate material and passwords are mounted as files. A network-isolated
init container copies controller secrets to a private volume owned by the
controller's non-root UID. Development may bind the web port to a host-only
address; production access requires TLS or a VPN.

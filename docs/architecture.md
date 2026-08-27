# MyPanel V2 Architecture

## Trust boundaries

- **Web** serves the React application and proxies `/api` to the controller.
- **Controller** is the public application boundary. It authenticates owners and users,
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
6. Checkout atomically reserves package capacity, port, ownership, subscription,
   simulated order, and provisioning job under a PostgreSQL advisory lock.

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
  stream. Initial Docker logs and persisted lifecycle events are merged by
  timestamp. Live Docker output then crosses one persistent agent stream and is
  written immediately, one message at a time, through a backpressured xterm
  queue. Safe SGR and Minecraft color codes are rendered, while
  cursor/title control sequences are removed. Host-wide telemetry is read by
  the agent and exposed only to owners through the authenticated capacity API.
- Navigation is role-aware: customers receive marketplace and subscription
  views; administrators receive account, package, capacity, and fleet controls.

## Public contracts

- Existing `/api/v1` paths remain the public namespace.
- Mutating lifecycle responses use `{ "server": Server, "job": Job }` and HTTP
  202. `GET /api/v1/jobs/{id}` exposes durable progress.
- `Server.state` is the observed state. New fields include `desiredState`,
  `nodeId`, `diskMb`, `bindIp`, `currentJob`, and `lastError`.
- `/api/v1/servers/{id}/console` is a WebSocket carrying `history`, `log`,
  `status`, `lifecycle`, and `command-result` messages. Browser commands are
  sent as `command` messages.
- Agent routes are under `/v1/servers/{uuid}` and are not browser-accessible.
- Every server-scoped controller route resolves authenticated ownership before
  reading or mutating data. Role-aware UI is convenience, not authorization.

## Storage and lifecycle

- Server data lives at `/var/lib/mypanel/servers/<uuid>` and is mounted at
  `/data`; backups live under `/var/lib/mypanel/backups/<uuid>`.
- The root agent runs with primary GID `1000`, matching the Minecraft runtime,
  so uploaded files and newly created directories remain readable by the
  server while retaining restrictive group-based modes.
- Total container memory is the user allocation. JVM maximum heap defaults to
  80% of that allocation to leave native-memory headroom.
- Server CPU follows Docker's core-relative percentage: 100% represents one
  fully used vCPU, so a two-vCPU server can reach 200%. Agent and browser both
  bound transient sampling artifacts to the configured vCPU capacity. Start
  and restart temporarily update the Docker quota to 125% of the allocation;
  the first readiness observation restores the exact runtime quota. Working RAM subtracts
  `inactive_file` on cgroup v2 (falling back to `cache`), and disk usage includes
  regular files only within the managed server root.
- Paper and Purpur containers receive a read-only, agent-generated startup patch
  that enables Paper's entity-lookup cache for explosions. The managed patch
  directory is excluded from browser file operations, backups, and disk quota
  accounting; Vanilla and modded runtimes are unchanged.
- Docker RFC3339Nano timestamps are retained internally as ordering cursors but
  removed before display because Minecraft already emits its own timestamp.
  The initial snapshot is followed by one Docker `follow` stream from the last
  cursor, avoiding repeated requests, burst truncation, and timer-based browser
  batching. Reconnects replay from the cursor and discard duplicates. The browser applies safe
  semantic ANSI colors to
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
  of opening RCON or Docker exec connections per command. The pipe is created
  inside the bind-mounted server root, kept out of file-manager operations, and
  opened with no-follow/type checks by the agent. Existing containers use a
  compatibility fallback until their config is next applied. A bounded,
  single-worker command queue keeps execution ordered while the WebSocket loop
  continues forwarding live Docker output.
- A running Docker process remains `starting` until the current container boot
  emits Paper's `Done (...)! For help, type ...` marker or its Minecraft
  healthcheck becomes healthy. Ready markers older than `State.StartedAt` are
  ignored. The console stream recognizes the current ready line immediately;
  state-only readiness checks skip Docker CPU sampling and run independently
  from slower metric collection. Start and restart jobs wait for that readiness path
  before completing, and both the browser and WebSocket command boundary reject
  commands until the observed state is `running`.
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
- Sessions carry a database-backed version. Suspend, password reset, and
  password change increment it so existing Redis sessions stop working.
- A subscription reserves CPU, RAM, disk, and a port for 30 days. Expiry stops
  the server and enters seven days of grace. After grace, a durable release job
  removes the container and port reservation but retains server data; retained
  disk remains counted. Reactivation allocates a new port around that data.

## Deployment

The default Compose topology has separate edge, control, and data networks.
Certificate material and passwords are mounted as files. A network-isolated
init container copies controller secrets to a private volume owned by the
controller's non-root UID. Development may bind the web port to a host-only
address; production access requires TLS or a VPN.

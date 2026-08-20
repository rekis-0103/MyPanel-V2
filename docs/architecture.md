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
- Total container memory is the user allocation. JVM maximum heap defaults to
  80% of that allocation to leave native-memory headroom.
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

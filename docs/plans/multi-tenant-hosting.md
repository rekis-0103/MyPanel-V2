# Multi-tenant Hosting Milestone

Status: implemented on `feat/multi-tenant-hosting`; pending final PR validation
and deployment.

## Scope and decisions

- Roles are `owner` and `user`; owner authentication remains compatible with
  the bootstrap account.
- Self-registration uses username and password and can be disabled by operator.
- Payment is explicitly simulated. No gateway, card data, webhook, or real
  balance is accepted.
- User access is default-deny outside owned server resources. Owner access spans
  all servers and includes server-ID/owner search.
- Packages are checked against configured CPU, memory, disk, and port capacity
  inside the checkout transaction.
- Periods are 30 days with manual renewal and seven days of grace. Resource
  release preserves server data; no automatic world deletion occurs.

## Acceptance checklist

- [x] Migration adds account state, ownership, packages, orders, subscriptions,
  indexes, constraints, and seed packages.
- [x] Registration, login, session invalidation, password change/reset, and
  account suspension are implemented.
- [x] Server, job, file, backup, schedule, console, and activity access is
  ownership-aware.
- [x] Checkout, renewal, retry, expiry, grace, and retained-data reactivation are
  implemented with durable jobs.
- [x] Customer and administrator UI routes are role-aware and state-complete.
- [x] Targeted backend and frontend regression tests are included.
- [x] Migration executed successfully against the Linux VM database inside an
  explicit transaction that was rolled back.
- Production rollout evidence is recorded in the pull request and deployment
  handoff because it occurs only after merge.
- [ ] Pull request reviewed, merged, and deployed.

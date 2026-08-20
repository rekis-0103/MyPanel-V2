BEGIN;

INSERT INTO nodes (id, name, status, token_hash)
VALUES ('00000000-0000-0000-0000-000000000001', 'local', 'online', 'mtls')
ON CONFLICT (id) DO NOTHING;

ALTER TABLE servers ADD COLUMN IF NOT EXISTS disk_mb INTEGER NOT NULL DEFAULT 10240;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS bind_ip TEXT NOT NULL DEFAULT '0.0.0.0';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE servers ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_desired_state_check;
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_observed_state_check;
ALTER TABLE servers ADD CONSTRAINT servers_desired_state_check
  CHECK (desired_state IN ('running','offline','deleted'));
ALTER TABLE servers ADD CONSTRAINT servers_observed_state_check
  CHECK (observed_state IN ('installing','starting','running','stopping','offline','error','deleting'));
UPDATE servers SET node_id='00000000-0000-0000-0000-000000000001' WHERE node_id IS NULL;
ALTER TABLE servers ALTER COLUMN node_id SET NOT NULL;
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_disk_mb_check;
ALTER TABLE servers ADD CONSTRAINT servers_disk_mb_check CHECK (disk_mb BETWEEN 1024 AND 102400);

CREATE TABLE IF NOT EXISTS allocations (
  id UUID PRIMARY KEY,
  node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  server_id UUID UNIQUE REFERENCES servers(id) ON DELETE SET NULL,
  bind_ip TEXT NOT NULL,
  port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (node_id, bind_ip, port)
);

INSERT INTO allocations (id, node_id, server_id, bind_ip, port)
SELECT gen_random_uuid(), node_id, id, bind_ip, port FROM servers
ON CONFLICT (node_id, bind_ip, port) DO NOTHING;

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS result JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_action_check;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_status_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_action_check
  CHECK (action IN ('provision','start','stop','restart','delete','backup','restore','command','update'));
ALTER TABLE jobs ADD CONSTRAINT jobs_status_check
  CHECK (status IN ('queued','running','completed','failed','canceled'));
DROP INDEX IF EXISTS jobs_one_active_action_idx;
CREATE UNIQUE INDEX IF NOT EXISTS jobs_one_active_server_idx
  ON jobs(server_id) WHERE status IN ('queued','running');

CREATE TABLE IF NOT EXISTS backups (
  id UUID PRIMARY KEY,
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  path TEXT,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  checksum_sha256 TEXT,
  status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','creating','ready','restoring','failed')),
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS backups_server_created_idx ON backups(server_id, created_at DESC);

CREATE TABLE IF NOT EXISTS schedules (
  id UUID PRIMARY KEY,
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('start','stop','restart','backup','command')),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  interval_minutes INTEGER NOT NULL CHECK (interval_minutes BETWEEN 1 AND 525600),
  enabled BOOLEAN NOT NULL DEFAULT true,
  next_run_at TIMESTAMPTZ NOT NULL,
  last_run_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS schedules_due_idx ON schedules(enabled, next_run_at);

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  target_type TEXT,
  target_id TEXT,
  ip TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_created_idx ON audit_events(created_at DESC);

INSERT INTO schema_migrations (version) VALUES (2) ON CONFLICT DO NOTHING;
COMMIT;

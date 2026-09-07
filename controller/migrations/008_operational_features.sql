BEGIN;

ALTER TABLE servers ADD COLUMN IF NOT EXISTS restart_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE backups ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE backups ADD COLUMN IF NOT EXISTS schedule_id UUID REFERENCES schedules(id) ON DELETE SET NULL;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS last_job_id UUID REFERENCES jobs(id) ON DELETE SET NULL;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS server_metric_samples (
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  bucket_at TIMESTAMPTZ NOT NULL,
  resolution TEXT NOT NULL CHECK (resolution IN ('1m','15m')),
  state TEXT NOT NULL,
  cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (cpu_percent >= 0),
  memory_bytes BIGINT NOT NULL DEFAULT 0 CHECK (memory_bytes >= 0),
  disk_bytes BIGINT NOT NULL DEFAULT 0 CHECK (disk_bytes >= 0),
  players_online INTEGER,
  players_max INTEGER,
  latency_ms INTEGER,
  PRIMARY KEY (server_id, resolution, bucket_at)
);
CREATE INDEX IF NOT EXISTS server_metric_samples_history_idx
  ON server_metric_samples(server_id, resolution, bucket_at DESC);

CREATE TABLE IF NOT EXISTS notifications (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  server_id UUID REFERENCES servers(id) ON DELETE CASCADE,
  severity TEXT NOT NULL CHECK (severity IN ('info','warning','danger')),
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT true,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS notifications_active_dedupe_idx
  ON notifications(user_id, dedupe_key) WHERE active;
CREATE INDEX IF NOT EXISTS notifications_user_created_idx
  ON notifications(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS server_addons (
  id UUID PRIMARY KEY,
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  provider TEXT NOT NULL CHECK (provider IN ('modrinth','curseforge')),
  project_id TEXT NOT NULL,
  version_id TEXT NOT NULL,
  name TEXT NOT NULL,
  file_name TEXT NOT NULL,
  file_hash TEXT NOT NULL,
  managed_path TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'installed' CHECK (status IN ('installing','installed','failed')),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (server_id, managed_path)
);
CREATE INDEX IF NOT EXISTS server_addons_server_idx ON server_addons(server_id, created_at DESC);

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_action_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_action_check CHECK (action IN
  ('provision','start','stop','restart','delete','backup','restore','command','update',
   'addon_install','addon_remove'));

INSERT INTO schema_migrations (version) VALUES (8) ON CONFLICT DO NOTHING;
COMMIT;

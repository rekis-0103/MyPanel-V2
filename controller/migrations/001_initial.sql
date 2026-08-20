CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('owner','operator','viewer')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline')),
  token_hash TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS servers (
  id UUID PRIMARY KEY,
  node_id UUID REFERENCES nodes(id),
  name TEXT NOT NULL,
  runtime TEXT NOT NULL,
  version TEXT NOT NULL,
  memory_mb INTEGER NOT NULL CHECK (memory_mb BETWEEN 1024 AND 8192),
  cpu INTEGER NOT NULL CHECK (cpu BETWEEN 1 AND 8),
  port INTEGER NOT NULL UNIQUE,
  desired_state TEXT NOT NULL DEFAULT 'offline' CHECK (desired_state IN ('running','offline')),
  observed_state TEXT NOT NULL DEFAULT 'offline' CHECK (observed_state IN ('running','offline','installing','error')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
  id UUID PRIMARY KEY,
  node_id UUID NOT NULL REFERENCES nodes(id),
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  action TEXT NOT NULL CHECK (action IN ('provision','start','stop','restart','delete')),
  status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','completed','failed')),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS jobs_node_status_idx ON jobs(node_id, status, created_at);

INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT DO NOTHING;

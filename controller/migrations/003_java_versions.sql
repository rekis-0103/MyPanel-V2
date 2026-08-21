BEGIN;

ALTER TABLE servers
  ADD COLUMN IF NOT EXISTS java_version INTEGER NOT NULL DEFAULT 21;

ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_java_version_check;
ALTER TABLE servers ADD CONSTRAINT servers_java_version_check
  CHECK (java_version IN (21, 25));

INSERT INTO schema_migrations (version) VALUES (3) ON CONFLICT DO NOTHING;

COMMIT;

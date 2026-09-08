BEGIN;

CREATE TABLE IF NOT EXISTS server_modpacks (
  server_id UUID PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
  provider TEXT NOT NULL DEFAULT 'curseforge' CHECK (provider = 'curseforge'),
  project_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  file_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version_name TEXT NOT NULL,
  icon_url TEXT NOT NULL DEFAULT '',
  runtime TEXT NOT NULL CHECK (runtime IN ('forge','neoforge')),
  minecraft_version TEXT NOT NULL,
  java_version INTEGER NOT NULL CHECK (java_version IN (21,25)),
  status TEXT NOT NULL CHECK (status IN ('installing','installed')),
  installed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_action_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_action_check CHECK (action IN
  ('provision','start','stop','restart','delete','backup','restore','command','update',
   'addon_install','addon_remove','modpack_install'));

INSERT INTO schema_migrations (version) VALUES (9) ON CONFLICT DO NOTHING;
COMMIT;

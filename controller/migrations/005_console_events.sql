BEGIN;

CREATE TABLE IF NOT EXISTS server_console_events (
  id BIGSERIAL PRIMARY KEY,
  server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  message TEXT NOT NULL CHECK (char_length(message) BETWEEN 1 AND 500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS server_console_events_server_id_idx
  ON server_console_events(server_id, id DESC);

INSERT INTO schema_migrations (version) VALUES (5) ON CONFLICT DO NOTHING;

COMMIT;

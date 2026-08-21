BEGIN;

-- Port ownership is enforced by allocations(node_id, bind_ip, port). The
-- original global constraint prevented reusing a port after a soft delete.
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_port_key;

-- Older migrations are intentionally idempotent and run on every startup.
-- Remove any allocation restored for a server that has already been deleted.
DELETE FROM allocations
WHERE server_id IN (SELECT id FROM servers WHERE deleted_at IS NOT NULL);

INSERT INTO schema_migrations (version) VALUES (4) ON CONFLICT DO NOTHING;

COMMIT;

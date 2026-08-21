BEGIN;

-- Port ownership is enforced by allocations(node_id, bind_ip, port). The
-- original global constraint prevented reusing a port after a soft delete.
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_port_key;

INSERT INTO schema_migrations (version) VALUES (4) ON CONFLICT DO NOTHING;

COMMIT;

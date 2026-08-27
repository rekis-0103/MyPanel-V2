BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS session_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET status='suspended', role='user' WHERE role IN ('operator','viewer');
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner','user'));
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check CHECK (status IN ('active','suspended'));
CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_key ON users(lower(username));

ALTER TABLE servers ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id);
UPDATE servers SET owner_user_id=(SELECT id FROM users WHERE role='owner' ORDER BY created_at LIMIT 1)
WHERE owner_user_id IS NULL;
ALTER TABLE servers ALTER COLUMN owner_user_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS servers_owner_created_idx ON servers(owner_user_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS hosting_packages (
  id UUID PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_idr BIGINT NOT NULL CHECK (price_idr >= 0),
  cpu INTEGER NOT NULL CHECK (cpu BETWEEN 1 AND 32),
  memory_mb INTEGER NOT NULL CHECK (memory_mb BETWEEN 1024 AND 131072),
  disk_mb INTEGER NOT NULL CHECK (disk_mb BETWEEN 1024 AND 1048576),
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS subscriptions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  server_id UUID NOT NULL UNIQUE REFERENCES servers(id) ON DELETE CASCADE,
  package_id UUID REFERENCES hosting_packages(id) ON DELETE SET NULL,
  package_name TEXT NOT NULL,
  price_idr BIGINT NOT NULL CHECK (price_idr >= 0),
  cpu INTEGER NOT NULL,
  memory_mb INTEGER NOT NULL,
  disk_mb INTEGER NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('provisioning','active','action_required','grace','released','canceled')),
  current_period_start TIMESTAMPTZ NOT NULL,
  current_period_end TIMESTAMPTZ NOT NULL,
  grace_ends_at TIMESTAMPTZ NOT NULL,
  resource_released_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS subscriptions_user_created_idx ON subscriptions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS subscriptions_expiry_idx ON subscriptions(status, current_period_end, grace_ends_at);

CREATE TABLE IF NOT EXISTS orders (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  server_id UUID NOT NULL REFERENCES servers(id),
  subscription_id UUID NOT NULL REFERENCES subscriptions(id),
  package_id UUID REFERENCES hosting_packages(id) ON DELETE SET NULL,
  kind TEXT NOT NULL CHECK (kind IN ('purchase','renewal')),
  status TEXT NOT NULL CHECK (status IN ('paid','action_required')),
  amount_idr BIGINT NOT NULL CHECK (amount_idr >= 0),
  package_name TEXT NOT NULL,
  cpu INTEGER NOT NULL,
  memory_mb INTEGER NOT NULL,
  disk_mb INTEGER NOT NULL,
  payment_reference TEXT NOT NULL UNIQUE,
  idempotency_key UUID NOT NULL,
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS orders_user_created_idx ON orders(user_id, created_at DESC);

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_action_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_action_check
  CHECK (action IN ('provision','start','stop','restart','delete','backup','restore','command','update','release'));

INSERT INTO hosting_packages (id,slug,name,description,price_idr,cpu,memory_mb,disk_mb,sort_order)
VALUES
  ('10000000-0000-0000-0000-000000000001','starter','Starter','Untuk server kecil dan bermain bersama teman.',29000,1,1024,10240,10),
  ('10000000-0000-0000-0000-000000000002','iron','Iron','Untuk komunitas kecil dengan plugin ringan.',59000,1,2048,20480,20),
  ('10000000-0000-0000-0000-000000000003','gold','Gold','Untuk komunitas menengah dan plugin lebih banyak.',99000,2,4096,40960,30),
  ('10000000-0000-0000-0000-000000000004','diamond','Diamond','Untuk komunitas besar atau modpack berat.',179000,4,8192,81920,40)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO schema_migrations (version) VALUES (6) ON CONFLICT DO NOTHING;
COMMIT;

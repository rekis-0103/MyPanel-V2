BEGIN;

ALTER TABLE hosting_packages
  ADD COLUMN IF NOT EXISTS theme_color TEXT NOT NULL DEFAULT '#3FB950',
  ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT 'grass',
  ADD COLUMN IF NOT EXISTS is_popular BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS is_recommended BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE hosting_packages DROP CONSTRAINT IF EXISTS hosting_packages_theme_color_check;
ALTER TABLE hosting_packages ADD CONSTRAINT hosting_packages_theme_color_check
  CHECK (theme_color ~ '^#[0-9A-Fa-f]{6}$');

ALTER TABLE hosting_packages DROP CONSTRAINT IF EXISTS hosting_packages_icon_check;
ALTER TABLE hosting_packages ADD CONSTRAINT hosting_packages_icon_check
  CHECK (icon IN ('grass','anvil','gold','diamond','feather','crystal'));

UPDATE hosting_packages
SET theme_color = CASE slug
    WHEN 'iron' THEN '#AEB7C2'
    WHEN 'gold' THEN '#E3B341'
    WHEN 'diamond' THEN '#58C7DF'
    ELSE '#3FB950'
  END,
  icon = CASE slug
    WHEN 'iron' THEN 'anvil'
    WHEN 'gold' THEN 'gold'
    WHEN 'diamond' THEN 'diamond'
    ELSE 'grass'
  END
WHERE theme_color = '#3FB950'
  AND icon = 'grass'
  AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 7);

INSERT INTO schema_migrations (version) VALUES (7) ON CONFLICT DO NOTHING;
COMMIT;

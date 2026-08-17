-- One-time production data migration for the two existing enterprises.
-- Run only after taking a PostgreSQL dump. The dump is the rollback artifact.
BEGIN;

INSERT INTO enterprises (name, code, status, registration_enabled, registration_mode, created_at, updated_at)
VALUES ('向量纪元', 'vector-epoch', 1, TRUE, 'open', EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint)
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  status = 1,
  registration_enabled = TRUE,
  registration_mode = 'open',
  updated_at = EXCLUDED.updated_at;

INSERT INTO enterprises (name, code, status, registration_enabled, registration_mode, created_at, updated_at)
VALUES ('星系数科', 'xingxi-shuke', 1, TRUE, 'open', EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint)
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  status = 1,
  registration_enabled = TRUE,
  registration_mode = 'open',
  updated_at = EXCLUDED.updated_at;

INSERT INTO enterprise_memberships (enterprise_id, user_id, role, status, invited_by, joined_at, updated_at)
SELECT e.id, u.id, 'owner', 1, u.id, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
FROM enterprises e
JOIN users u ON u.username = 'vectore'
WHERE e.code = 'vector-epoch'
ON CONFLICT (user_id) DO UPDATE SET
  enterprise_id = EXCLUDED.enterprise_id,
  role = 'owner',
  status = 1,
  invited_by = EXCLUDED.invited_by,
  updated_at = EXCLUDED.updated_at,
  deleted_at = NULL;

INSERT INTO enterprise_memberships (enterprise_id, user_id, role, status, invited_by, joined_at, updated_at)
SELECT e.id, u.id, 'member', 1, u.id, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
FROM enterprises e
JOIN users u ON u.username = 'roarkist'
WHERE e.code = 'vector-epoch'
ON CONFLICT (user_id) DO UPDATE SET
  enterprise_id = EXCLUDED.enterprise_id,
  role = 'member',
  status = 1,
  invited_by = EXCLUDED.invited_by,
  updated_at = EXCLUDED.updated_at,
  deleted_at = NULL;

INSERT INTO enterprise_memberships (enterprise_id, user_id, role, status, invited_by, joined_at, updated_at)
SELECT e.id, u.id, CASE WHEN u.username = 'PG付' OR u.display_name = 'PG付' THEN 'admin' ELSE 'member' END, 1, u.id, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
FROM enterprises e
JOIN users u ON u.username NOT IN ('vectore', 'roarkist')
WHERE e.code = 'xingxi-shuke'
ON CONFLICT (user_id) DO UPDATE SET
  enterprise_id = EXCLUDED.enterprise_id,
  role = EXCLUDED.role,
  status = 1,
  invited_by = EXCLUDED.invited_by,
  updated_at = EXCLUDED.updated_at,
  deleted_at = NULL;

INSERT INTO enterprise_invitations (enterprise_id, code_hash, status, expires_at, max_uses, used_count, created_by, created_at, updated_at)
SELECT e.id, '6ffc1d8ab2e503adec143eede86b760f93e2279d8fdbfd561cc4e27c7ae05821', 1, 0, 0, 0, u.id, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
FROM enterprises e
JOIN users u ON u.username = 'vectore'
WHERE e.code = 'vector-epoch'
ON CONFLICT (code_hash) DO NOTHING;

INSERT INTO enterprise_invitations (enterprise_id, code_hash, status, expires_at, max_uses, used_count, created_by, created_at, updated_at)
SELECT e.id, '6be555ce7b3bf278d787b2bc2a0943a331ce563102fde365557a2f3490bbc714', 1, 0, 0, 0, u.id, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
FROM enterprises e
JOIN users u ON u.username = 'vectore'
WHERE e.code = 'xingxi-shuke'
ON CONFLICT (code_hash) DO NOTHING;

UPDATE logs l
SET enterprise_id = m.enterprise_id
FROM enterprise_memberships m
WHERE l.user_id = m.user_id AND m.status = 1;

UPDATE tasks t
SET enterprise_id = m.enterprise_id
FROM enterprise_memberships m
WHERE t.user_id = m.user_id AND m.status = 1;

UPDATE midjourneys m0
SET enterprise_id = m.enterprise_id
FROM enterprise_memberships m
WHERE m0.user_id = m.user_id AND m.status = 1;

UPDATE quota_data q
SET enterprise_id = m.enterprise_id
FROM enterprise_memberships m
WHERE q.user_id = m.user_id AND m.status = 1;

INSERT INTO options (key, value) VALUES
  ('AutoGroups', '["OpenAI","ClaudeMax","Grok","Kimi","GLM","DeepSeek"]'),
  ('DefaultUseAutoGroup', 'true'),
  ('MaxTokenAutoGroups', '6'),
  ('UserUsableGroups', '{"auto":"自动分组","OpenAI":"OpenAI","ClaudeMax":"ClaudeMax","Grok":"Grok","Kimi":"Kimi","GLM":"GLM","DeepSeek":"DeepSeek"}')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

UPDATE tokens
SET "group" = 'auto', cross_group_retry = TRUE, auto_groups = ''
WHERE user_id IN (SELECT user_id FROM enterprise_memberships WHERE status = 1);

COMMIT;

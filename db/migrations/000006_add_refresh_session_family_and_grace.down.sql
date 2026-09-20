DROP INDEX IF EXISTS idx_refresh_sessions_family_active;
DROP INDEX IF EXISTS idx_refresh_sessions_family_id;

ALTER TABLE refresh_sessions
    DROP COLUMN IF EXISTS replacement_token_enc,
    DROP COLUMN IF EXISTS grace_until,
    DROP COLUMN IF EXISTS revocation_reason,
    DROP COLUMN IF EXISTS family_id;

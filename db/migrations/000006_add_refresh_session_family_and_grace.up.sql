ALTER TABLE refresh_sessions
    ADD COLUMN family_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN revocation_reason VARCHAR(32),
    ADD COLUMN grace_until BIGINT,
    ADD COLUMN replacement_token_enc BYTEA;

CREATE INDEX idx_refresh_sessions_family_id ON refresh_sessions (family_id);

CREATE INDEX idx_refresh_sessions_family_active
    ON refresh_sessions (family_id)
    WHERE revoked_at IS NULL;

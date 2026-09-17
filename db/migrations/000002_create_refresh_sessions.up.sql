CREATE TABLE refresh_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    revoked_at BIGINT,
    replaced_by_hash VARCHAR(64)
);

CREATE INDEX idx_refresh_sessions_user_id ON refresh_sessions (user_id);

CREATE INDEX idx_refresh_sessions_expires_at ON refresh_sessions (expires_at);

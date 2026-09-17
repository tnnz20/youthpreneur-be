CREATE TABLE training_catalog (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(16) NOT NULL UNIQUE,
    name VARCHAR(255),
    description TEXT,
    pic_phone VARCHAR(50),
    category VARCHAR(100),
    training_slots INTEGER,
    training_status process_status_enum,
    link VARCHAR(255),
    training_date DATE,
    training_period VARCHAR(100),
    speaker TEXT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT,
    CONSTRAINT training_catalog_training_slots_positive
        CHECK (training_slots IS NULL OR training_slots > 0)
);

COMMENT ON COLUMN training_catalog.public_id IS 'Public lookup handle, not a secret or auth credential.';
COMMENT ON COLUMN training_catalog.training_slots IS 'Optional capacity; NULL means unlimited.';
COMMENT ON COLUMN training_catalog.training_status IS 'Reuses process_status_enum; NULL means unset.';
COMMENT ON COLUMN training_catalog.deleted_at IS 'Unix epoch seconds; non-null marks a soft-deleted row.';

CREATE INDEX idx_training_catalog_deleted_at ON training_catalog (deleted_at);

CREATE INDEX idx_training_catalog_training_date ON training_catalog (training_date);

CREATE INDEX idx_training_catalog_category ON training_catalog (category);

CREATE INDEX idx_training_catalog_training_status ON training_catalog (training_status);

CREATE INDEX idx_training_catalog_cursor ON training_catalog (id) WHERE deleted_at IS NULL;

CREATE TABLE training_enrollments (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(16) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    training_catalog_id INTEGER NOT NULL REFERENCES training_catalog (id) ON DELETE CASCADE,
    register_date DATE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT
);

COMMENT ON COLUMN training_enrollments.public_id IS 'Public lookup handle, not a secret or auth credential.';
COMMENT ON COLUMN training_enrollments.user_id IS 'Enrolling user; always server-derived from the authenticated identity.';
COMMENT ON COLUMN training_enrollments.deleted_at IS 'Unix epoch seconds; non-null cancels the enrollment while preserving history.';

CREATE INDEX idx_training_enrollments_catalog_id ON training_enrollments (training_catalog_id);

CREATE INDEX idx_training_enrollments_user_id ON training_enrollments (user_id);

CREATE INDEX idx_training_enrollments_deleted_at ON training_enrollments (deleted_at);

CREATE INDEX idx_training_enrollments_cursor ON training_enrollments (id) WHERE deleted_at IS NULL;

CREATE INDEX idx_training_enrollments_catalog_active ON training_enrollments (training_catalog_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_training_enrollments_user_active ON training_enrollments (user_id, id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX training_enrollments_active_unique
    ON training_enrollments (training_catalog_id, user_id) WHERE deleted_at IS NULL;

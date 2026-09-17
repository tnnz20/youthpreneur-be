CREATE TYPE business_sector_enum AS ENUM (
    'Kuliner',
    'Perdagangan Ritel',
    'Agribisnis & Ketahanan Pangan',
    'Jasa & Layanan Publik',
    'Fashion & Konveksi',
    'E-Commerce & Ekonomi Kreatif'
);

CREATE TYPE enterprise_status_enum AS ENUM ('active', 'inactive');

CREATE TYPE legal_status_enum AS ENUM ('complete', 'in_progress', 'none');

CREATE TYPE business_digitization_enum AS ENUM ('high', 'medium', 'low');

CREATE TYPE intervention_needs_enum AS ENUM (
    'Pelatihan',
    'Mentoring',
    'Digitalisasi',
    'Legalitas',
    'Permodalan',
    'Kemitraan',
    'Pemasaran'
);

CREATE TYPE process_status_enum AS ENUM ('completed', 'ongoing', 'planned');

CREATE TYPE general_status_enum AS ENUM ('yes', 'no', 'in_progress');

CREATE TABLE enterprises (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(16) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name VARCHAR(255),
    business_sector business_sector_enum NOT NULL,
    legal_status legal_status_enum,
    business_digitization business_digitization_enum,
    intervention_needs intervention_needs_enum,
    training_status process_status_enum,
    mentoring_status process_status_enum,
    capital_access general_status_enum,
    partnership general_status_enum,
    initial_turnover DECIMAL(15, 2) NOT NULL DEFAULT 0,
    current_turnover DECIMAL(15, 2) NOT NULL DEFAULT 0,
    district VARCHAR(128),
    status enterprise_status_enum NOT NULL DEFAULT 'active',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT
);

COMMENT ON COLUMN enterprises.public_id IS 'Public lookup handle, not a secret or auth credential.';
COMMENT ON COLUMN enterprises.user_id IS 'Owning user; one user owns many enterprises.';
COMMENT ON COLUMN enterprises.initial_turnover IS 'Initial turnover as DECIMAL(15,2); non-negative.';
COMMENT ON COLUMN enterprises.current_turnover IS 'Current turnover as DECIMAL(15,2); non-negative.';
COMMENT ON COLUMN enterprises.deleted_at IS 'Unix epoch seconds; non-null marks a soft-deleted row.';

CREATE INDEX idx_enterprises_user_id ON enterprises (user_id);

CREATE INDEX idx_enterprises_district ON enterprises (district);

CREATE INDEX idx_enterprises_status ON enterprises (status);

CREATE INDEX idx_enterprises_business_sector ON enterprises (business_sector);

CREATE INDEX idx_enterprises_legal_status ON enterprises (legal_status);

CREATE INDEX idx_enterprises_business_digitization ON enterprises (business_digitization);

CREATE INDEX idx_enterprises_intervention_needs ON enterprises (intervention_needs);

CREATE INDEX idx_enterprises_training_status ON enterprises (training_status);

CREATE INDEX idx_enterprises_mentoring_status ON enterprises (mentoring_status);

CREATE INDEX idx_enterprises_capital_access ON enterprises (capital_access);

CREATE INDEX idx_enterprises_partnership ON enterprises (partnership);

CREATE INDEX idx_enterprises_deleted_at ON enterprises (deleted_at);

CREATE INDEX idx_enterprises_user_cursor ON enterprises (user_id, id) WHERE deleted_at IS NULL;

CREATE INDEX idx_enterprises_cursor ON enterprises (id) WHERE deleted_at IS NULL;

CREATE INDEX idx_enterprises_district_status ON enterprises (district, status) WHERE deleted_at IS NULL;

CREATE INDEX idx_enterprises_sector_status ON enterprises (business_sector, status) WHERE deleted_at IS NULL;

CREATE TABLE enterprise_audit_events (
    id SERIAL PRIMARY KEY,
    enterprise_id INTEGER NOT NULL REFERENCES enterprises (id) ON DELETE CASCADE,
    actor_user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    changed_fields JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at BIGINT NOT NULL
);

COMMENT ON COLUMN enterprise_audit_events.action IS 'One of create, update, or delete.';
COMMENT ON COLUMN enterprise_audit_events.changed_fields IS 'JSON object of fields changed by the mutation.';

CREATE INDEX idx_enterprise_audit_events_enterprise_id ON enterprise_audit_events (enterprise_id);

CREATE INDEX idx_enterprise_audit_events_actor_user_id ON enterprise_audit_events (actor_user_id);

CREATE INDEX idx_enterprise_audit_events_created_at ON enterprise_audit_events (created_at);

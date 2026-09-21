CREATE TYPE training_category_enum AS ENUM (
    'Wirausaha & Agribisnis',
    'Kriya & Kreativitas',
    'Digital & IPTEK',
    'Olahraga & Prestasi',
    'Komunitas & Pemuda'
);

CREATE TYPE training_enrollment_status_enum AS ENUM (
    'pending',
    'accepted',
    'rejected'
);

-- Refactor training_catalog columns
ALTER TABLE training_catalog RENAME COLUMN name TO title;
ALTER TABLE training_catalog RENAME COLUMN speaker TO mentor;
ALTER TABLE training_catalog RENAME COLUMN training_slots TO max_slots;

ALTER TABLE training_catalog DROP CONSTRAINT IF EXISTS training_catalog_training_slots_positive;
ALTER TABLE training_catalog ADD CONSTRAINT training_catalog_max_slots_positive
    CHECK (max_slots IS NULL OR max_slots > 0);

ALTER TABLE training_catalog ADD COLUMN address TEXT;
ALTER TABLE training_catalog ADD COLUMN thumbnail VARCHAR(255);
ALTER TABLE training_catalog ADD COLUMN start_date DATE;
ALTER TABLE training_catalog ADD COLUMN end_date DATE;
ALTER TABLE training_catalog ADD COLUMN registered_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE training_catalog ADD CONSTRAINT training_catalog_registered_count_non_negative
    CHECK (registered_count >= 0);

-- Migrate existing training_date to start_date
UPDATE training_catalog SET start_date = training_date WHERE start_date IS NULL AND training_date IS NOT NULL;

DROP INDEX IF EXISTS idx_training_catalog_training_date;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS training_date;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS training_period;

CREATE INDEX idx_training_catalog_start_date ON training_catalog (start_date);

-- Convert category to enum
DROP INDEX IF EXISTS idx_training_catalog_category;
UPDATE training_catalog SET category = NULL
WHERE category NOT IN (
    'Wirausaha & Agribisnis',
    'Kriya & Kreativitas',
    'Digital & IPTEK',
    'Olahraga & Prestasi',
    'Komunitas & Pemuda'
);
ALTER TABLE training_catalog ALTER COLUMN category TYPE training_category_enum
    USING category::training_category_enum;
CREATE INDEX idx_training_catalog_category ON training_catalog (category);

COMMENT ON COLUMN training_catalog.title IS 'Training title.';
COMMENT ON COLUMN training_catalog.mentor IS 'Training mentor name / bio.';
COMMENT ON COLUMN training_catalog.max_slots IS 'Optional capacity; NULL means unlimited.';
COMMENT ON COLUMN training_catalog.registered_count IS 'Active accepted enrollment count.';
COMMENT ON COLUMN training_catalog.thumbnail IS 'Relative URL path to uploaded thumbnail image.';
COMMENT ON COLUMN training_catalog.start_date IS 'Training offering start date.';
COMMENT ON COLUMN training_catalog.end_date IS 'Training offering end date.';

-- Refactor training_enrollments status
ALTER TABLE training_enrollments ADD COLUMN status training_enrollment_status_enum NOT NULL DEFAULT 'pending';

-- Backfill registered_count from existing accepted enrollments if any
UPDATE training_catalog c
SET registered_count = COALESCE((
    SELECT COUNT(*)
    FROM training_enrollments e
    WHERE e.training_catalog_id = c.id
      AND e.deleted_at IS NULL
      AND e.status = 'accepted'
), 0);
CREATE INDEX idx_training_enrollments_status ON training_enrollments (status);
COMMENT ON COLUMN training_enrollments.status IS 'Approval status: pending, accepted, or rejected.';

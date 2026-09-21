-- Revert training_enrollments status
DROP INDEX IF EXISTS idx_training_enrollments_status;
ALTER TABLE training_enrollments DROP COLUMN IF EXISTS status;
DROP TYPE IF EXISTS training_enrollment_status_enum;

-- Revert training_catalog category enum
DROP INDEX IF EXISTS idx_training_catalog_category;
ALTER TABLE training_catalog ALTER COLUMN category TYPE VARCHAR(100) USING category::VARCHAR(100);
CREATE INDEX idx_training_catalog_category ON training_catalog (category);
DROP TYPE IF EXISTS training_category_enum;

-- Revert training_catalog dates
DROP INDEX IF EXISTS idx_training_catalog_start_date;
ALTER TABLE training_catalog ADD COLUMN training_date DATE;
UPDATE training_catalog SET training_date = start_date WHERE start_date IS NOT NULL;
CREATE INDEX idx_training_catalog_training_date ON training_catalog (training_date);
ALTER TABLE training_catalog ADD COLUMN training_period VARCHAR(100);
ALTER TABLE training_catalog DROP COLUMN IF EXISTS end_date;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS start_date;

-- Revert new columns and renames
ALTER TABLE training_catalog DROP CONSTRAINT IF EXISTS training_catalog_registered_count_non_negative;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS registered_count;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS thumbnail;
ALTER TABLE training_catalog DROP COLUMN IF EXISTS address;

ALTER TABLE training_catalog DROP CONSTRAINT IF EXISTS training_catalog_max_slots_positive;
ALTER TABLE training_catalog RENAME COLUMN max_slots TO training_slots;
ALTER TABLE training_catalog ADD CONSTRAINT training_catalog_training_slots_positive
    CHECK (training_slots IS NULL OR training_slots > 0);

ALTER TABLE training_catalog RENAME COLUMN mentor TO speaker;
ALTER TABLE training_catalog RENAME COLUMN title TO name;

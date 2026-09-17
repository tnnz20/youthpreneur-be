-- training_enrollments references training_catalog, so drop it first. Both
-- table drops also remove their own indexes. process_status_enum is shared with
-- the enterprise migration and is intentionally preserved.
DROP TABLE IF EXISTS training_enrollments;

DROP TABLE IF EXISTS training_catalog;

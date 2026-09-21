ALTER TABLE enterprises
    DROP COLUMN IF EXISTS dispora_support,
    DROP COLUMN IF EXISTS focus_commodity,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS description;

ALTER TABLE enterprises ALTER COLUMN enterprise_name DROP NOT NULL;
ALTER TABLE enterprises RENAME COLUMN enterprise_name TO name;

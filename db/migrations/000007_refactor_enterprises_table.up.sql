ALTER TABLE enterprises RENAME COLUMN name TO enterprise_name;

UPDATE enterprises SET enterprise_name = 'Enterprise ' || id WHERE enterprise_name IS NULL;
ALTER TABLE enterprises ALTER COLUMN enterprise_name SET NOT NULL;

ALTER TABLE enterprises
    ADD COLUMN description TEXT,
    ADD COLUMN address TEXT,
    ADD COLUMN focus_commodity VARCHAR(255),
    ADD COLUMN dispora_support VARCHAR(255);

COMMENT ON COLUMN enterprises.enterprise_name IS 'Official name of the enterprise; non-null.';
COMMENT ON COLUMN enterprises.focus_commodity IS 'Primary product line or focus commodity; up to 255 chars.';
COMMENT ON COLUMN enterprises.dispora_support IS 'Dispora program support or grant received; up to 255 chars.';

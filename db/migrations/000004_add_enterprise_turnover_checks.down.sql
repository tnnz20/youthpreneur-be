ALTER TABLE enterprises
    DROP CONSTRAINT IF EXISTS enterprises_current_turnover_non_negative;

ALTER TABLE enterprises
    DROP CONSTRAINT IF EXISTS enterprises_initial_turnover_non_negative;

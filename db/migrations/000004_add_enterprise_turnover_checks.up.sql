ALTER TABLE enterprises
    ADD CONSTRAINT enterprises_initial_turnover_non_negative
        CHECK (initial_turnover >= 0 AND initial_turnover < 10000000000000);

ALTER TABLE enterprises
    ADD CONSTRAINT enterprises_current_turnover_non_negative
        CHECK (current_turnover >= 0 AND current_turnover < 10000000000000);

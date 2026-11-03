ALTER TABLE assistant_steps DROP CONSTRAINT assistant_steps_kind_check;
ALTER TABLE assistant_steps ADD CONSTRAINT assistant_steps_kind_check
    CHECK (kind IN ('inventory','health','metrics','events','topology','logs','alert_history','workload_history','inference'));

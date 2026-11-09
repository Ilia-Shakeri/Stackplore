ALTER TABLE runbook_corpora
 ADD COLUMN provenance_assistance_reviewed BOOLEAN NOT NULL DEFAULT false,
 ADD CONSTRAINT runbook_assistance_scope_reviewed CHECK (
  NOT provenance_assistance_reviewed OR
  (provenance_use_reviewed IS TRUE AND provenance_kind IS NOT NULL)
 );

ALTER TABLE assistant_steps DROP CONSTRAINT assistant_steps_kind_check;
ALTER TABLE assistant_steps ADD CONSTRAINT assistant_steps_kind_check
 CHECK (kind IN ('inventory','health','metrics','events','topology','logs','alert_history','workload_history','reviewed_document','inference'));

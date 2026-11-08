ALTER TABLE runbook_corpora
 ADD COLUMN provenance_kind TEXT,
 ADD COLUMN provenance_source TEXT,
 ADD COLUMN provenance_license TEXT,
 ADD COLUMN provenance_use_reviewed BOOLEAN,
 ADD CONSTRAINT runbook_provenance_complete CHECK (
  (provenance_kind IS NULL AND provenance_source IS NULL AND provenance_license IS NULL AND provenance_use_reviewed IS NULL)
  OR
  (provenance_kind IS NOT NULL AND provenance_source IS NOT NULL AND provenance_license IS NOT NULL AND provenance_use_reviewed IS NOT NULL
   AND provenance_kind IN ('operator_authored','licensed_copy','public_domain')
   AND octet_length(provenance_source) BETWEEN 1 AND 512
   AND octet_length(provenance_license) BETWEEN 1 AND 128
   AND provenance_use_reviewed)
 );

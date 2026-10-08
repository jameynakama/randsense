-- OEWN's plural is a rare variant ("camerae") and SUBTLEX-US has the regular
-- plural far more often, so the spelling rules win. Set by ingest every run.
ALTER TABLE nouns ADD COLUMN regular_plural BOOLEAN NOT NULL DEFAULT FALSE;

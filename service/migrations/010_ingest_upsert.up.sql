-- plural_guess is ingest's heuristic, rewritten every run. plural_override is
-- curation, which ingest never writes; NULL means the guess stands.
ALTER TABLE nouns RENAME COLUMN plural TO plural_guess;
ALTER TABLE nouns ADD COLUMN plural_override BOOLEAN;
ALTER TABLE nouns ADD COLUMN plural BOOLEAN NOT NULL
    GENERATED ALWAYS AS (coalesce(plural_override, plural_guess)) STORED;

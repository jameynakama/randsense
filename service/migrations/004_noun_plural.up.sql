-- Lemma is already a plural form ("Rastas"), so it isn't pluralized again and
-- forces its noun phrase plural. Set by ingest heuristically; curatable.
ALTER TABLE nouns ADD COLUMN plural BOOLEAN NOT NULL DEFAULT FALSE;

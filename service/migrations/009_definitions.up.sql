-- A content word's OEWN glosses in sense order, filled by ingest.
ALTER TABLE nouns      ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE verbs      ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE adjectives ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE adverbs    ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';

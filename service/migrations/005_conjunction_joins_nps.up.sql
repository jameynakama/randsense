-- Conjunction can join noun phrases ("the goose and the moon"), not just
-- clauses. "for", "so", "because" can't.
ALTER TABLE conjunctions ADD COLUMN joins_nps BOOLEAN NOT NULL DEFAULT FALSE;
